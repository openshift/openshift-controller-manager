package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"go.uber.org/atomic"
	"golang.org/x/exp/slices"
	"gopkg.in/square/go-jose.v2/jwt"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

type imagePullSecretController struct {
	client          kubernetes.Interface
	secrets         listers.SecretLister
	serviceAccounts listers.ServiceAccountLister
	cacheSyncs      []cache.InformerSynced
	queue           workqueue.RateLimitingInterface
	urls            *atomic.Pointer[[]string]
	urlsC           chan []string
	kids            *atomic.Pointer[[]string]
	kidsC           chan []string
}

// some handy types so we don't mixup these channels
type urlsChan chan<- []string
type kidsChan chan<- []string

func NewImagePullSecretController(kubeclient kubernetes.Interface, secrets informers.SecretInformer, serviceAccounts informers.ServiceAccountInformer) (*imagePullSecretController, kidsChan, urlsChan) {
	c := &imagePullSecretController{
		client:          kubeclient,
		secrets:         secrets.Lister(),
		serviceAccounts: serviceAccounts.Lister(),
		cacheSyncs:      []cache.InformerSynced{secrets.Informer().HasSynced, serviceAccounts.Informer().HasSynced},
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "bound-token-managed-image-pull-secrets"),
		kids:            atomic.NewPointer[[]string](nil),
		urls:            atomic.NewPointer[[]string](nil),
		kidsC:           make(chan []string),
		urlsC:           make(chan []string),
	}
	secrets.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: isManagedImagePullSecret,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj any) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err == nil {
					c.queue.Add(key)
				}
			},
			UpdateFunc: func(_ any, new any) {
				key, err := cache.MetaNamespaceKeyFunc(new)
				if err == nil {
					c.queue.Add(key)
				}
			},
			DeleteFunc: func(obj any) {
				key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
				if err == nil {
					c.queue.Add(key)
				}
			},
		},
	})
	return c, c.kidsC, c.urlsC
}

func isManagedImagePullSecret(obj any) bool {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return false
	}
	return secret.Type == corev1.SecretTypeDockercfg && len(secret.Annotations[InternalRegistryAuthTokenServiceAccountAnnotation]) > 0
}

func (c *imagePullSecretController) resync() {
	secrets, err := c.secrets.List(labels.Everything())
	if err != nil {
		klog.V(1).ErrorS(err, "error listing secrets")
		return
	}
	for _, s := range secrets {
		if isManagedImagePullSecret(s) {
			key, err := cache.MetaNamespaceKeyFunc(s)
			if err == nil {
				c.queue.Add(key)
			}
		}
	}
}

const imagePullSecretControllerFieldManager = "openshift.io/image-registry-pull-secrets_image-pull-secret-controller"

