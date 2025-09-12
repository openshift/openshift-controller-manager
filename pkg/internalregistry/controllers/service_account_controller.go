package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/build/naming"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type serviceAccountController struct {
	client          kubernetes.Interface
	dynamicClient   dynamic.Interface
	serviceAccounts listers.ServiceAccountLister
	secrets         listers.SecretLister
	cacheSyncs      []cache.InformerSynced
	queue           workqueue.RateLimitingInterface

	// Track registry state to detect changes
	lastRegistryState bool
	lastCleanupTime   time.Time
}

func serviceAccountNameForManagedSecret(secret *corev1.Secret) string {
	n := secret.Annotations[InternalRegistryAuthTokenServiceAccountAnnotation]
	if len(n) > 0 {
		return n
	}
	// legacy fallback
	return secret.Annotations[corev1.ServiceAccountNameKey]
}

// NewServiceAccountController creates a controller that for each service
// account in the cluster, creates an image pull secret that can be used
// to pull images from the integrated image registry.
func NewServiceAccountController(kubeclient kubernetes.Interface, serviceAccounts informers.ServiceAccountInformer, secrets informers.SecretInformer, dynamicClient dynamic.Interface) *serviceAccountController {
	c := &serviceAccountController{
		client:            kubeclient,
		dynamicClient:     dynamicClient,
		serviceAccounts:   serviceAccounts.Lister(),
		secrets:           secrets.Lister(),
		cacheSyncs:        []cache.InformerSynced{serviceAccounts.Informer().HasSynced, secrets.Informer().HasSynced},
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "service-accounts"),
		lastRegistryState: false, // Assume registry is enabled at startup
		lastCleanupTime:   time.Now(),
	}

	serviceAccounts.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				c.queue.Add(key)
			}
		},
		UpdateFunc: func(old any, new any) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				c.queue.Add(key)
			}
		},
	})

	secrets.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj any) bool {
			secret, ok := obj.(*corev1.Secret)
			return ok && secret.Type == corev1.SecretTypeDockercfg
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj any) {
				secret := obj.(*corev1.Secret)
				serviceAccountName := serviceAccountNameForManagedSecret(secret)
				if len(serviceAccountName) > 0 {
					key := cache.NewObjectName(secret.Namespace, serviceAccountName).String()
					c.queue.Add(key)
				}
			},
			UpdateFunc: func(old any, new any) {
				secret := old.(*corev1.Secret)
				serviceAccountName := serviceAccountNameForManagedSecret(secret)
				if len(serviceAccountName) > 0 {
					key := cache.NewObjectName(secret.Namespace, serviceAccountName).String()
					c.queue.Add(key)
				}
			},
			DeleteFunc: func(obj any) {
				var secret *corev1.Secret
				switch o := obj.(type) {
				case cache.DeletedFinalStateUnknown:
					var ok bool
					if secret, ok = o.Obj.(*corev1.Secret); !ok {
						return
					}
				case *corev1.Secret:
					secret = o
				}
				serviceAccountName := serviceAccountNameForManagedSecret(secret)
				if len(serviceAccountName) > 0 {
					key := cache.NewObjectName(secret.Namespace, serviceAccountName).String()
					c.queue.Add(key)
				}
			},
		},
	})
	return c
}

const serviceAccountControllerFieldManager = "openshift.io/image-registry-pull-secrets_service-account-controller"

