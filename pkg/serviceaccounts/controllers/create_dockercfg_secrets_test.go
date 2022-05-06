package controllers

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	authenticationv1 "k8s.io/api/authentication/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/watch"
	fake "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func TestDockercfgController_secretExpirationsChecker(t *testing.T) {
	tests := []struct {
		name             string
		secrets          []*v1.Secret
		expectedQueueLen int
	}{
		{
			name:             "no secrets",
			expectedQueueLen: 0,
		},
		{
			name: "non-expiring secret",
			secrets: []*v1.Secret{
				secretWithExpiration(v1.SecretTypeDockercfg, 15*time.Minute),
			},
			expectedQueueLen: 0,
		},
		{
			name: "expiring secret",
			secrets: []*v1.Secret{
				secretWithExpiration(v1.SecretTypeDockercfg, 5*time.Minute),
			},
			expectedQueueLen: 1,
		},
		{
			name: "non-dockercfg secret",
			secrets: []*v1.Secret{
				secretWithExpiration(v1.SecretTypeServiceAccountToken, 5*time.Minute),
			},
			expectedQueueLen: 0,
		},
		{
			name: "mix of expiring and non-expiring secrets",
			secrets: []*v1.Secret{
				secretWithExpiration(v1.SecretTypeDockercfg, 14*time.Minute),
				secretWithExpiration(v1.SecretTypeDockercfg, 3*time.Minute),
				secretWithExpiration(v1.SecretTypeDockercfg, 4*time.Minute),
				secretWithExpiration(v1.SecretTypeDockercfg, 50*time.Minute),
				secretWithExpiration(v1.SecretTypeDockercfg, 1*time.Minute),
				secretWithExpiration(v1.SecretTypeDockercfg, 0),
				secretWithExpiration(v1.SecretTypeBasicAuth, 5*time.Minute),
				secretWithExpiration(v1.SecretTypeDockercfg, 37*time.Minute),
			},
			expectedQueueLen: 4,
		},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			secretQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

			secretCache := cache.NewStore(cache.MetaNamespaceKeyFunc)
			for _, s := range tt.secrets {
				require.NoError(t, secretCache.Add(s))
			}

			e := &DockercfgController{
				secretCache: secretCache,
				secretQueue: secretQueue,
			}
			e.secretExpirationsChecker()

			require.Equal(t, tt.expectedQueueLen, secretQueue.Len())
		})
	}
}

func secretWithExpiration(secretType v1.SecretType, expirationOffset time.Duration) *v1.Secret {
	expiration := time.Now().Add(expirationOffset)
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      expiration.String(),
			Namespace: "testns",
			Annotations: map[string]string{
				v1.ServiceAccountNameKey:         "testsa",
				DockercfgExpirationAnnotationKey: strconv.FormatInt(expiration.Unix(), 10),
			},
		},
		Type: secretType,
		Data: make(map[string][]byte),
	}
}

