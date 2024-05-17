package controllers

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const legacyImagePullSecretControllerFieldManager = "openshift.io/image-registry-pull-secrets_legacy-token-secrets-controller"

type legacyImagePullSecretController struct {
	client     kubernetes.Interface
	secrets    listers.SecretLister
	cacheSyncs []cache.InformerSynced
	queue      workqueue.RateLimitingInterface
}

func NewLegacyImagePullSecretController(client kubernetes.Interface, secrets informers.SecretInformer) *legacyImagePullSecretController {
	c := &legacyImagePullSecretController{
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
			if _, ok = secret.Annotations["openshift.io/token-secret.name"]; !ok {
				// does not appear to be a legacy managed image pull secret
				return false
			}
			return true
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

func (c *legacyImagePullSecretController) sync(ctx context.Context, key string) error {
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

	if !secret.DeletionTimestamp.IsZero() {
		// legacy image pull secret is being deleted, delete the corresponding legacy token secret
		if slices.Contains(secret.Finalizers, "openshift.io/legacy-token") {
			t := secret.Annotations["openshift.io/token-secret.name"]
			if len(t) > 0 {
				err := c.client.CoreV1().Secrets(ns).Delete(ctx, t, metav1.DeleteOptions{})
				if err != nil && !errors.IsNotFound(err) {
					return err
				}
			}
			// either no token secret was specified, or it was successfully deleted. clear finalizer
			var finalizers []string
			for _, f := range secret.Finalizers {
				if f != "openshift.io/legacy-token" {
					finalizers = append(finalizers, f)
				}
			}
			patch := applycorev1.Secret(name, ns).
				WithAnnotations(map[string]string{InternalRegistryAuthTokenTypeAnnotation: AuthTokenTypeLegacy}).
				WithFinalizers(finalizers...)
			_, err = c.client.CoreV1().Secrets(ns).Apply(ctx, patch, metav1.ApplyOptions{FieldManager: legacyTokenSecretControllerFieldManager})
			return err
		}
		// finalizer has already been removed, nothing to do, delete in progress
		return nil
	}
	patch := applycorev1.Secret(name, ns).
		WithAnnotations(map[string]string{InternalRegistryAuthTokenTypeAnnotation: AuthTokenTypeLegacy}).
		WithFinalizers("openshift.io/legacy-token")

	_, err = c.client.CoreV1().Secrets(ns).Apply(ctx, patch, metav1.ApplyOptions{Force: true, FieldManager: legacyImagePullSecretControllerFieldManager})
	return err
}

func (c *legacyImagePullSecretController) Run(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()
	const name = "openshift.io/internal-image-registry-pull-secrets_legacy-image-pull-secret"
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

func (c *legacyImagePullSecretController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *legacyImagePullSecretController) processNextWorkItem(ctx context.Context) bool {
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