func (c *serviceAccountController) sync(ctx context.Context, key string) error {
	klog.V(4).InfoS("sync", "key", key)
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	serviceAccount, err := c.serviceAccounts.ServiceAccounts(ns).Get(name)
	if kerrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	// Check registry state and handle cleanup if needed
	currentRegistryDisabled := c.isImageRegistryDisabled(ctx)
	if currentRegistryDisabled {
		// If registry is currently disabled, check if we need to run cleanup
		shouldCleanup := false

		// Cleanup if registry state changed from enabled to disabled
		if !c.lastRegistryState && currentRegistryDisabled {
			klog.V(4).Infof("Registry state changed from enabled to disabled, triggering cleanup")
			shouldCleanup = true
		}

		// Also cleanup periodically (every 5 minutes) when registry is disabled
		// to handle cases where the controller missed the state change
		if time.Since(c.lastCleanupTime) > 5*time.Minute {
			klog.V(4).Infof("Periodic cleanup triggered (last cleanup: %v ago)", time.Since(c.lastCleanupTime))
			shouldCleanup = true
		}

		if shouldCleanup {
			if err := c.cleanupExistingDockercfgSecrets(ctx); err != nil {
				klog.Warningf("Failed to cleanup dockercfg secrets: %v", err)
				// Don't return error to avoid blocking service account processing
			}
			c.lastCleanupTime = time.Now()
		}
	}

	// Update last known registry state
	c.lastRegistryState = currentRegistryDisabled

	// name of managed secret
	secretName, err := c.managedImagePullSecretName(ctx, serviceAccount)

	patch := applycorev1.ServiceAccount(name, ns).
		WithAnnotations(map[string]string{InternalRegistryImagePullSecretRefKey: secretName}).
		// TODO stop adding to .Secrets part of API-1798
		WithSecrets(applycorev1.ObjectReference().WithName(secretName))

	// Extract an apply patch from the service account. The extracted patch is used
	// to deduce some field ownership information without having to parse
	// ManagedFields, and it must remain unmodified so we can detect if the apply
	// call is necessary.
	extracted, err := applycorev1.ExtractServiceAccount(serviceAccount, serviceAccountControllerFieldManager)
	if err != nil {
		return err
	}

	// As an atomic field, an update to ImagePullSecrets replaces the the entire
	// field. We want to be cautious about not overwriting the changes made to
	// ImagePullSecrets by other users or controllers. When updating ImagePullSecrets
	// always add the ResourceVersion to the patch to ensure the apply fails if the
	// cache was stale.
	if slices.ContainsFunc(serviceAccount.ImagePullSecrets, func(ref corev1.LocalObjectReference) bool { return ref.Name == secretName }) {
		// If this controller owns the field, we copy the value from the extracted patch
		// to preserve it upon apply with force=true. If this controller does not own the
		// field, the copied extracted value should be nil, also preserving the existing
		// value upon apply with force=true.
		patch.ImagePullSecrets = extracted.ImagePullSecrets
	} else {
		// preserve the existing ImagePullSecrets items and add the managed image pull secret.
		for _, ref := range serviceAccount.ImagePullSecrets {
			patch.WithImagePullSecrets(applycorev1.LocalObjectReference().WithName(ref.Name))
		}
		patch.WithImagePullSecrets(applycorev1.LocalObjectReference().WithName(secretName))
	}

	// apply patch only if necessary
	if !equality.Semantic.DeepEqual(patch, extracted) {
		// add the UID to the patch to ensure we don't re-create the service account if it no longer exists
		patch.WithUID(serviceAccount.UID)
		if len(patch.ImagePullSecrets) > 0 {
			// prevent inadvertently overwriting someone else's updates to ImagePullSecrets
			patch.WithResourceVersion(serviceAccount.ResourceVersion)
		}
		serviceAccount, err = c.client.CoreV1().ServiceAccounts(ns).Apply(ctx, patch, metav1.ApplyOptions{Force: true, FieldManager: serviceAccountControllerFieldManager})
		if err != nil {
			return err
		}
	}

	// get the managed image pull secret
	secret, err := c.secrets.Secrets(serviceAccount.Namespace).Get(secretName)
	if err != nil && !kerrors.IsNotFound(err) {
		return err
	}

	// nothing more to do if the manged secret is a lecacy image pull secret
	if secret != nil && isLegacyImagePullSecret(secret) {
		return nil
	}

	// if secret does not exist, or owner reference is missing, apply secret
	var secretOwnerRefNeedsUpdate, secretSARefNeedsUpdate bool
	if secret != nil {
		secretSARefNeedsUpdate = secret.Annotations[InternalRegistryAuthTokenServiceAccountAnnotation] != serviceAccount.Name
		secretOwnerRefNeedsUpdate = true
		for _, ref := range secret.OwnerReferences {
			if ref.Name == serviceAccount.Name && ref.UID == serviceAccount.UID && ref.Kind == "ServiceAccount" && ref.APIVersion == "v1" {
				secretOwnerRefNeedsUpdate = false
				break
			}
		}
	}
	// PREVENTION FIX: Check if image registry is disabled before creating dockercfg secrets
	if c.isImageRegistryDisabled(ctx) {
		klog.V(4).Infof("Skipping dockercfg secret creation for service account %s/%s - image registry is disabled", serviceAccount.Namespace, serviceAccount.Name)
		return nil
	}
	if secret == nil || secretSARefNeedsUpdate || secretOwnerRefNeedsUpdate {
		patch := applycorev1.Secret(secretName, ns).
			WithAnnotations(map[string]string{
				InternalRegistryAuthTokenServiceAccountAnnotation: serviceAccount.Name,
			}).
			WithOwnerReferences(
				applymetav1.OwnerReference().
					WithAPIVersion("v1").
					WithKind("ServiceAccount").
					WithName(serviceAccount.Name).
					WithUID(serviceAccount.UID),
			).
			WithType(corev1.SecretTypeDockercfg).
			WithData(map[string][]byte{corev1.DockerConfigKey: []byte("{}")})
		if secret != nil {
			patch.WithData(map[string][]byte{corev1.DockerConfigKey: secret.Data[corev1.DockerConfigKey]})
		}
		secret, err = c.client.CoreV1().Secrets(serviceAccount.Namespace).Apply(ctx, patch, metav1.ApplyOptions{FieldManager: serviceAccountControllerFieldManager, Force: true})
		if err != nil {
			return fmt.Errorf("unable to update managed image pull secret: %v", err)
		}
	}

	return nil
}