func TestDockercfgController_syncSecret(t *testing.T) {
	tests := []struct {
		name         string
		secret       *v1.Secret
		wantModified bool
		wantErr      bool
	}{
		{
			name:         "expiring secret",
			wantModified: true,
			secret:       secretWithExpiration(v1.DockerConfigKey, 5*time.Minute),
		},
		{
			name:         "non-expiring secret",
			wantModified: false,
			secret:       secretWithExpiration(v1.DockerConfigKey, 15*time.Minute),
		},
		{
			name:         "non-existent secret",
			wantModified: false,
			wantErr:      false, // it's just ignored
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset(&v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testsa",
					Namespace: "testns",
				},
			})
			kubeClient.PrependReactor(
				"create", "serviceaccounts",
				func(action ktesting.Action) (bool, runtime.Object, error) {
					if action.GetSubresource() == "token" {
						return true, &authenticationv1.TokenRequest{
							Status: authenticationv1.TokenRequestStatus{
								Token:               "topsecretdontshow",
								ExpirationTimestamp: metav1.NewTime(time.Now().Add(1 * time.Hour)),
							},
						}, nil
					}
					return false, nil, nil
				},
			)

			secretCache := cache.NewStore(cache.MetaNamespaceKeyFunc)
			if tt.secret != nil {
				require.NoError(t, secretCache.Add(tt.secret))

				// this controller only works with existing secrets
				_, err := kubeClient.CoreV1().Secrets("testns").Create(context.Background(),
					tt.secret,
					metav1.CreateOptions{})
				require.NoError(t, err)
			}

			e := &DockercfgController{
				client:        kubeClient,
				dockerURLLock: sync.Mutex{},
				dockerURLs:    []string{"fairly.random.url.url"},
				secretCache:   secretCache,
			}

			var err error
			key := "somesecret"
			if tt.secret != nil {
				key, err = cache.MetaNamespaceKeyFunc(tt.secret)
				require.NoError(t, err)
			}

			secretWatcher, err := kubeClient.CoreV1().Secrets("testns").Watch(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			if secretWatcher != nil {
				defer secretWatcher.Stop()
			}
			var secretModified *v1.Secret
			finished := make(chan bool)

			// higher wait number here helps when debugging the sync in the other goroutine
			timedCtx, timedCtxCancel := context.WithTimeout(context.Background(), 1500*time.Second)
			go func() {
				secretChan := secretWatcher.ResultChan()
				for {
					select {
					case secretEvent := <-secretChan:
						secret, ok := secretEvent.Object.(*v1.Secret)
						require.True(t, ok)
						if secret.Name == tt.secret.Name && secretEvent.Type == watch.Modified {
							secretModified = secret
						}
						// check secretEvent.Type is watch.Modified
					case <-timedCtx.Done():
						finished <- true
						return
					}
				}
			}()

			go func() {
				if err := e.syncSecret(key); (err != nil) != tt.wantErr {
					t.Errorf("DockercfgController.syncSecret() error = %v, wantErr %v", err, tt.wantErr)
				}

				time.Sleep(1 * time.Second)
				timedCtxCancel()
			}()

			<-finished
			require.Equal(t, tt.wantModified, (secretModified != nil), "expected secret update to be %v, but it was %v", tt.wantModified, secretModified)

			if secretModified != nil {
				expectedSecret, err := createSADockerCfg(e.dockerURLs, "topsecretdontshow")
				require.NoError(t, err)
				require.Equal(t,
					expectedSecret, secretModified.Data[v1.DockerConfigKey],
					diff.StringDiff(string(expectedSecret), string(secretModified.Data[v1.DockerConfigKey])),
				)
			}
		})
	}
}

