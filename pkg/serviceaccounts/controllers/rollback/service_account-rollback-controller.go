package rollback

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type serviceAccountRollbackController struct {
	client          kubernetes.Interface
	serviceAccounts listers.ServiceAccountLister
	secrets         listers.SecretLister
	cacheSyncs      []cache.InformerSynced
	queue           workqueue.RateLimitingInterface
}

func NewServiceAccountRollbackController(kubeclient kubernetes.Interface, serviceAccounts informers.ServiceAccountInformer, secrets informers.SecretInformer) *serviceAccountRollbackController {
	c := &serviceAccountRollbackController{
		client:          kubeclient,
		serviceAccounts: serviceAccounts.Lister(),
		secrets:         secrets.Lister(),
		cacheSyncs:      []cache.InformerSynced{serviceAccounts.Informer().HasSynced, secrets.Informer().HasSynced},
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "service-accounts"),
	}

	serviceAccounts.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj any) bool {
			sa, ok := obj.(*corev1.ServiceAccount)
			if !ok {
				return false
			}
			_, ok = sa.Annotations["openshift.io/internal-registry-pull-secret-ref"]
			return ok
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj any) {
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
		},
	})
	return c
}

func (c *serviceAccountRollbackController) sync(ctx context.Context, key string) error {
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
	imagePullSecretName := serviceAccount.Annotations["openshift.io/internal-registry-pull-secret-ref"]
	ops := []string{
		`{"op": "remove", "path": "/metadata/annotations/openshift.io~1internal-registry-pull-secret-ref"}`,
	}
	var rollbackRefs bool
	secret, err := c.secrets.Secrets(ns).Get(imagePullSecretName)
	if kerrors.IsNotFound(err) {
		rollbackRefs = true
	} else if err != nil {
		return err
	} else if _, ok := secret.Annotations["openshift.io/internal-registry-auth-token.service-account"]; ok {
		// this image pull secret is from the future
		rollbackRefs = true
	}
	if rollbackRefs {
		index := slices.IndexFunc(serviceAccount.Secrets, func(ref corev1.ObjectReference) bool {
			return ref.Name == imagePullSecretName
		})
		if index > -1 {
			ops = append(ops, fmt.Sprintf(`{"op": "test", "path": "/secrets/%d/name", "value": "%s"}`, index, imagePullSecretName))
			ops = append(ops, fmt.Sprintf(`{"op": "remove", "path": "/secrets/%d"}`, index))
		}
		index = slices.IndexFunc(serviceAccount.ImagePullSecrets, func(ref corev1.LocalObjectReference) bool {
			return ref.Name == imagePullSecretName
		})
		if index > -1 {
			ops = append(ops, fmt.Sprintf(`{"op": "test", "path": "/imagePullSecrets/%d/name", "value":"%s"}`, index, imagePullSecretName))
			ops = append(ops, fmt.Sprintf(`{"op": "remove", "path": "/imagePullSecrets/%d"}`, index))
		}
	}
	patch := "[" + strings.Join(ops, ",") + "]"
	klog.V(1).InfoS("rolling back service account", "ns", serviceAccount.Namespace, "serviceAccount", serviceAccount.Name, "rollback", imagePullSecretName)
	_, err = c.client.CoreV1().ServiceAccounts(serviceAccount.Namespace).Patch(ctx, serviceAccount.Name, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
	return err
}

func (c *serviceAccountRollbackController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	const name = "openshift.io/internal-image-registry-pull-secrets_service-account-rollback"
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

func (c *serviceAccountRollbackController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *serviceAccountRollbackController) processNextWorkItem(ctx context.Context) bool {
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
