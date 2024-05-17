package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1 "k8s.io/client-go/applyconfigurations/core/v1"
	informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const legacyTokenSecretControllerFieldManager = "openshift.io/image-registry-pull-secrets_legacy-token-secrets-controller"

type legacyTokenSecretController struct {
	client     kubernetes.Interface
	secrets    listers.SecretLister
	cacheSyncs []cache.InformerSynced
	queue      workqueue.RateLimitingInterface
}

func NewLegacyTokenSecretController(client kubernetes.Interface, secrets informers.SecretInformer) *legacyTokenSecretController {
	c := &legacyTokenSecretController{
		client:     client,
		secrets:    secrets.Lister(),
		cacheSyncs: []cache.InformerSynced{secrets.Informer().HasSynced},
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "legacy-service-account-token-secrets"),
	}
	secrets.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj any) bool {
			secret, ok := obj.(*corev1.Secret)
			if !ok {
				return false
			}
			if secret.Type != corev1.SecretTypeServiceAccountToken {
				// not a service account token
				return false
			}
			if _, ok := secret.Labels["openshift.io/legacy-token"]; ok {
				// already has the needed label
				return false
			}
			if secret.Annotations["kubernetes.io/created-by"] != "openshift.io/create-dockercfg-secrets" {
				// not a secret previously managed by openshift-controller-manager
				return false
			}
			if _, ok := secret.Annotations[corev1.ServiceAccountNameKey]; !ok {
				// not expected, can't handle
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

func (c *legacyTokenSecretController) sync(ctx context.Context, key string) error {
	klog.V(4).InfoS("secret", "sync", key)
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	_, err = c.secrets.Secrets(ns).Get(name)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	patch := v1.Secret(name, ns)
	patch.WithLabels(map[string]string{"openshift.io/legacy-token": "true"})
	_, err = c.client.CoreV1().Secrets(ns).Apply(ctx, patch, metav1.ApplyOptions{Force: true, FieldManager: legacyTokenSecretControllerFieldManager})
	return err
}

func (c *legacyTokenSecretController) Run(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()
	const name = "openshift.io/internal-image-registry-pull-secrets_legacy-token-secret"
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

func (c *legacyTokenSecretController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *legacyTokenSecretController) processNextWorkItem(ctx context.Context) bool {
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
