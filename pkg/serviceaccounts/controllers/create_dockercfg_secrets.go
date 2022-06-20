package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	v1 "k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/registry/core/secret"

	"github.com/openshift/library-go/pkg/build/naming"
)

const (
	MaxRetriesBeforeResync = 5
	ExpirationCheckPeriod  = 10 * time.Minute

	// ServiceAccountTokenValueAnnotation stores the actual value of the token so that a dockercfg secret can be
	// made without having a value dockerURL
	ServiceAccountTokenValueAnnotation = "openshift.io/token-secret.value"

	// CreateDockercfgSecretsController is the name of this controller that should be
	// attached to all token secrets this controller create
	CreateDockercfgSecretsController = "openshift.io/create-dockercfg-secrets"

	DockercfgExpirationAnnotationKey = "openshift.io/dockercfg-token-expiry"

	// PendingTokenAnnotation contains the name of the dockercfg secret that is waiting for the
	// token data population
	PendingTokenAnnotation = "openshift.io/create-dockercfg-secrets.pending-secret"

	// DeprecatedKubeCreatedByAnnotation was removed by https://github.com/kubernetes/kubernetes/pull/54445 (liggitt approved).
	DeprecatedKubeCreatedByAnnotation = "kubernetes.io/created-by"

	// These constants are here to create a name that is short enough to survive chopping by generate name
	maxNameLength             = 63
	randomLength              = 5
	maxSecretPrefixNameLength = maxNameLength - randomLength
)

// DockercfgControllerOptions contains options for the DockercfgController
type DockercfgControllerOptions struct {
	// Resync is the time.Duration at which to fully re-list service accounts.
	// If zero, re-list will be delayed as long as possible
	Resync time.Duration

	// DockerURLsInitialized is used to send a signal to this controller that it has the correct set of docker urls
	// This is normally signaled from the DockerRegistryServiceController which watches for updates to the internal
	// container image registry service.
	DockerURLsInitialized chan struct{}
}

// NewDockercfgController returns a new *DockercfgController.
func NewDockercfgController(
	serviceAccounts informers.ServiceAccountInformer,
	secrets informers.SecretInformer,
	cl kclientset.Interface,
	options DockercfgControllerOptions,
) *DockercfgController {
	e := &DockercfgController{
		client:                cl,
		saQueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "serviceaccount-create-dockercfg"),
		secretQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "secrets-create-dockercfg"),
		dockerURLsInitialized: options.DockerURLsInitialized,
	}

	serviceAccountCache := serviceAccounts.Informer().GetStore()
	e.serviceAccountController = serviceAccounts.Informer().GetController()
	serviceAccounts.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				serviceAccount := obj.(*v1.ServiceAccount)
				klog.V(5).Infof("Adding service account %s", serviceAccount.Name)
				e.enqueueServiceAccount(serviceAccount)
			},
			UpdateFunc: func(old, cur interface{}) {
				serviceAccount := cur.(*v1.ServiceAccount)
				klog.V(5).Infof("Updating service account %s", serviceAccount.Name)
				// Resync on service object relist.
				e.enqueueServiceAccount(serviceAccount)
			},
		},
	)
	e.serviceAccountCache = NewEtcdMutationCache(serviceAccountCache)

	e.secretCache = secrets.Informer().GetIndexer()
	e.secretController = secrets.Informer().GetController()
	secrets.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *v1.Secret:
					return t.Type == v1.SecretTypeDockercfg
				default:
					utilruntime.HandleError(fmt.Errorf("object passed to %T that is not expected: %T", e, obj))
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				// We don't need to react to secret deletes, the deleted_dockercfg_secrets controller does that
				// It also updates the SA so we will eventually get back to creating a new secret
				AddFunc:    func(cur interface{}) { e.handleTokenSecretUpdate(nil, cur) },
				UpdateFunc: func(old, cur interface{}) { e.handleTokenSecretUpdate(old, cur) },
			},
		},
	)

	return e
}

