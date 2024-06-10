package controllers

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	"gopkg.in/go-jose/go-jose.v2/jwt"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type keyIDObservation struct {
	client     kubernetes.Interface
	secrets    listers.SecretLister
	cacheSyncs []cache.InformerSynced
	queue      workqueue.RateLimitingInterface
	ch         kidsChan
}

func NewKeyIDObservationController(kubeclient kubernetes.Interface, secrets informers.SecretInformer, ch kidsChan) *keyIDObservation {
	c := &keyIDObservation{
		client:     kubeclient,
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

func (c *keyIDObservation) fallbackWorker(ctx context.Context) {
	// if the signing key secret exists, return until next call of this method.
	// if an error occurs trying to get the hash, retry for 1 minute before giving up until the next call of this method.
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := c.secrets.Secrets("openshift-kube-apiserver").Get("bound-service-account-signing-key")
		if err == nil {
			// signing key secret exists, skip and let sync handle
			return true, nil
		}
		if !errors.IsNotFound(err) {
			// something went wrong, retry
			runtime.HandleError(err)
			return false, nil
		}
		// signing key secret not found, continue

		// create a throwaway API token
		expirationSeconds := int64(10 * time.Minute / time.Second)
		tokenRequest := &authenticationv1.TokenRequest{Spec: authenticationv1.TokenRequestSpec{Audiences: []string{"not-api"}, ExpirationSeconds: &expirationSeconds}}
		tokenRequest, err = c.client.CoreV1().ServiceAccounts("default").CreateToken(ctx, "default", tokenRequest, metav1.CreateOptions{})
		if err != nil {
			runtime.HandleError(fmt.Errorf("unable to create throw-away token: %w", err))
			return false, nil
		}

		// parse token and extract the kids
		token, err := jwt.ParseSigned(tokenRequest.Status.Token)
		if err != nil {
			runtime.HandleError(fmt.Errorf("unable to parse throw-away token: %w", err))
			return false, nil
		}
		var kids []string
		for _, header := range token.Headers {
			kids = append(kids, header.KeyID)
		}
		slices.Sort(kids)
		select {
		case c.ch <- kids:
		case <-ctx.Done():
		}
		return true, nil
	})
	if err != nil {
		runtime.HandleError(err)
	}
}

func (c *keyIDObservation) Run(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()
	const name = "openshift.io/internal-image-registry-pull-secrets_kids"
	klog.InfoS("Starting controller", "name", name)
	if !cache.WaitForNamedCacheSync(name, ctx.Done(), c.cacheSyncs...) {
		return
	}
	go wait.UntilWithContext(ctx, c.fallbackWorker, 1*time.Minute)
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
