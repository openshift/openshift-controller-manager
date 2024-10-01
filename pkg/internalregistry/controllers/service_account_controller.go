package controllers

import (
	"context"
	"fmt"
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

	"github.com/openshift/library-go/pkg/build/naming"
	"github.com/openshift/openshift-controller-manager/pkg/serviceaccounts/controllers"
)

type serviceAccountController struct {
	client          kubernetes.Interface
	serviceAccounts listers.ServiceAccountLister
	secrets         listers.SecretLister
	cacheSyncs      []cache.InformerSynced
	queue           workqueue.RateLimitingInterface
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
func NewServiceAccountController(kubeclient kubernetes.Interface, serviceAccounts informers.ServiceAccountInformer, secrets informers.SecretInformer) *serviceAccountController {
	c := &serviceAccountController{
		client:          kubeclient,
		serviceAccounts: serviceAccounts.Lister(),
		secrets:         secrets.Lister(),
		cacheSyncs:      []cache.InformerSynced{serviceAccounts.Informer().HasSynced, secrets.Informer().HasSynced},
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "service-accounts"),
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
	corev1.ServiceAccountNameKey:                   func(sa *corev1.ServiceAccount, v string) bool { return sa.Name == v },
	corev1.ServiceAccountUIDKey:                    func(sa *corev1.ServiceAccount, v string) bool { return sa.UID == types.UID(v) },
	controllers.ServiceAccountTokenSecretNameKey:   func(sa *corev1.ServiceAccount, v string) bool { return true },
	controllers.ServiceAccountTokenValueAnnotation: func(sa *corev1.ServiceAccount, v string) bool { return true },
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