// DockercfgController manages dockercfg secrets for ServiceAccount objects
type DockercfgController struct {
	client kclientset.Interface

	dockerURLLock         sync.Mutex
	dockerURLs            []string
	dockerURLsInitialized chan struct{}

	serviceAccountCache      MutationCache
	serviceAccountController cache.Controller
	secretCache              cache.Store
	secretController         cache.Controller
	apiAudiences             []string

	saQueue     workqueue.RateLimitingInterface
	secretQueue workqueue.RateLimitingInterface
}

// handleTokenSecretUpdate checks the type of the updated secret and re-syncs
// its owning SA if it is a dockercfg secret in case its data was rewritten
func (e *DockercfgController) handleTokenSecretUpdate(_, newObj interface{}) {
	secret := newObj.(*v1.Secret)
	if secret.Type == v1.SecretTypeDockercfg {
		e.enqueueSecret(secret)
	}
}

func (e *DockercfgController) enqueueSecret(s *v1.Secret) {
	key, err := cache.MetaNamespaceKeyFunc(s)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error syncing dockercfg secret %s/%s: %v", s.Namespace, s.Name, err))
		return
	}
	// we'll need to recreate the secret
	e.secretQueue.Add(key)
}

func (e *DockercfgController) enqueueServiceAccountForToken(dockerCfgSecret *v1.Secret) {
	serviceAccount := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dockerCfgSecret.Annotations[v1.ServiceAccountNameKey],
			Namespace: dockerCfgSecret.Namespace,
			UID:       types.UID(dockerCfgSecret.Annotations[v1.ServiceAccountUIDKey]),
		},
	}
	key, err := controller.KeyFunc(serviceAccount)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error syncing token secret %s/%s: %v", dockerCfgSecret.Namespace, dockerCfgSecret.Name, err))
		return
	}
	e.saQueue.Add(key)
}

func (e *DockercfgController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer e.saQueue.ShutDown()
	defer e.secretQueue.ShutDown()

	klog.Infof("Starting DockercfgController controller")
	defer klog.Infof("Shutting down DockercfgController controller")

	// Wait for the store to sync before starting any work in this controller.
	ready := make(chan struct{})
	go e.waitForDockerURLs(ctx, ready)
	select {
	case <-ready:
	case <-ctx.Done():
		return
	}
	klog.V(1).Infof("urls found")

	// Wait for the stores to fill
	if !cache.WaitForCacheSync(ctx.Done(), e.serviceAccountController.HasSynced, e.secretController.HasSynced) {
		return
	}
	klog.V(1).Infof("caches synced")

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, e.serviceAccountWorker, time.Second)
		go wait.UntilWithContext(ctx, e.secretWorker, time.Second)
		go wait.UntilWithContext(ctx, e.secretExpirationsChecker, ExpirationCheckPeriod)
	}
	<-ctx.Done()
}

func (c *DockercfgController) waitForDockerURLs(ctx context.Context, ready chan<- struct{}) {
	defer utilruntime.HandleCrash()

	// wait for the initialization to complete to be informed of a stop
	select {
	case <-c.dockerURLsInitialized:
	case <-ctx.Done():
		return
	}

	close(ready)
}

func (e *DockercfgController) enqueueServiceAccount(serviceAccount *v1.ServiceAccount) {
	if !needsDockercfgSecret(serviceAccount) {
		return
	}

	key, err := controller.KeyFunc(serviceAccount)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", serviceAccount, err)
		return
	}

	e.saQueue.Add(key)
}

// serviceAccountWorker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (e *DockercfgController) serviceAccountWorker(ctx context.Context) {
	key, quit := e.saQueue.Get()
	if quit {
		return
	}
	defer e.saQueue.Done(key)

	if err := e.syncServiceAccount(ctx, key.(string)); err == nil {
		// this means the request was successfully handled.  We should "forget" the item so that any retry
		// later on is reset
		e.saQueue.Forget(key)

	} else {
		// if we had an error it means that we didn't handle it, which means that we want to requeue the work
		if e.saQueue.NumRequeues(key) > MaxRetriesBeforeResync {
			utilruntime.HandleError(fmt.Errorf("error syncing service, it will be tried again on a resync %v: %v", key, err))
			e.saQueue.Forget(key)
		} else {
			klog.V(4).Infof("error syncing service, it will be retried %v: %v", key, err)
			e.saQueue.AddRateLimited(key)

		}
	}
}

