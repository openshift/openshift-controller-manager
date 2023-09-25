package controller

import (
	"fmt"

	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/controller/serviceaccount"
)

func RunServiceAccountController(ctx *ControllerContext) (bool, error) {
	managedNames := ctx.OpenshiftControllerConfig.ServiceAccount.ManagedNames
	if len(managedNames) == 0 {
		klog.Info(openshiftcontrolplanev1.OpenShiftServiceAccountController + ": no managed names specified")
		return false, nil
	}
	return runServiceAccountsController(ctx, managedNames...)
}

func RunBuilderServiceAccountController(ctx *ControllerContext) (bool, error) {
	return runServiceAccountsController(ctx, "builder")
}

func RunDeployerServiceAccountController(ctx *ControllerContext) (bool, error) {
	return runServiceAccountsController(ctx, "deployer")
}

func runServiceAccountsController(cctx *ControllerContext, managedNames ...string) (bool, error) {
	options := serviceaccount.DefaultServiceAccountsControllerOptions()
	options.ServiceAccounts = nil
	for _, name := range managedNames {
		if name == "default" {
			// kube-controller-manager already does this one
			continue
		}
		options.ServiceAccounts = append(options.ServiceAccounts,
			corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name}},
		)
	}
	if len(options.ServiceAccounts) == 0 {
		return false, fmt.Errorf("no managed names specified")
	}
	controller, err := serviceaccount.NewServiceAccountsController(
		cctx.KubernetesInformers.Core().V1().ServiceAccounts(),
		cctx.KubernetesInformers.Core().V1().Namespaces(),
		cctx.ClientBuilder.ClientOrDie(infraServiceAccountControllerServiceAccountName),
		options,
	)
	if err != nil {
		return false, err
	}
	go controller.Run(cctx.Context, 3)
	return true, nil
}
