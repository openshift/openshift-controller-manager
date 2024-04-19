package rollback

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func withAnnotation[T metav1.Object](k, v string) func(T) {
	return func(s T) {
		if s.GetAnnotations() == nil {
			s.SetAnnotations(map[string]string{})
		}
		s.GetAnnotations()[k] = v
	}
}

func TestServiceAccountRollbackControllerSync(t *testing.T) {

	secret := func(opts ...func(*corev1.Secret)) *corev1.Secret {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "test_dockercfg_0"}}
		for _, f := range opts {
			f(s)
		}
		return s
	}

	withServiceAccountAnnotation := func(sa string) func(*corev1.Secret) {
		return withAnnotation[*corev1.Secret]("openshift.io/internal-registry-auth-token.service-account", sa)
	}

	serviceAccount := func(opts ...func(*corev1.ServiceAccount)) *corev1.ServiceAccount {
		s := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "test"}}
		for _, f := range opts {
			f(s)
		}
		return s
	}

	withImagePullSecretAnnotation := func(secret string) func(*corev1.ServiceAccount) {
		return withAnnotation[*corev1.ServiceAccount]("openshift.io/internal-registry-pull-secret-ref", secret)
	}

	withImagePullSecrets := func(secrets ...string) func(*corev1.ServiceAccount) {
		return func(sa *corev1.ServiceAccount) {
			for _, s := range secrets {
				sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{Name: s})
			}
		}
	}

	withSecrets := func(secrets ...string) func(*corev1.ServiceAccount) {
		return func(sa *corev1.ServiceAccount) {
			for _, s := range secrets {
				sa.Secrets = append(sa.Secrets, corev1.ObjectReference{Name: s})
			}
		}
	}

	testCases := []struct {
		name                 string
		secret               *corev1.Secret
		serviceAccount       *corev1.ServiceAccount
		cachedServiceAccount *corev1.ServiceAccount
		expected             *corev1.ServiceAccount
		expectErr            bool
	}{
		{
			name: "existing future image pull secret refs in secrets imagepullsecrets",
			serviceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withImagePullSecrets("test_dockercfg_0"),
				withSecrets("test_dockercfg_0"),
			),
			secret:   secret(withServiceAccountAnnotation("test")),
			expected: serviceAccount(),
		},
		{
			name: "existing future image pull secret refs in secrets imagepullsecrets variation",
			serviceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withImagePullSecrets("a", "test_dockercfg_0"),
				withSecrets("a", "test_dockercfg_0", "b"),
			),
			secret: secret(withServiceAccountAnnotation("test")),
			expected: serviceAccount(
				withImagePullSecrets("a"),
				withSecrets("a", "b"),
			),
		},
		{
			name: "existing future image pull secret refs in imagepullsecrets",
			serviceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withImagePullSecrets("test_dockercfg_0"),
			),
			secret:   secret(withServiceAccountAnnotation("test")),
			expected: serviceAccount(),
		},
		{
			name: "existing future image pull secret refs in secrets",
			serviceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withSecrets("test_dockercfg_0"),
			),
			secret:   secret(withServiceAccountAnnotation("test")),
			expected: serviceAccount(),
		},
		{
			name: "existing future image pull secret refs in none",
			serviceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
			),
			secret:   secret(withServiceAccountAnnotation("test")),
			expected: serviceAccount(),
		},
		{
			name: "deleted future image pull secret refs in secrets imagepullsecrets",
			serviceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withImagePullSecrets("test_dockercfg_0"),
				withSecrets("test_dockercfg_0"),
			),
			secret:   nil,
			expected: serviceAccount(),
		},
		{
			name: "deleted future image pull secret refs in imagepullsecrets",
			serviceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withImagePullSecrets("test_dockercfg_0"),
			),
			secret:   nil,
			expected: serviceAccount(),
		},
		{
			name: "deleted future image pull secret refs in secrets",
			serviceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withSecrets("test_dockercfg_0"),
			),
			secret:   nil,
			expected: serviceAccount(),
		},
		{
			name: "deleted future image pull secret refs in none",
			serviceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
			),
			secret:   nil,
			expected: serviceAccount(),
		},
		{
			name: "not future image pull secret refs in secrets imagepullsecrets",
			serviceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withImagePullSecrets("test_dockercfg_0"),
				withSecrets("test_dockercfg_0"),
			),
			secret: secret(),
			expected: serviceAccount(
				withImagePullSecrets("test_dockercfg_0"),
				withSecrets("test_dockercfg_0"),
			),
		},
		{
			name: "existing future image pull secret refs in imagepullsecrets",
			serviceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withImagePullSecrets("test_dockercfg_0"),
			),
			secret:   secret(withServiceAccountAnnotation("test")),
			expected: serviceAccount(),
		},
		{
			name: "existing future image pull secret refs in secrets",
			serviceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withSecrets("test_dockercfg_0"),
			),
			secret:   secret(withServiceAccountAnnotation("test")),
			expected: serviceAccount(),
		},
		{
			name: "cache stale secrets imagepullsecrets",
			serviceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withImagePullSecrets("test_dockercfg_1", "test_dockercfg_0"),
				withSecrets("test_dockercfg_0"),
			),
			cachedServiceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withImagePullSecrets("test_dockercfg_0"),
				withSecrets("test_dockercfg_1", "test_dockercfg_0"),
			),
			secret:    secret(withServiceAccountAnnotation("test")),
			expectErr: true,
		},
		{
			name: "cache stale imagepullsecrets",
			serviceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withImagePullSecrets("test_dockercfg_1", "test_dockercfg_0"),
			),
			cachedServiceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withImagePullSecrets("test_dockercfg_0"),
			),
			secret:    secret(withServiceAccountAnnotation("test")),
			expectErr: true,
		},
		{
			name: "cache stale secrets",
			serviceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withSecrets("test_dockercfg_1", "test_dockercfg_0"),
			),
			cachedServiceAccount: serviceAccount(
				withImagePullSecretAnnotation("test_dockercfg_0"),
				withSecrets("test_dockercfg_0"),
			),
			secret:    secret(withServiceAccountAnnotation("test")),
			expectErr: true,
		},
	}

	ctx := context.Background()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cachedServiceAccount == nil {
				tc.cachedServiceAccount = tc.serviceAccount
			}
			objects := []runtime.Object{tc.serviceAccount}
			if tc.secret != nil {
				objects = append(objects, tc.secret)
			}
			client := fake.NewSimpleClientset(objects...)
			secretsIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			if tc.secret != nil {
				if err := secretsIndexer.Add(tc.secret); err != nil {
					t.Fatal(err)
				}
			}
			serviceAccountsIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			if err := serviceAccountsIndexer.Add(tc.cachedServiceAccount); err != nil {
				t.Fatal(err)
			}

			controller := &serviceAccountRollbackController{
				client:          client,
				serviceAccounts: listers.NewServiceAccountLister(serviceAccountsIndexer),
				secrets:         listers.NewSecretLister(secretsIndexer),
			}

			err := controller.sync(ctx, tc.serviceAccount.Namespace+"/"+tc.serviceAccount.Name)
			if err != nil {
				if !tc.expectErr {
					t.Fatal(err)
				}
				return
			}
			if tc.expectErr {
				t.Fatal("expected error")
			}

			actual, err := client.CoreV1().ServiceAccounts(tc.serviceAccount.Namespace).Get(ctx, tc.serviceAccount.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}

			if !equality.Semantic.DeepEqual(tc.expected, actual) {
				t.Fatal(cmp.Diff(tc.expected, actual))
			}

		})
	}
}