// secretWorker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (e *DockercfgController) secretWorker(ctx context.Context) {
	key, quit := e.secretQueue.Get()
	if quit {
		return
	}
	defer e.secretQueue.Done(key)

	if err := e.syncSecret(ctx, key.(string)); err == nil {
		// this means the request was successfully handled.  We should "forget" the item so that any retry
		// later on is reset
		e.secretQueue.Forget(key)

	} else {
		// if we had an error it means that we didn't handle it, which means that we want to requeue the work
		if e.secretQueue.NumRequeues(key) > MaxRetriesBeforeResync {
			utilruntime.HandleError(fmt.Errorf("error syncing service, it will be tried again on a resync %v: %v", key, err))
			e.secretQueue.Forget(key)
		} else {
			klog.V(4).Infof("error syncing service, it will be retried %v: %v", key, err)
			e.secretQueue.AddRateLimited(key)
		}
	}
}

func (e *DockercfgController) secretExpirationsChecker(_ context.Context) {
	for _, s := range e.secretCache.List() {
		secretObj := s.(*v1.Secret)
		if secretObj.Type != v1.SecretTypeDockercfg {
			continue
		}

		expires, err := isExpiring(secretObj)
		if err != nil {
			klog.Errorf("failed to determine expiration: %v", err)
			continue
		}

		if expires {
			e.enqueueSecret(secretObj)
		}
	}
}

func isExpiring(secret *v1.Secret) (bool, error) {
	expiry, ok := secret.Annotations[DockercfgExpirationAnnotationKey]
	if !ok {
		return false, nil
	}

	expiryUnix, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil {
		return false, fmt.Errorf("failed to parse expiry of the secret '%s/%s': %v", secret.Namespace, secret.Name, err)

	}

	expiryTime := time.Unix(expiryUnix, 0)
	if time.Now().After(expiryTime.Add(-ExpirationCheckPeriod - 1*time.Minute)) {
		return true, nil
	}
	return false, nil
}

func (e *DockercfgController) SetDockerURLs(newDockerURLs ...string) {
	e.dockerURLLock.Lock()
	defer e.dockerURLLock.Unlock()

	e.dockerURLs = newDockerURLs
}

func needsDockercfgSecret(serviceAccount *v1.ServiceAccount) bool {
	mountableDockercfgSecrets, imageDockercfgPullSecrets := getGeneratedDockercfgSecretNames(serviceAccount)

	// look for an ImagePullSecret in the form
	if len(imageDockercfgPullSecrets) > 0 && len(mountableDockercfgSecrets) > 0 {
		return false
	}

	return true
}