func (c *imagePullSecretController) sync(ctx context.Context, key string) (error, time.Duration) {
	klog.V(4).InfoS("sync", "key", key)

	kids := c.kids.Load()
	urls := c.urls.Load()

	// if we don't have a kid yet, requeue
	if kids == nil {
		// return error to requeue
		return fmt.Errorf("service account token keys have not been observed yet"), 0
	}

	// if we don't have registry urls yet, requeue
	if urls == nil {
		// return error to requeue
		return fmt.Errorf("image registry urls have not been observed yet"), 0
	}

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err, 0
	}
	secret, err := c.secrets.Secrets(ns).Get(name)
	if errors.IsNotFound(err) {
		return nil, 0
	}
	if err != nil {
		return err, 0
	}

	orphaned, err := c.isOrphanedManagedImagePullSecret(secret)
	if err != nil {
		return err, 0
	}
	if orphaned {
		return c.cleanupOrphanedManagedImagePullSecret(ctx, secret), 0
	}

	refreshNow, refreshAt := c.isSecretRefreshNeeded(secret, *urls, *kids)
	if !refreshNow {

		// the annotation is missing or incorrect, fix it
		if secret.Annotations[InternalRegistryAuthTokenTypeAnnotation] != AuthTokenTypeBound {
			patch, err := applycorev1.ExtractSecret(secret, imagePullSecretControllerFieldManager)
			if err != nil {
				return err, 0
			}
			patch.WithAnnotations(map[string]string{InternalRegistryAuthTokenTypeAnnotation: AuthTokenTypeBound})
			_, err = c.client.CoreV1().Secrets(secret.Namespace).Apply(ctx, patch, metav1.ApplyOptions{Force: true, FieldManager: imagePullSecretControllerFieldManager})
			if err != nil {
				return err, 0
			}
		}

		// token is not expired and not expiring soon, requeue when expected to need a refresh
		refreshAfter := refreshAt.Sub(time.Now())
		klog.V(4).InfoS(key, "refreshAfter", refreshAfter)
		return nil, refreshAfter
	}

	var serviceAccountName = serviceAccountNameForManagedSecret(secret)
	klog.V(2).InfoS("Refreshing image pull secret", "ns", secret.Namespace, "name", secret.Name, "serviceaccount", serviceAccountName)

	// request new token, bound by default time and bound to this secret
	tokenRequest, err := c.client.CoreV1().ServiceAccounts(secret.Namespace).CreateToken(ctx, serviceAccountName,
		&authenticationv1.TokenRequest{Spec: authenticationv1.TokenRequestSpec{BoundObjectRef: &authenticationv1.BoundObjectReference{
			APIVersion: "v1", Kind: "Secret", Name: secret.Name, UID: secret.UID,
		}}},
		metav1.CreateOptions{},
	)
	if err != nil {
		return err, 0
	}

	// compute registry authentication data
	data, err := json.Marshal(dockerConfig(tokenRequest.Status.Token, *urls))
	if err != nil {
		return fmt.Errorf("unable to serialize registry auth data: %w", err), 0
	}

	patch := applycorev1.Secret(name, ns).
		WithAnnotations(map[string]string{
			InternalRegistryAuthTokenTypeAnnotation: AuthTokenTypeBound,
		}).
		WithType(corev1.SecretTypeDockercfg).
		WithData(map[string][]byte{corev1.DockerConfigKey: data})
	_, err = c.client.CoreV1().Secrets(secret.Namespace).Apply(ctx, patch, metav1.ApplyOptions{Force: true, FieldManager: imagePullSecretControllerFieldManager})

	if err != nil {
		return err, 0
	}

	refreshAfter := refreshAt.Sub(time.Now())
	return nil, refreshAfter
}

func (c *imagePullSecretController) cleanupOrphanedManagedImagePullSecret(ctx context.Context, secret *corev1.Secret) error {
	ns := secret.Namespace
	serviceAccountName := serviceAccountNameForManagedSecret(secret)
	if len(serviceAccountName) > 0 {
		var updateServiceAccount bool
		serviceAccount, err := c.serviceAccounts.ServiceAccounts(ns).Get(serviceAccountName)
		if errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("removing orphaned managed image pull secret from serviceaccount: %v", err)
		}
		var imagePullSecrets []corev1.LocalObjectReference
		for _, ref := range serviceAccount.ImagePullSecrets {
			if ref.Name == secret.Name {
				updateServiceAccount = true
				continue
			}
			imagePullSecrets = append(imagePullSecrets, ref)
		}
		var mountableSecrets []corev1.ObjectReference
		for _, ref := range serviceAccount.Secrets {
			if ref.Name == secret.Name {
				updateServiceAccount = true
				continue
			}
			mountableSecrets = append(mountableSecrets, ref)
		}
		if updateServiceAccount {
			sa := serviceAccount.DeepCopy()
			sa.ImagePullSecrets = imagePullSecrets
			sa.Secrets = mountableSecrets
			_, err = c.client.CoreV1().ServiceAccounts(ns).Update(ctx, sa, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("removing refrences to orphaned managed image pull secret from service account: %v", err)
			}
		}
	}
	if err := c.client.CoreV1().Secrets(ns).Delete(ctx, secret.Name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("deleting orphaned managed image pull secret: %v", err)
	}
	return nil
}