func ownerReferenceDeSynced(serviceAccount *corev1.ServiceAccount, secret *corev1.Secret) bool {
	for _, ref := range secret.OwnerReferences {
		if ref.Name == serviceAccount.Name && ref.UID == serviceAccount.UID && ref.Kind == "ServiceAccount" && ref.APIVersion == "v1" {
			return true
		}
	}
	return false
}

func (c *serviceAccountController) managedImagePullSecretName(ctx context.Context, serviceAccount *corev1.ServiceAccount) (string, error) {
	// happy path
	name := serviceAccount.Annotations[InternalRegistryImagePullSecretRefKey]
	if len(name) != 0 {
		return name, nil
	}
	// maybe the annotation was clobbered, look for an existing managed image pull secret
	secrets, err := c.secrets.Secrets(serviceAccount.Namespace).List(labels.Everything())
	if err != nil {
		return "", err
	}
	for _, secret := range secrets {
		if secret.Type != corev1.SecretTypeDockercfg {
			continue
		}
		if secret.Annotations != nil {
			if sa, ok := secret.Annotations[InternalRegistryAuthTokenServiceAccountAnnotation]; ok {
				if sa == serviceAccount.Name {
					return secret.Name, nil
				}
				continue
			}
		}
		for _, ref := range secret.OwnerReferences {
			if ref.Name == serviceAccount.Name && ref.UID == serviceAccount.UID {
				return secret.Name, nil
			}
		}
	}
	// try to reuse the legacy image pull secret name.
	name, err = c.legacyImagePullSecretName(ctx, serviceAccount)
	if err != nil {
		return "", err
	}
	if len(name) > 0 {
		return name, nil
	}
	// no existing name found, generate one
	name = names.SimpleNameGenerator.GenerateName(naming.GetName(serviceAccount.Name, "dockercfg-", 58))
	return name, nil
}

func (c *serviceAccountController) legacyImagePullSecretName(ctx context.Context, serviceAccount *corev1.ServiceAccount) (string, error) {
	// find a legacy image pull secret in the same namespace
	for _, ref := range serviceAccount.ImagePullSecrets {
		secret, err := c.secrets.Secrets(serviceAccount.Namespace).Get(ref.Name)
		if kerrors.IsNotFound(err) {
			// reference image pull secret does not exist, ignore
			continue
		}
		if err != nil {
			return "", err
		}
		if isLegacyImagePullSecretForServiceAccount(secret, serviceAccount) {
			// return the first one found
			klog.V(1).InfoS("found legacy managed image pull secret", "ns", serviceAccount.Namespace, "serviceAccount", serviceAccount.Name, "secret", secret.Name)
			return secret.Name, nil
		}
	}
	return "", nil
}

var expectedLegacyAnnotations = map[string]func(*corev1.ServiceAccount, string) bool{
	corev1.ServiceAccountNameKey:      func(sa *corev1.ServiceAccount, v string) bool { return sa.Name == v },
	corev1.ServiceAccountUIDKey:       func(sa *corev1.ServiceAccount, v string) bool { return sa.UID == types.UID(v) },
	"openshift.io/token-secret.name":  func(sa *corev1.ServiceAccount, v string) bool { return true },
	"openshift.io/token-secret.value": func(sa *corev1.ServiceAccount, v string) bool { return true },
}

func isLegacyImagePullSecret(secret *corev1.Secret) bool {
	for key := range expectedLegacyAnnotations {
		if _, ok := secret.Annotations[key]; !ok {
			return false
		}
	}
	return true
}