func (e *DockercfgController) syncServiceAccount(ctx context.Context, key string) error {
	obj, exists, err := e.serviceAccountCache.GetByKey(key)
	if err != nil {
		klog.V(4).Infof("Unable to retrieve service account %v from store: %v", key, err)
		return err
	}
	if !exists {
		klog.V(4).Infof("Service account has been deleted %v", key)
		return nil
	}

	if !needsDockercfgSecret(obj.(*v1.ServiceAccount)) {
		return nil
	}

	serviceAccount := obj.(*v1.ServiceAccount).DeepCopyObject().(*v1.ServiceAccount)

	mountableDockercfgSecrets, imageDockercfgPullSecrets := getGeneratedDockercfgSecretNames(serviceAccount)

	// If we have a pull secret in one list, use it for the other.  It must only be in one list because
	// otherwise we wouldn't "needsDockercfgSecret"
	foundPullSecret := len(imageDockercfgPullSecrets) > 0
	foundMountableSecret := len(mountableDockercfgSecrets) > 0
	if foundPullSecret || foundMountableSecret {
		switch {
		case foundPullSecret:
			serviceAccount.Secrets = append(serviceAccount.Secrets, v1.ObjectReference{Name: imageDockercfgPullSecrets.List()[0]})
		case foundMountableSecret:
			serviceAccount.ImagePullSecrets = append(serviceAccount.ImagePullSecrets, v1.LocalObjectReference{Name: mountableDockercfgSecrets.List()[0]})
		}
		// Clear the pending token annotation when updating
		delete(serviceAccount.Annotations, PendingTokenAnnotation)

		updatedSA, err := e.client.CoreV1().ServiceAccounts(serviceAccount.Namespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
		if err == nil {
			e.serviceAccountCache.Mutation(updatedSA)
		}
		return err
	}

	dockercfgSecret, created, err := e.createDockerPullSecret(ctx, serviceAccount)
	if err != nil {
		return err
	}
	if !created {
		klog.V(5).Infof("The dockercfg secret was not created for service account %s/%s, will retry", serviceAccount.Namespace, serviceAccount.Name)
		return nil
	}

	first := true
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if !first {
			obj, exists, err := e.serviceAccountCache.GetByKey(key)
			if err != nil {
				return err
			}
			if !exists || !needsDockercfgSecret(obj.(*v1.ServiceAccount)) || serviceAccount.UID != obj.(*v1.ServiceAccount).UID {
				// somehow a dockercfg secret appeared or the SA disappeared.  cleanup the secret we made and return
				klog.V(2).Infof("Deleting secret because the work is already done %s/%s", dockercfgSecret.Namespace, dockercfgSecret.Name)
				e.client.CoreV1().Secrets(dockercfgSecret.Namespace).Delete(ctx, dockercfgSecret.Name, metav1.DeleteOptions{})
				return nil
			}

			serviceAccount = obj.(*v1.ServiceAccount).DeepCopyObject().(*v1.ServiceAccount)
		}
		first = false

		serviceAccount.Secrets = append(serviceAccount.Secrets, v1.ObjectReference{Name: dockercfgSecret.Name})
		serviceAccount.ImagePullSecrets = append(serviceAccount.ImagePullSecrets, v1.LocalObjectReference{Name: dockercfgSecret.Name})
		// Clear the pending token annotation when updating
		delete(serviceAccount.Annotations, PendingTokenAnnotation)

		updatedSA, err := e.client.CoreV1().ServiceAccounts(serviceAccount.Namespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
		if err == nil {
			e.serviceAccountCache.Mutation(updatedSA)
		}
		return err
	})

	if err != nil {
		// nothing to do.  Our choice was stale or we got a conflict.  Either way that means that the service account was updated.  We simply need to return because we'll get an update notification later
		// we do need to clean up our dockercfgSecret.  token secrets are cleaned up by the controller handling service account dockercfg secret deletes
		klog.V(2).Infof("Deleting secret %s/%s (err=%v)", dockercfgSecret.Namespace, dockercfgSecret.Name, err)
		e.client.CoreV1().Secrets(dockercfgSecret.Namespace).Delete(ctx, dockercfgSecret.Name, metav1.DeleteOptions{})
	}
	return err
}

func (e *DockercfgController) syncSecret(ctx context.Context, key string) error {
	secret, exists, err := e.secretCache.GetByKey(key)
	if err != nil {
		klog.V(4).Infof("Unable to retrieve secret %v from store: %v", key, err)
		return err
	}

	if !exists {
		return nil
	}

	secretObj := secret.(*v1.Secret)
	expires, err := isExpiring(secretObj)
	if err != nil {
		return fmt.Errorf("failed to determine secret token expiration: %v", err)
	}

	if !expires {
		return nil
	}

	// the token is expiring soon, request a new one

	saName := secretObj.Annotations[v1.ServiceAccountNameKey]
	if len(saName) == 0 {
		klog.V(4).Infof("secret '%s/%s' has no %s annotation", secretObj.Namespace, secretObj.Name, v1.ServiceAccountNameKey)
		return nil
	}

	_, err = e.renewSecretToken(ctx, saName, secretObj)
	return err
}

