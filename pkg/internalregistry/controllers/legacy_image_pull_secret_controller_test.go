package controllers

import (
	"context"
	"testing"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// TestLegacyImagePullSecretControllerSync_DeletionPaths exercises the finalizer-removal
// logic in sync(), including the case (OCPBUGS-100179) where a secret has already
// transitioned out of the "token-secret.name" annotation state but still carries the
// openshift.io/legacy-token finalizer and a deletionTimestamp.
func TestLegacyImagePullSecretControllerSync_DeletionPaths(t *testing.T) {
	now := metav1.Now()

	mkSecret := func(opts ...func(*corev1.Secret)) *corev1.Secret {
		s := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns1",
				Name:      "dockercfg-abc",
			},
			Type: corev1.SecretTypeDockercfg,
		}
		for _, f := range opts {
			f(s)
		}
		return s
	}

	withFinalizer := func(s *corev1.Secret) {
		s.Finalizers = append(s.Finalizers, "openshift.io/legacy-token")
	}
	withDeletionTimestamp := func(s *corev1.Secret) {
		s.DeletionTimestamp = &now
	}
	withTokenAnnotation := func(s *corev1.Secret) {
		if s.Annotations == nil {
			s.Annotations = map[string]string{}
		}
		s.Annotations["openshift.io/token-secret.name"] = "token-abc"
	}

	testCases := []struct {
		name          string
		secret        *corev1.Secret // object stored in the API server
		cachedSecret  *corev1.Secret // object returned by the informer lister (nil → same as secret)
		wantFinalizer bool           // whether legacy-token should still be present after sync
		wantErr       bool
	}{
		{
			// Core fix for OCPBUGS-100179: a secret that has deletionTimestamp set and
			// the legacy-token finalizer but lacks the token-secret.name annotation
			// (i.e. has already transitioned to the "bound" auth type) must have its
			// finalizer removed so that namespace deletion can complete.
			name:          "stuck secret: deletionTimestamp set, finalizer present, no token-secret.name annotation",
			secret:        mkSecret(withDeletionTimestamp, withFinalizer),
			wantFinalizer: false,
		},
		{
			name:          "normal deletion: deletionTimestamp set, finalizer and annotation both present",
			secret:        mkSecret(withDeletionTimestamp, withFinalizer, withTokenAnnotation),
			wantFinalizer: false,
		},
		{
			name:          "deletion already complete: deletionTimestamp set, finalizer already gone",
			secret:        mkSecret(withDeletionTimestamp),
			wantFinalizer: false,
		},
		{
			// The JSON Patch "test" op verifies the finalizer is still at the cached index
			// before removing it. If the cache is stale (finalizers differ), the patch
			// fails and sync() should return an error so the item is requeued.
			name:         "cache stale: cached finalizer index does not match live object",
			secret:       mkSecret(withDeletionTimestamp, withFinalizer),
			cachedSecret: mkSecret(withDeletionTimestamp, func(s *corev1.Secret) { s.Finalizers = []string{"other", "openshift.io/legacy-token"} }),
			wantErr:      true,
		},
	}

	ctx := context.Background()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cachedSecret == nil {
				tc.cachedSecret = tc.secret
			}
			client := fake.NewClientset(tc.secret)
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			if err := indexer.Add(tc.cachedSecret); err != nil {
				t.Fatal(err)
			}
			c := legacyImagePullSecretController{
				client:  client,
				secrets: listers.NewSecretLister(indexer),
			}
			err := c.sync(ctx, tc.secret.Namespace+"/"+tc.secret.Name)
			if (err != nil) != tc.wantErr {
				t.Fatalf("sync() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			actual, err := client.CoreV1().Secrets(tc.secret.Namespace).Get(ctx, tc.secret.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			hasFinalizer := slices.Contains(actual.Finalizers, "openshift.io/legacy-token")
			if hasFinalizer != tc.wantFinalizer {
				t.Errorf("openshift.io/legacy-token finalizer present = %v, want %v", hasFinalizer, tc.wantFinalizer)
			}
		})
	}
}

// TestLegacyImagePullSecretControllerFilterFunc verifies that the informer
// FilterFunc correctly enqueues (or ignores) secrets based on the new rule:
// a Dockercfg secret being deleted that still carries the legacy-token
// finalizer must reach the queue even without the token-secret.name annotation.
func TestLegacyImagePullSecretControllerFilterFunc(t *testing.T) {
	now := metav1.Now()

	testCases := []struct {
		name         string
		secret       *corev1.Secret
		wantEnqueued bool
	}{
		{
			name: "stuck secret: deletionTimestamp, finalizer, no annotation — must be enqueued",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "ns1",
					Name:              "dockercfg-stuck",
					Finalizers:        []string{"openshift.io/legacy-token"},
					DeletionTimestamp: &now,
				},
				Type: corev1.SecretTypeDockercfg,
			},
			wantEnqueued: true,
		},
		{
			name: "live secret with annotation — must be enqueued",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "ns1",
					Name:        "dockercfg-legacy",
					Annotations: map[string]string{"openshift.io/token-secret.name": "token-abc"},
				},
				Type: corev1.SecretTypeDockercfg,
			},
			wantEnqueued: true,
		},
		{
			name: "live secret, finalizer but no annotation, no deletionTimestamp — must not be enqueued",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  "ns1",
					Name:       "dockercfg-bound",
					Finalizers: []string{"openshift.io/legacy-token"},
				},
				Type: corev1.SecretTypeDockercfg,
			},
			wantEnqueued: false,
		},
		{
			name: "wrong secret type — must not be enqueued",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "ns1",
					Name:        "opaque-secret",
					Annotations: map[string]string{"openshift.io/token-secret.name": "token-abc"},
				},
				Type: corev1.SecretTypeOpaque,
			},
			wantEnqueued: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewClientset(tc.secret)
			factory := informers.NewSharedInformerFactory(client, 0)
			secretInformer := factory.Core().V1().Secrets()

			c := NewLegacyImagePullSecretController(client, secretInformer)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			factory.Start(ctx.Done())
			if !cache.WaitForCacheSync(ctx.Done(), secretInformer.Informer().HasSynced) {
				t.Fatal("informer cache never synced")
			}

			wantKey := tc.secret.Namespace + "/" + tc.secret.Name
			if tc.wantEnqueued {
				if c.queue.Len() == 0 {
					t.Fatalf("expected %q in queue but queue is empty", wantKey)
				}
				key, _ := c.queue.Get()
				defer c.queue.Done(key)
				if key != wantKey {
					t.Errorf("queue got key %q, want %q", key, wantKey)
				}
			} else {
				if c.queue.Len() != 0 {
					t.Errorf("expected empty queue but got %d item(s)", c.queue.Len())
				}
			}
		})
	}
}