func TestDockercfgController_syncServiceAccount(t *testing.T) {
	type fields struct {
		saQueue     workqueue.RateLimitingInterface
		secretQueue workqueue.RateLimitingInterface
	}

	const saName = "testsa"
	const testNS = "testns"

	dockercfgSecretName := getDockercfgSecretNamePrefix(saName) + "bogus"
	tests := []struct {
		name            string
		fields          fields
		sa              *v1.ServiceAccount
		secret          *v1.Secret
		expectNewSecret bool
		expectSAUpdate  bool
		wantErr         bool
		wantNewSecret   bool
	}{
		{
			name: "fresh SA",
			sa: &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      saName,
					Namespace: testNS,
				},
			},
			expectNewSecret: true,
			expectSAUpdate:  true,
		},
		{
			name: "SA needs updating imagepullsecrets",
			sa: &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      saName,
					Namespace: testNS,
					Annotations: map[string]string{
						PendingTokenAnnotation: dockercfgSecretName,
					},
				},
				Secrets: []v1.ObjectReference{{
					Name: dockercfgSecretName,
				}},
			},
			expectSAUpdate: true,
		},
		{
			name: "SA needs updating secrets",
			sa: &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      saName,
					Namespace: testNS,
					Annotations: map[string]string{
						PendingTokenAnnotation: dockercfgSecretName,
					},
				},
				ImagePullSecrets: []v1.LocalObjectReference{{
					Name: dockercfgSecretName,
				}},
			},
			expectSAUpdate: true,
		},
		{
			name: "SA with pending token but the secret is already created and has a valid token",
			sa: &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      saName,
					Namespace: testNS,
					Annotations: map[string]string{
						PendingTokenAnnotation: dockercfgSecretName,
					},
				},
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dockercfgSecretName,
					Namespace: testNS,
					Annotations: map[string]string{
						ServiceAccountTokenValueAnnotation: "supersecrettoken",
					},
				},
			},
			expectNewSecret: false,
			expectSAUpdate:  true,
		},
		{
			name: "fully populated SA",
			sa: &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      saName,
					Namespace: testNS,
				},
				ImagePullSecrets: []v1.LocalObjectReference{{
					Name: dockercfgSecretName,
				}},
				Secrets: []v1.ObjectReference{{
					Name: dockercfgSecretName,
				}},
			},
			expectNewSecret: false,
			expectSAUpdate:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saCache := cache.NewStore(cache.MetaNamespaceKeyFunc)
			secretCache := cache.NewStore(cache.MetaNamespaceKeyFunc)

			var objs []runtime.Object
			if tt.sa != nil {
				require.NoError(t, saCache.Add(tt.sa))
				objs = append(objs, tt.sa)
			}
			if tt.secret != nil {
				require.NoError(t, secretCache.Add(tt.secret))
				objs = append(objs, tt.secret)
			}

			fakeClient := fake.NewSimpleClientset(objs...)
			fakeClient.PrependReactor(
				"create", "serviceaccounts",
				func(action ktesting.Action) (bool, runtime.Object, error) {
					if action.GetSubresource() == "token" {
						return true, &authenticationv1.TokenRequest{
							Status: authenticationv1.TokenRequestStatus{
								Token:               "topsecretdontshow",
								ExpirationTimestamp: metav1.NewTime(time.Now().Add(1 * time.Hour)),
							},
						}, nil
					}
					return false, nil, nil
				},
			)

			e := &DockercfgController{
				client:              fakeClient,
				dockerURLLock:       sync.Mutex{},
				dockerURLs:          []string{"fairly.random.url.url"},
				serviceAccountCache: NewEtcdMutationCache(saCache),
				secretCache:         secretCache,
				saQueue:             tt.fields.saQueue,
				secretQueue:         tt.fields.secretQueue,
			}
			var err error
			key := "somesecret"
			if tt.sa != nil {
				key, err = cache.MetaNamespaceKeyFunc(tt.sa)
				require.NoError(t, err)
			}

			saWatcher, err := fakeClient.CoreV1().ServiceAccounts(testNS).Watch(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			if saWatcher != nil {
				defer saWatcher.Stop()
			}
			secretWatcher, err := fakeClient.CoreV1().Secrets(testNS).Watch(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			if secretWatcher != nil {
				defer secretWatcher.Stop()
			}

			var (
				saModified         *v1.ServiceAccount
				secretModified     *v1.Secret
				secretCreated      bool
				saSecretsPopulated bool
			)

			finished := make(chan bool)
			// higher wait number here helps when debugging the sync in the other goroutine
			timedCtx, timedCtxCancel := context.WithTimeout(context.Background(), 1500*time.Second)
			go func() {
				saChan := saWatcher.ResultChan()
				secretChan := secretWatcher.ResultChan()
				for {
					select {
					case saEvent := <-saChan:
						sa, ok := saEvent.Object.(*v1.ServiceAccount)
						require.True(t, ok)
						if sa.Name == tt.sa.Name && saEvent.Type == watch.Modified {
							saModified = sa
						}

						secrets, imagePullSecrets := getGeneratedDockercfgSecretNames(sa)
						if len(secrets) > 0 && len(imagePullSecrets) > 0 {
							saSecretsPopulated = true

						}
					case secretEvent := <-secretChan:
						secret, ok := secretEvent.Object.(*v1.Secret)
						require.True(t, ok)
						if secretEvent.Type == watch.Added {
							secretCreated = true
							continue
						}

						if secretEvent.Type == watch.Modified && secret.Annotations[v1.ServiceAccountNameKey] == tt.sa.Name {
							secretModified = secret
						}

					case <-timedCtx.Done():
						finished <- true
						return
					}
				}
			}()

			go func() {
				if err := e.syncServiceAccount(key); (err != nil) != tt.wantErr {
					t.Errorf("DockercfgController.syncServiceAccount() error = %v, wantErr %v", err, tt.wantErr)
				}

				time.Sleep(1 * time.Second)
				timedCtxCancel()
			}()

			<-finished
			if tt.expectSAUpdate {
				require.Truef(t, saSecretsPopulated, "expected both sa.secrets and sa.imagePullSecrets to be populated: %v", saModified)
			}
			require.Equal(t, tt.expectSAUpdate, (saModified != nil), "expected SA update %v, but it was %v", tt.expectSAUpdate, saModified != nil)
			require.Equal(t, tt.expectNewSecret, secretCreated, "expected new secret %v, but it was %v", tt.expectNewSecret, secretCreated)
			require.Equal(t, tt.expectNewSecret, (secretModified != nil), "expected secret update %v to get the new token, but it was %v", tt.expectNewSecret, secretModified)

		})
	}
}