func (e *DockercfgController) renewSecretToken(ctx context.Context, saName string, dockercfgSecret *v1.Secret) (*v1.Secret, error) {
	saToken, saTokenExpiry, err := e.requestToken(ctx, dockercfgSecret.Namespace, saName, dockercfgSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to request SA token: %w", err)
	}

	e.dockerURLLock.Lock()
	defer e.dockerURLLock.Unlock()
	dockercfgContent, err := createSADockerCfg(e.dockerURLs, saToken)
	if err != nil {
		return nil, err
	}

	dockercfgSecretCopy := dockercfgSecret.DeepCopy()
	if dockercfgSecretCopy.Annotations == nil {
		dockercfgSecretCopy.Annotations = map[string]string{}
	}
	dockercfgSecretCopy.Annotations[ServiceAccountTokenValueAnnotation] = saToken
	dockercfgSecretCopy.Annotations[DockercfgExpirationAnnotationKey] = strconv.FormatInt(saTokenExpiry.Unix(), 10)
	dockercfgSecretCopy.Data[v1.DockerConfigKey] = dockercfgContent

	return e.client.CoreV1().Secrets(dockercfgSecretCopy.Namespace).Update(ctx, dockercfgSecretCopy, metav1.UpdateOptions{})
}

func (e *DockercfgController) requestToken(ctx context.Context, saNamespace, saName string, dockercfgSecret *v1.Secret) (string, *metav1.Time, error) {
	dockercfgSecretRef := metav1.NewControllerRef(dockercfgSecret, v1.SchemeGroupVersion.WithKind("Secret"))

	tokenResp, err := e.client.CoreV1().ServiceAccounts(saNamespace).
		CreateToken(
			ctx,
			saName,
			&authenticationv1.TokenRequest{
				Spec: authenticationv1.TokenRequestSpec{
					Audiences: e.apiAudiences,
					BoundObjectRef: &authenticationv1.BoundObjectReference{
						Kind:       dockercfgSecretRef.Kind,
						APIVersion: dockercfgSecretRef.APIVersion,
						Name:       dockercfgSecretRef.Name,
						UID:        dockercfgSecretRef.UID,
					},
				},
			},
			metav1.CreateOptions{},
		)
	if err != nil {
		return "", nil, fmt.Errorf("failed to retrieve a token for SA '%s/%s': %v", saNamespace, saName, err)
	}

	respToken := tokenResp.Status.Token
	if len(respToken) == 0 {
		return "", nil, fmt.Errorf("retrieved an empty token for SA '%s/%s'", saNamespace, saName)
	}

	return respToken, &tokenResp.Status.ExpirationTimestamp, nil
}

