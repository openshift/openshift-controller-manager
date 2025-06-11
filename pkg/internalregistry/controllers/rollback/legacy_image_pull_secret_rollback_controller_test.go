package rollback

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func TestLegacyImagePullSecretRollbackControllerSync(t *testing.T) {

	secret := func(opts ...func(*corev1.Secret)) *corev1.Secret {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "test"}}
		for _, f := range opts {
			f(s)
		}
		return s
	}
	withFinalizers := func(f ...string) func(*corev1.Secret) {
		return func(s *corev1.Secret) {
			s.ObjectMeta.Finalizers = append(s.ObjectMeta.Finalizers, f...)
		}
	}

	testCases := []struct {
		name         string
		secret       *corev1.Secret
		cachedSecret *corev1.Secret
		expected     *corev1.Secret
		expectErr    bool
	}{
		{
			name:     "no finalizer",
			secret:   secret(),
			expected: secret(),
		},
		{
			name:     "with finalizer",
			secret:   secret(withFinalizers("openshift.io/legacy-token")),
			expected: secret(),
		},
		{
			name:     "other finalizer",
			secret:   secret(withFinalizers("test")),
			expected: secret(withFinalizers("test")),
		},
		{
			name:     "mixed finalizers",
			secret:   secret(withFinalizers("test", "openshift.io/legacy-token", "test2")),
			expected: secret(withFinalizers("test", "test2")),
		},
		{
			name:         "cache behind",
			secret:       secret(withFinalizers("test", "openshift.io/legacy-token", "test2")),
			cachedSecret: secret(withFinalizers("openshift.io/legacy-token", "test3")),
			expectErr:    true,
		},
	}

	ctx := context.Background()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cachedSecret == nil {
				tc.cachedSecret = tc.secret
			}
			client := fake.NewSimpleClientset(tc.secret)
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			if err := indexer.Add(tc.cachedSecret); err != nil {
				t.Fatal(err)
			}
			c := legacyImagePullSecretRollbackController{
				client:  client,
				secrets: listers.NewSecretLister(indexer),
			}
			err := c.sync(ctx, tc.secret.Namespace+"/"+tc.secret.Name)
			if err != nil {
				if !tc.expectErr {
					t.Fatal(err)
				}
				return
			}
			if tc.expectErr {
				t.Fatal("expected error")
			}
			actual, err := client.CoreV1().Secrets(tc.secret.Namespace).Get(ctx, tc.secret.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if !equality.Semantic.DeepEqual(tc.expected, actual) {
				if len(actual.Finalizers) == 0 {
					// normalize
					actual.Finalizers = nil
				}
				t.Fatal(cmp.Diff(tc.expected, actual))
			}

		})
	}
}
