package controllers

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	informers "k8s.io/client-go/informers"
	externalfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/controller"
)

const dockerCfgSecretName = "default-dockercfg-fplln"

func TestDockercfgDeletion(t *testing.T) {
	testcases := map[string]struct {
		ClientObjects []runtime.Object

		DeletedSecret *v1.Secret

		ExpectedActions []clientgotesting.Action
	}{
		"deleted dockercfg secret without serviceaccount": {
			DeletedSecret: createdDockercfgSecret(),

			ExpectedActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(schema.GroupVersionResource{Resource: "serviceaccounts", Version: "v1"}, "default", "default"),
			},
		},
		"deleted dockercfg secret with serviceaccount with reference": {
			ClientObjects: []runtime.Object{serviceAccount(addTokenSecretReference(tokenSecretReferences()), imagePullSecretReferences()), createdDockercfgSecret()},

			DeletedSecret: createdDockercfgSecret(),
			ExpectedActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(schema.GroupVersionResource{Resource: "serviceaccounts", Version: "v1"}, "default", "default"),
				clientgotesting.NewUpdateAction(schema.GroupVersionResource{Resource: "serviceaccounts", Version: "v1"}, "default", serviceAccount(tokenSecretReferences(), []v1.LocalObjectReference{})),
			},
		},
		"deleted dockercfg secret with serviceaccount without reference": {
			ClientObjects: []runtime.Object{serviceAccount(addTokenSecretReference(tokenSecretReferences()), imagePullSecretReferences()), createdDockercfgSecret()},

			DeletedSecret: createdDockercfgSecret(),
			ExpectedActions: []clientgotesting.Action{
				clientgotesting.NewGetAction(schema.GroupVersionResource{Resource: "serviceaccounts", Version: "v1"}, "default", "default"),
				clientgotesting.NewUpdateAction(schema.GroupVersionResource{Resource: "serviceaccounts", Version: "v1"}, "default", serviceAccount(tokenSecretReferences(), []v1.LocalObjectReference{})),
			},
		},
	}

	for k, tc := range testcases {
		client := externalfake.NewSimpleClientset(tc.ClientObjects...)
		informerFactory := informers.NewSharedInformerFactory(client, controller.NoResyncPeriodFunc())
		controller := NewDockercfgDeletedController(
			informerFactory.Core().V1().Secrets(),
			client,
			DockercfgDeletedControllerOptions{},
		)
		stopCh := make(chan struct{})
		informerFactory.Start(stopCh)
		if !cache.WaitForCacheSync(stopCh, controller.secretController.HasSynced) {
			t.Fatalf("unable to reach cache sync")
		}
		client.ClearActions()

		if tc.DeletedSecret != nil {
			controller.secretDeleted(tc.DeletedSecret)
		}

		for i, action := range client.Actions() {
			if len(tc.ExpectedActions) < i+1 {
				t.Errorf("%s: %d unexpected actions: %+v", k, len(client.Actions())-len(tc.ExpectedActions), client.Actions()[i:])
				break
			}

			expectedAction := tc.ExpectedActions[i]
			if !reflect.DeepEqual(expectedAction, action) {
				t.Errorf("%s: Expected %v, got %v", k, expectedAction, action)
				continue
			}
		}

		if len(tc.ExpectedActions) > len(client.Actions()) {
			t.Errorf("%s: %d additional expected actions:%+v", k, len(tc.ExpectedActions)-len(client.Actions()), tc.ExpectedActions[len(client.Actions()):])
		}
		close(stopCh)
	}
}

// createdDockercfgSecret returns the ServiceAccountToken secret posted when creating a new token secret.
// Named "default-token-fplln", since that is the first generated name after rand.Seed(1)
func createdDockercfgSecret() *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dockerCfgSecretName,
			Namespace: "default",
			Annotations: map[string]string{
				v1.ServiceAccountNameKey:           "default",
				v1.ServiceAccountUIDKey:            "12345",
				ServiceAccountTokenValueAnnotation: "verysecrettoken",
			},
		},
		Type: v1.SecretTypeDockercfg,
		Data: map[string][]byte{
			v1.DockerConfigKey: []byte(`{"docker-registry.default.svc.cluster.local":{"Username":"serviceaccount","Password":"verysecrettoken","Email":"serviceaccount@example.org"}}`),
		},
	}
}

// serviceAccount returns a service account with the given secret refs
func serviceAccount(secretRefs []v1.ObjectReference, imagePullSecretRefs []v1.LocalObjectReference) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "default",
			UID:             "12345",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Secrets:          secretRefs,
		ImagePullSecrets: imagePullSecretRefs,
	}
}

// addTokenSecretReference adds a reference to the ServiceAccountToken that will be created
func addTokenSecretReference(refs []v1.ObjectReference) []v1.ObjectReference {
	return append(refs, v1.ObjectReference{Name: dockerCfgSecretName})
}

func imagePullSecretReferences() []v1.LocalObjectReference {
	return []v1.LocalObjectReference{{Name: dockerCfgSecretName}}
}

// tokenSecretReferences is used by a service account that references a ServiceAccountToken secret
func tokenSecretReferences() []v1.ObjectReference {
	return []v1.ObjectReference{
		{
			Name: "token-secret-1",
			UID:  "23456",
		},
	}
}
