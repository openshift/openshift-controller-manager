package controllers

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type keyIDObservation struct {
	secrets    listers.SecretLister
	cacheSyncs []cache.InformerSynced
	queue      workqueue.RateLimitingInterface
	ch         kidsChan
}

func NewKeyIDObservationController(secrets informers.SecretInformer, ch kidsChan) *keyIDObservation {
	c := &keyIDObservation{
		secrets:    secrets.Lister(),
		ch:         ch,
		cacheSyncs: []cache.InformerSynced{secrets.Informer().HasSynced},
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "sa-signing-key-secrets"),
	}
	secrets.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj any) bool {
			if secret, ok := obj.(*corev1.Secret); ok {
				return (secret.Namespace == "openshift-kube-apiserver") && (secret.Name == "bound-service-account-signing-key")
			}
			return false
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj any) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err == nil {
					c.queue.Add(key)
				}
			},
			UpdateFunc: func(_, obj any) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err == nil {
					c.queue.Add(key)
				}
			},
		},
	})
	return c
}

func (c *keyIDObservation) sync(ctx context.Context, key string) error {
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
	pem, ok := secret.Data["service-account.pub"]
	if !ok {
		return fmt.Errorf("expected data service-account.pub not found")
	}
	keys, err := keyutil.ParsePublicKeysPEM(pem)
	if err != nil {
		return err
	}
	// compute key ID (e.g. hash) for token signing private keys from the public keys
	var kids []string
	for _, key := range keys {
		der, err := x509.MarshalPKIXPublicKey(key)
		if err != nil {
			return err
		}
		hashFunc := crypto.SHA256.New()
		hashFunc.Reset()
		_, err = hashFunc.Write(der)
		if err != nil {
			return err
		}
		kids = append(kids, base64.RawURLEncoding.EncodeToString(hashFunc.Sum(nil)))
	}
	slices.Sort(kids)
	select {
	case c.ch <- kids:
	case <-ctx.Done():
	}
	return nil
}

func (c *keyIDObservation) Run(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()
	const name = "openshift.io/internal-image-registry-pull-secrets_kids"
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

func (c *keyIDObservation) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *keyIDObservation) processNextWorkItem(ctx context.Context) bool {
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