func dockerConfig(token string, urls []string) any {
	// not using credentialprovider.DockerConfig to keep redundant username/password/email out of secret
	auth := map[string]map[string]string{}
	entry := map[string]string{
		"auth": base64.StdEncoding.EncodeToString([]byte("<token>:" + token)),
	}
	for _, url := range urls {
		auth[url] = entry
	}
	return auth
}

func (c *imagePullSecretController) isSecretRefreshNeeded(secret *corev1.Secret, urls, kids []string) (bool, time.Time) {
	valid, refreshAt := c.registryAuthenticationFileValid(secret, urls, kids)
	return !valid, refreshAt
}

func (c *imagePullSecretController) registryAuthenticationFileValid(imagePullSecret *corev1.Secret, urls, kids []string) (bool, time.Time) {
	if imagePullSecret.Type != corev1.SecretTypeDockercfg {
		klog.V(2).InfoS("Internal registry pull secret type is incorrect.", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "type", imagePullSecret.Type)
		return false, time.Now()
	}
	// registry authentication file must exist
	if _, ok := imagePullSecret.Data[corev1.DockerConfigKey]; !ok {
		klog.V(2).InfoS("Internal registry pull secret does not contain the expected key", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "keys", reflect.ValueOf(imagePullSecret.Data).MapKeys())
		return false, time.Now()
	}
	// parse the registry authentication file
	auth := credentialprovider.DockerConfig{}
	if err := json.Unmarshal(imagePullSecret.Data[corev1.DockerConfigKey], &auth); err != nil {
		klog.V(2).InfoS("Internal registry pull secret auth data cannot be parsed", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name)
		return false, time.Now()
	}
	// there should be an entries for each internal registry url
	if len(auth) != len(urls) {
		klog.V(2).InfoS("Internal registry pull secret auth data does not contain the correct number of entries", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "expected", len(urls), "actual", len(auth))
		return false, time.Now()
	}
	matches := 0
CheckUrl:
	for _, url := range urls {
		for key := range auth {
			if key == url {
				matches++
				continue CheckUrl
			}
		}
	}
	if matches != len(urls) {
		klog.V(2).InfoS("Internal registry pull secret needs to be refreshed", "reason", "auth data does not contain the correct entries", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "expected", urls, "actual", reflect.ValueOf(auth).MapKeys())
		return false, time.Now()
	}

	// track the earliest refresh time of the token (they should all be the same, but check anyway)
	var requeueAt time.Time

	// check the token embedded in the registry authentication file
	for url, entry := range auth {
		token, err := jwt.ParseSigned(entry.Password)
		if err != nil {
			klog.V(2).InfoS("Internal registry pull secret needs to be refreshed", "reason", "auth token cannot be parsed", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "url", url, "error", err)
			return false, time.Now()
		}

		// was token created with previoud token signing cert?
		var validKeyID bool
		for _, kid := range kids {
			if token.Headers[0].KeyID == kid {
				validKeyID = true
				break
			}
		}
		if !validKeyID {
			klog.V(2).InfoS("Internal registry pull secret needs to be refreshed", "reason", "auth token was not signed by a current signer", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "url", url, "error", err)
			return false, time.Now()
		}

		var claims jwt.Claims
		// "unsafe" in the following API just means we are not validating the signature
		err = token.UnsafeClaimsWithoutVerification(&claims)
		if err != nil {
			klog.V(2).InfoS("Internal registry pull secret needs to be refreshed", "reason", "auth token claim cannot be parsed", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "url", url, "error", err)
			return false, time.Now()
		}
		// if token is expired or will only be valid less than 40% of its remaining duration we want to trigger a new token request
		refreshTime := claims.Expiry.Time().Add(time.Duration(-int64(float64(time.Now().Sub(claims.Expiry.Time())) * 0.4)))
		klog.V(4).InfoS("Token expiration check.", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "url", url, "expirtyTime", claims.Expiry.Time(), "refreshTime", refreshTime)
		if time.Now().After(refreshTime) {
			klog.V(2).InfoS("Internal registry pull secret needs to be refreshed", "reason", "auth token needs to be refreshed", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name, "url", url, "expirtyTime", claims.Expiry.Time(), "refreshTime", refreshTime)
			return false, time.Now()
		}
		if requeueAt.IsZero() || requeueAt.After(refreshTime) {
			requeueAt = refreshTime
		}
	}
	klog.V(4).InfoS("Internal registry pull secret does not need to be refreshed.", "ns", imagePullSecret.Namespace, "name", imagePullSecret.Name)
	return true, requeueAt
}

