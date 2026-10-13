package defaultrolebindings

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestRoleBindingsForController(t *testing.T) {
	tests := map[string]struct {
		controller                string
		expectedRoleBindingsNames sets.Set[string]
	}{
		"test-builder": {
			controller:                "BuilderRoleBindingController",
			expectedRoleBindingsNames: sets.Set[string]{"system:image-builders": {}},
		},
		"test-deployer": {
			controller:                "DeployerRoleBindingController",
			expectedRoleBindingsNames: sets.Set[string]{"system:deployers": {}},
		},
		"test-image-puller": {
			controller:                "ImagePullerRoleBindingController",
			expectedRoleBindingsNames: sets.Set[string]{"system:image-pullers": {}},
		},
		"test-default": {
			controller: "DefaultRoleBindingController",
			expectedRoleBindingsNames: sets.Set[string]{"system:image-pullers": {},
				"system:image-builders": {},
				"system:deployers":      {},
			},
		},
	}

	for tName, tCase := range tests {
		t.Run(tName, func(t *testing.T) {
			controllerRoleBindings := GetRoleBindingsForController(tCase.controller)
			roleBindingNames := GetBootstrapServiceAccountProjectRoleBindingNames(controllerRoleBindings)

			if !cmp.Equal(roleBindingNames, tCase.expectedRoleBindingsNames) {
				t.Fatalf("expected %v, got %#v", tCase.expectedRoleBindingsNames, roleBindingNames)
			}

		})
	}
}
