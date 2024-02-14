package controller

import (
	"github.com/openshift/openshift-controller-manager/pkg/authorization/defaultrolebindings"
	"k8s.io/client-go/kubernetes"
)

func RunDefaultRoleBindingController(ctx *ControllerContext) (bool, error) {
	kubeClient, err := ctx.ClientBuilder.Client(infraDefaultRoleBindingsControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	return runRoleBindingController(ctx, kubeClient, "DefaultRoleBindingController")
}

func RunBuilderRoleBindingController(ctx *ControllerContext) (bool, error) {
	// Role binding controllers currently share the same service account,
	// as these are created by "bootstrap" logic located elsewhere in Openshift.
	// TODO: Refactor the controller service accounts to be managed by openshift-controller-manager-operator.
	kubeClient, err := ctx.ClientBuilder.Client(infraDefaultRoleBindingsControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	return runRoleBindingController(ctx, kubeClient, "BuilderRoleBindingController")
}

func RunDeployerRoleBindingController(ctx *ControllerContext) (bool, error) {
	// Role binding controllers currently share the same service account,
	// as these are created by "bootstrap" logic located elsewhere in Openshift.
	// TODO: Refactor the controller service accounts to be managed by openshift-controller-manager-operator.
	kubeClient, err := ctx.ClientBuilder.Client(infraDefaultRoleBindingsControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	return runRoleBindingController(ctx, kubeClient, "DeployerRoleBindingController")
}

func RunImagePullerRoleBindingController(ctx *ControllerContext) (bool, error) {
	// Role binding controllers currently share the same service account,
	// as these are created by "bootstrap" logic located elsewhere in Openshift.
	// TODO: Refactor the controller service accounts to be managed by openshift-controller-manager-operator.
	kubeClient, err := ctx.ClientBuilder.Client(infraDefaultRoleBindingsControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	return runRoleBindingController(ctx, kubeClient, "ImagePullerRoleBindingController")
}

func runRoleBindingController(cctx *ControllerContext, kubeClient kubernetes.Interface, controllerName string) (bool, error) {
	go defaultrolebindings.NewRoleBindingsController(
		cctx.KubernetesInformers.Rbac().V1().RoleBindings(),
		cctx.KubernetesInformers.Core().V1().Namespaces(),
		kubeClient.RbacV1(),
		controllerName,
	).Run(5, cctx.Stop)

	return true, nil
}