// createDockerPullSecret creates a dockercfg secret based on the token secret
func (e *DockercfgController) createDockerPullSecret(ctx context.Context, serviceAccount *v1.ServiceAccount) (*v1.Secret, bool, error) {
	pendingTokenName := serviceAccount.Annotations[PendingTokenAnnotation]

	// If this service account has no record of a pending token name, record one
	if len(pendingTokenName) == 0 {
		pendingTokenName = secret.Strategy.GenerateName(getDockercfgSecretNamePrefix(serviceAccount.Name))
		if serviceAccount.Annotations == nil {
			serviceAccount.Annotations = map[string]string{}
		}
		serviceAccount.Annotations[PendingTokenAnnotation] = pendingTokenName
		updatedServiceAccount, err := e.client.CoreV1().ServiceAccounts(serviceAccount.Namespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
		// Conflicts mean we'll get called to sync this service account again
		if kapierrors.IsConflict(err) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, fmt.Errorf("failed to update the SA: %w", err)
		}
		serviceAccount = updatedServiceAccount
	}
	currentSecret, err := e.client.CoreV1().Secrets(serviceAccount.Namespace).Get(ctx, pendingTokenName, metav1.GetOptions{})
	if err != nil {
		if kapierrors.IsNotFound(err) {
			// this client supplies an empty struct in this case for some reason
			currentSecret = nil
		} else {
			return nil, false, fmt.Errorf("failed to retrieve the current dockercfg secret: %w", err)
		}
	}

	if currentSecret != nil && len(currentSecret.Annotations[ServiceAccountTokenValueAnnotation]) != 0 {
		return currentSecret, true, nil
	}

	if currentSecret == nil {
		dockercfgSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pendingTokenName,
				Namespace: serviceAccount.Namespace,
				Annotations: map[string]string{
					v1.ServiceAccountNameKey: serviceAccount.Name,
					v1.ServiceAccountUIDKey:  string(serviceAccount.UID),
				},
			},
			Type: v1.SecretTypeDockercfg,
			Data: map[string][]byte{
				v1.DockerConfigKey: []byte("{}"), // required key but we have no data yet
			},
		}

		blockDeletion := false
		ownerRef := metav1.NewControllerRef(serviceAccount, v1.SchemeGroupVersion.WithKind("ServiceAccount"))
		ownerRef.BlockOwnerDeletion = &blockDeletion
		dockercfgSecret.SetOwnerReferences([]metav1.OwnerReference{*ownerRef})

		klog.V(4).Infof("Creating dockercfg secret %q for service account %s/%s", dockercfgSecret.Name, serviceAccount.Namespace, serviceAccount.Name)

		// Save the secret
		_, err = e.client.CoreV1().Secrets(serviceAccount.Namespace).Create(ctx, dockercfgSecret, metav1.CreateOptions{})
		// If we cannot create this secret because the namespace it is being terminated isn't a thing we should fail and requeue a retry.
		// Instead, we know that when a new namespace gets created, the serviceaccount will be recreated and we'll get a second shot at
		// processing the serviceaccount.
		if kapierrors.HasStatusCause(err, v1.NamespaceTerminatingCause) {
			return nil, false, nil
		}

		if err != nil {
			return nil, false, fmt.Errorf("failed to create the dockercfg secret: %w", err)
		}

		currentSecret, err = e.client.CoreV1().Secrets(serviceAccount.Namespace).Get(ctx, dockercfgSecret.Name, metav1.GetOptions{})
		if err != nil {
			return nil, false, fmt.Errorf("failed to retrieve the dockercfg secret that was supposed to be already created: %w", err)
		}
	}

	updatedSecret, err := e.renewSecretToken(ctx, serviceAccount.Name, currentSecret)
	return updatedSecret, err == nil, err
}

func createSADockerCfg(dockerURLs []string, saToken string) ([]byte, error) {
	dockercfg := credentialprovider.DockerConfig{}
	for _, dockerURL := range dockerURLs {
		dockercfg[dockerURL] = credentialprovider.DockerConfigEntry{
			Username: "serviceaccount",
			Password: string(saToken),
			Email:    "serviceaccount@example.org",
		}
	}

	return json.Marshal(&dockercfg)
}

func getGeneratedDockercfgSecretNames(serviceAccount *v1.ServiceAccount) (sets.String, sets.String) {
	mountableDockercfgSecrets := sets.String{}
	imageDockercfgPullSecrets := sets.String{}

	secretNamePrefix := getDockercfgSecretNamePrefix(serviceAccount.Name)

	for _, s := range serviceAccount.Secrets {
		if strings.HasPrefix(s.Name, secretNamePrefix) {
			mountableDockercfgSecrets.Insert(s.Name)
		}
	}
	for _, s := range serviceAccount.ImagePullSecrets {
		if strings.HasPrefix(s.Name, secretNamePrefix) {
			imageDockercfgPullSecrets.Insert(s.Name)
		}
	}
	return mountableDockercfgSecrets, imageDockercfgPullSecrets
}

func getDockercfgSecretNamePrefix(serviceAccountName string) string {
	return naming.GetName(serviceAccountName, "dockercfg-", maxSecretPrefixNameLength)
}