func isLegacyImagePullSecretForServiceAccount(secret *corev1.Secret, serviceAccount *corev1.ServiceAccount) bool {
	for key, valueOK := range expectedLegacyAnnotations {
		value, ok := secret.Annotations[key]
		if !ok {
			return false
		}
		if !valueOK(serviceAccount, value) {
			return false
		}
	}
	return true
}

func (c *serviceAccountController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	const name = "openshift.io/internal-image-registry-pull-secrets_service-account"
	klog.InfoS("Starting controller", "name", name)
	if !cache.WaitForNamedCacheSync(name, ctx.Done(), c.cacheSyncs...) {
		return
	}
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}
	klog.InfoS("Started controller", "name", name)
	<-ctx.Done()
	klog.InfoS("Shutting down controller", "name", name)
}

func (c *serviceAccountController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem deals with one key off the queue.  It returns false
// when it's time to quit.
func (c *serviceAccountController) processNextWorkItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)
	err := c.sync(ctx, key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}
	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", key, err))
	c.queue.AddRateLimited(key)
	return true
}

// isImageRegistryDisabled checks if the image registry is disabled (managementState: Removed)
// This prevents creating dockercfg secrets when registry is not available
func (c *serviceAccountController) isImageRegistryDisabled(ctx context.Context) bool {
	// Define the GroupVersionResource for imageregistry configs
	gvr := schema.GroupVersionResource{
		Group:    "imageregistry.operator.openshift.io",
		Version:  "v1",
		Resource: "configs",
	}

	// Get the cluster image registry configuration using dynamic client
	unstructuredConfig, err := c.dynamicClient.Resource(gvr).Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		// If we can't get the registry config, assume registry is enabled (safe fallback)
		klog.V(4).Infof("Failed to get image registry config, assuming registry is enabled: %v", err)
		return false
	}

	// Extract managementState from the unstructured object
	managementState, found, err := unstructured.NestedString(unstructuredConfig.Object, "spec", "managementState")
	if err != nil || !found {
		klog.V(4).Infof("Failed to get managementState from registry config, assuming registry is enabled: err=%v, found=%t", err, found)
		return false
	}

	// Check if registry is disabled (managementState: Removed)
	isDisabled := managementState == string(operatorv1.Removed)

	if isDisabled {
		klog.V(4).Infof("Image registry is disabled (managementState: %s)", managementState)
	} else {
		klog.V(6).Infof("Image registry is enabled (managementState: %s)", managementState)
	}

	return isDisabled
}

// cleanupExistingDockercfgSecrets removes all existing secrets when registry is disabled
// This is called when the registry state changes from enabled to disabled
func (c *serviceAccountController) cleanupExistingDockercfgSecrets(ctx context.Context) error {
	klog.V(4).Info("Cleaning up existing secrets due to registry removal")

	// Get all namespaces
	namespaces, err := c.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	var deletedCount int

	for _, ns := range namespaces.Items {
		// Skip system namespaces that might need dockercfg for other purposes
		if c.isSystemNamespace(ns.Name) {
			klog.V(6).Infof("Skipping system namespace %s during cleanup", ns.Name)
			continue
		}

		// Get all secrets in this namespace
		secrets, err := c.client.CoreV1().Secrets(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Warningf("Failed to list secrets in namespace %s: %v", ns.Name, err)
			continue
		}

		for _, secret := range secrets.Items {
			klog.V(4).Infof("Deleting secret %s/%s (type: %s)", secret.Namespace, secret.Name, secret.Type)

			err := c.client.CoreV1().Secrets(secret.Namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{})
			if err != nil && !kerrors.IsNotFound(err) {
				klog.Warningf("Failed to delete secret %s/%s: %v", secret.Namespace, secret.Name, err)
				continue
			}
			deletedCount++
		}
	}

	klog.Infof("Cleaned up %d secrets due to registry removal", deletedCount)
	return nil
}

// isSystemNamespace identifies system namespaces that should be preserved during cleanup
func (c *serviceAccountController) isSystemNamespace(namespace string) bool {
	systemNamespaces := []string{
		"kube-system",
		"kube-public",
		"openshift-image-registry",
		"openshift-image-registry-operator",
		"openshift-config",
		"openshift-config-managed",
		"openshift-controller-manager",
	}

	for _, sysNS := range systemNamespaces {
		if namespace == sysNS {
			return true
		}
	}

	// Be cautious with openshift-* namespaces
	return strings.HasPrefix(namespace, "openshift-")
}