func (c *imagePullSecretController) Run(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()
	const name = "openshift.io/internal-image-registry-pull-secrets_image-pull-secret"
	klog.InfoS("Starting controller", "name", name)

	if !cache.WaitForNamedCacheSync(name, ctx.Done(), c.cacheSyncs...) {
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		var v []string
		for len(v) == 0 {
			klog.V(2).Info("Waiting for image registry urls to be observed")
			select {
			case v = <-c.urlsC:
				c.urls.Store(&v)
				klog.V(2).InfoS("Observed image registry urls", "urls", v)
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		var v []string
		for len(v) == 0 {
			klog.V(2).Info("Waiting for service account token signing cert to be observed")
			select {
			case v = <-c.kidsC:
				klog.V(2).InfoS("Observed service account token signing certs", "kids", v)
				c.kids.Store(&v)
			case <-ctx.Done():
				return
			}
		}
	}()
	wg.Wait()

	// start workers
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	// start observers
	go func() {
		for {
			select {
			case v := <-c.urlsC:
				if len(v) == 0 {
					klog.V(1).ErrorS(nil, "unable to observe at least one image registry url")
					continue // controller can not do anything useful without a value, so do nothing
				}
				klog.V(2).InfoS("Observed image registry urls", "urls", v)
				old := c.urls.Swap(&v)
				if !slices.Equal(*old, v) {
					c.resync()
				}
			case v := <-c.kidsC:
				if len(v) == 0 {
					klog.V(1).ErrorS(nil, "unable to observe at least one service account token signing cert")
					continue // controller can not do anything useful without a value, so do nothing
				}
				klog.V(2).InfoS("Observed service account token signing certs", "kids", v)
				old := c.kids.Swap(&v)
				if !slices.Equal(*old, v) {
					c.resync()
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	klog.InfoS("Started controller", "name", name)
	<-ctx.Done()
	klog.InfoS("Shutting down controller", "name", name)
}

func (c *imagePullSecretController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *imagePullSecretController) processNextWorkItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)
	err, requeueAfter := c.sync(ctx, key.(string))
	if err == nil {
		c.queue.Forget(key)
		if requeueAfter > 0 {
			c.queue.AddAfter(key, requeueAfter)
		}
		return true
	}
	runtime.HandleError(fmt.Errorf("%v failed with : %v", key, err))
	c.queue.AddRateLimited(key)
	return true
}

func (c *imagePullSecretController) isOrphanedManagedImagePullSecret(secret *corev1.Secret) (bool, error) {
	// the annotation referencing the service account must exist, or this controller would not of been triggered
	serviceAccount, err := c.serviceAccounts.ServiceAccounts(secret.Namespace).Get(secret.Annotations[InternalRegistryAuthTokenServiceAccountAnnotation])
	if errors.IsNotFound(err) {
		// service account does not exist, this secret should not exist either, unless the ownerrefs were clobbered
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if serviceAccount.Annotations == nil {
		// service account's secret ref annotation is missing, we take no action until it is reconciled by its owning controller
		return false, nil
	}
	secretRef, ok := serviceAccount.Annotations[InternalRegistryImagePullSecretRefKey]
	if !ok {
		// service account's secret ref annotation is missing, we take no action until it is reconciled by its owning controller
		return false, nil
	}
	// secret if considered orphaned the service account it references has a secret ref, and it does not reference the secret
	return secretRef != secret.Name, nil
}
