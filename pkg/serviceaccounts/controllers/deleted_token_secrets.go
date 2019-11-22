package controllers

import (
	"fmt"
	"time"

	"k8s.io/klog"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	informers "k8s.io/client-go/informers/core/v1"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	api "k8s.io/kubernetes/pkg/apis/core"
)

// DockercfgTokenDeletedControllerOptions contains options for the DockercfgTokenDeletedController
type DockercfgTokenDeletedControllerOptions struct {
	// Resync is the time.Duration at which to fully re-list secrets.
	// If zero, re-list will be delayed as long as possible
	Resync time.Duration
}

// NewDockercfgTokenDeletedController returns a new *DockercfgTokenDeletedController.
func NewDockercfgTokenDeletedController(secrets informers.SecretInformer, cl kclientset.Interface, options DockercfgTokenDeletedControllerOptions) *DockercfgTokenDeletedController {
	e := &DockercfgTokenDeletedController{
		client: cl,
	}

	e.secretController = secrets.Informer().GetController()
	secrets.Informer().AddEventHandlerWithResyncPeriod(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				// case cache.DeletedFinalStateUnknown:
				// 	secret, ok := t.Obj.(*v1.Secret)
				// 	if !ok {
				// 		return false
				// 	}
				// 	return secret.Type == v1.SecretTypeServiceAccountToken
				case *v1.Secret:
					return t.Type == v1.SecretTypeServiceAccountToken
				default:
					utilruntime.HandleError(fmt.Errorf("object passed to %T that is not expected: %T", e, obj))
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				DeleteFunc: e.secretDeleted,
			},
		},
		options.Resync,
	)

	return e
}

// The DockercfgTokenDeletedController watches for service account tokens to be deleted.
// On delete, it removes the associated dockercfg secret if it exists.
type DockercfgTokenDeletedController struct {
	client           kclientset.Interface
	secretController cache.Controller
}

// Runs controller loops and returns on shutdown
func (e *DockercfgTokenDeletedController) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	klog.Infof("Starting DockercfgTokenDeletedController controller")
	defer klog.Infof("Shutting down DockercfgTokenDeletedController controller")

	// Wait for the stores to fill
	if !cache.WaitForCacheSync(stopCh, e.secretController.HasSynced) {
		return
	}
	klog.V(1).Infof("caches synced")

	<-stopCh
}

// secretDeleted reacts to a token secret being deleted by looking for a corresponding dockercfg secret and deleting it if it exists
func (e *DockercfgTokenDeletedController) secretDeleted(obj interface{}) {
	tokenSecret, ok := obj.(*v1.Secret)
	if !ok {
		// tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		// if !ok {
		// 	return
		// }
		// tokenSecret, ok = tombstone.Obj.(*v1.Secret)
		// if !ok {
		// 	return
		// }
		return
	}

	klog.Infof("Token secret %s/%s deleted, finding associated dockercfg secrets to delete", tokenSecret.Namespace, tokenSecret.Name)

	dockercfgSecrets, err := e.findDockercfgSecrets(tokenSecret)
	if err != nil {
		klog.Error(err)
		return
	}
	if len(dockercfgSecrets) == 0 {
		klog.Infof("No dockercfg secrets found for %s/%s", tokenSecret.Namespace, tokenSecret.Name)
		return
	}

	// remove the reference token secrets
	for _, dockercfgSecret := range dockercfgSecrets {
		klog.Infof("Deleting dockercfg secret %s/%s because associated token secret %s has been deleted.", dockercfgSecret.Namespace, dockercfgSecret.Name, tokenSecret.Name)
		if err := e.client.CoreV1().Secrets(dockercfgSecret.Namespace).Delete(dockercfgSecret.Name, nil); (err != nil) && !apierrors.IsNotFound(err) {
			utilruntime.HandleError(err)
		}
	}
}

// findDockercfgSecret checks all the secrets in the namespace to see if the token secret has any existing dockercfg secrets that reference it
func (e *DockercfgTokenDeletedController) findDockercfgSecrets(tokenSecret *v1.Secret) ([]*v1.Secret, error) {
	dockercfgSecrets := []*v1.Secret{}

	options := metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector(api.SecretTypeField, string(v1.SecretTypeDockercfg)).String()}
	potentialSecrets, err := e.client.CoreV1().Secrets(tokenSecret.Namespace).List(options)
	if err != nil {
		return nil, err
	}

	for i, currSecret := range potentialSecrets.Items {
		if currSecret.Annotations[ServiceAccountTokenSecretNameKey] == tokenSecret.Name {
			dockercfgSecrets = append(dockercfgSecrets, &potentialSecrets.Items[i])
		}
	}

	return dockercfgSecrets, nil
}
