package rollback

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type legacyImagePullSecretRollbackController struct {
	client     kubernetes.Interface
	secrets    listers.SecretLister
	cacheSyncs []cache.InformerSynced
	queue      workqueue.RateLimitingInterface
}

func NewLegacyImagePullSecretRollbackController(client kubernetes.Interface, secrets informers.SecretInformer) *legacyImagePullSecretRollbackController {
	c := &legacyImagePullSecretRollbackController{
		client:     client,
		secrets:    secrets.Lister(),
		cacheSyncs: []cache.InformerSynced{secrets.Informer().HasSynced},
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "legacy-image-pull-secrets"),
	}
	secrets.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj any) bool {
			secret, ok := obj.(*corev1.Secret)
			if !ok {
				return false
			}
			if secret.Type != corev1.SecretTypeDockercfg {
				// not an image pull secret
				return false
			}
			return slices.Contains(secret.Finalizers, "openshift.io/legacy-token")
		},
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
	return c
}

func (c *legacyImagePullSecretRollbackController) sync(ctx context.Context, key string) error {
	klog.V(4).InfoS("sync", "key", key)
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	secret, err := c.secrets.Secrets(ns).Get(name)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	index := slices.Index(secret.Finalizers, "openshift.io/legacy-token")
	if index < 0 {
		return nil
	}
	patch := fmt.Sprintf(`[`+
		`{"op": "test", "path": "/metadata/finalizers/%d", "value": "%s"},`+
		`{"op": "remove", "path": "/metadata/finalizers/%[1]d"}`+
		`]`, index, "openshift.io/legacy-token")
	klog.V(1).InfoS("rolling back legacy managed image pull secret", "ns", secret.Namespace, "serviceAccount", secret.Name)
	_, err = c.client.CoreV1().Secrets(secret.Namespace).Patch(ctx, secret.Name, types.JSONPatchType, []byte(patch), v1.PatchOptions{})
	return err
}

func (c *legacyImagePullSecretRollbackController) Run(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()
	const name = "openshift.io/internal-image-registry-pull-secrets_legacy-image-pull-secret-rollback"
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

func (c *legacyImagePullSecretRollbackController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *legacyImagePullSecretRollbackController) processNextWorkItem(ctx context.Context) bool {
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
	runtime.HandleError(fmt.Errorf("%v failed with : %v", key, err))
	c.queue.AddRateLimited(key)
	return true
}
