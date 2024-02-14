package defaultrolebindings

import (
	"github.com/openshift/api/annotations"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
)

const (
	openShiftDescription = annotations.OpenShiftDescription

	ImagePullerRoleName  = "system:image-puller"
	ImageBuilderRoleName = "system:image-builder"
	DeployerRoleName     = "system:deployer"

	DeployerRoleBindingName     = DeployerRoleName + "s"
	ImagePullerRoleBindingName  = ImagePullerRoleName + "s"
	ImageBuilderRoleBindingName = ImageBuilderRoleName + "s"

	BuilderServiceAccountName  = "builder"
	DeployerServiceAccountName = "deployer"
)

type projectRoleBindings func(namespace string) []rbacv1.RoleBinding
type serviceAccountRoleBinding func(namespace string) rbacv1.RoleBinding

// GetImagePullerProjectRoleBindings generates a role binding that allows all pods to pull ImageStream images associated with given namespace.
// These should only be created if the "ImageRegistry" capability is enabled on the cluster.
func GetImagePullerProjectRoleBindings(namespace string) rbacv1.RoleBinding {
	imagePuller := newOriginRoleBindingForClusterRoleWithGroup(ImagePullerRoleBindingName, ImagePullerRoleName, namespace, serviceaccount.MakeNamespaceGroupName(namespace))
	imagePuller.Annotations[openShiftDescription] = "Allows all pods in this namespace to pull images from this namespace.  It is auto-managed by a controller; remove subjects to disable."

	return imagePuller
}

// GetBuilderServiceAccountProjectRoleBindings generates the role bindings specific to the "builder" service account of given namespace.
// These should only be created if the "Build" capability is enabled on the cluster.
func GetBuilderServiceAccountProjectRoleBindings(namespace string) rbacv1.RoleBinding {
	imageBuilder := newOriginRoleBindingForClusterRoleWithSA(ImageBuilderRoleBindingName, ImageBuilderRoleName, namespace, BuilderServiceAccountName)
	imageBuilder.Annotations[openShiftDescription] = "Allows builds in this namespace to push images to this namespace.  It is auto-managed by a controller; remove subjects to disable."

	return imageBuilder
}

// GetDeployerServiceAccountProjectRoleBindings generates the role bindings specific to the "builder" service account of given namespace.
// These should only be created if the "DeploymentConfig" capability is enabled on the cluster.
func GetDeployerServiceAccountProjectRoleBindings(namespace string) rbacv1.RoleBinding {
	deployer := newOriginRoleBindingForClusterRoleWithSA(DeployerRoleBindingName, DeployerRoleName, namespace, DeployerServiceAccountName)
	deployer.Annotations[openShiftDescription] = "Allows deploymentconfigs in this namespace to rollout pods in this namespace.  It is auto-managed by a controller; remove subjects to disable."

	return deployer
}

func composeRoleBindings(roleBindings ...serviceAccountRoleBinding) projectRoleBindings {
	return func(namespace string) []rbacv1.RoleBinding {
		bindings := []rbacv1.RoleBinding{}
		for _, rbfunc := range roleBindings {
			bindings = append(bindings, rbfunc(namespace))
		}
		return bindings
	}
}

// GetRoleBindingsForController returns the appropriate generator function for the
// given named controller that will reconcile role bindings in a namespace.
func GetRoleBindingsForController(controller string) projectRoleBindings {
	switch controller {
	case "BuilderRoleBindingController":
		return composeRoleBindings(GetBuilderServiceAccountProjectRoleBindings)
	case "DeployerRoleBindingController":
		return composeRoleBindings(GetDeployerServiceAccountProjectRoleBindings)
	case "ImagePullerRoleBindingController":
		return composeRoleBindings(GetImagePullerProjectRoleBindings)
	default:
		return composeRoleBindings(GetImagePullerProjectRoleBindings,
			GetBuilderServiceAccountProjectRoleBindings,
			GetDeployerServiceAccountProjectRoleBindings,
		)
	}
}

func GetBootstrapServiceAccountProjectRoleBindingNames(roleBindings projectRoleBindings) sets.Set[string] {
	names := sets.Set[string]{}

	// Gets the roleBinding Names in the "default" namespace
	for _, roleBinding := range roleBindings("default") {
		names.Insert(roleBinding.Name)
	}

	return names
}

func newOriginRoleBindingForClusterRoleWithGroup(bindingName, roleName, namespace, group string) rbacv1.RoleBinding {
	return rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        bindingName,
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.GroupKind, APIGroup: "rbac.authorization.k8s.io", Name: group},
		},
	}
}

func newOriginRoleBindingForClusterRoleWithSA(bindingName, roleName, namespace, saName string) rbacv1.RoleBinding {
	return rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        bindingName,
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.ServiceAccountKind, Namespace: namespace, Name: saName},
		},
	}
}
