package controller

import (
	"github.com/openshift/library-go/pkg/operator/events"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	sacontroller "k8s.io/kubernetes/pkg/controller/serviceaccount"
)

func RunServiceAccountController(ctx *ControllerContext) (bool, error) {
	if len(ctx.OpenshiftControllerConfig.ServiceAccount.ManagedNames) == 0 {
		klog.Infof("Skipped starting Service Account Manager, no managed names specified")
		return false, nil
	}

	options := sacontroller.DefaultServiceAccountsControllerOptions()
	options.ServiceAccounts = []corev1.ServiceAccount{}

	for _, saName := range ctx.OpenshiftControllerConfig.ServiceAccount.ManagedNames {
		// the upstream controller does this one, so we don't have to
		if saName == "default" {
			continue
		}
		sa := corev1.ServiceAccount{}
		sa.Name = saName

		options.ServiceAccounts = append(options.ServiceAccounts, sa)
	}

	controller, err := sacontroller.NewServiceAccountsController(
		ctx.KubernetesInformers.Core().V1().ServiceAccounts(),
		ctx.KubernetesInformers.Core().V1().Namespaces(),
		ctx.ClientBuilder.ClientOrDie(infraServiceAccountControllerServiceAccountName),
		options)
	if err != nil {
		return true, nil
	}
	go controller.Run(ctx.Context, 3)

	return true, nil
}

func RunServiceAccountPullSecretsController(controllerContext *ControllerContext) (bool, error) {
	// Bug 1785023: Increase the rate limit for the SA Pull Secrets controller.
	// The pull secrets controller needs to create new dockercfg secrets at the same rate as the
	// upstream token secret controller.
	kubeClient := controllerContext.HighRateLimitClientBuilder.ClientOrDie(iInfraServiceAccountPullSecretsControllerServiceAccountName)
	additionalRegistryURLs := controllerContext.OpenshiftControllerConfig.DockerPullSecret.RegistryURLs
	recorder := events.NewRecorder(kubeClient.CoreV1().Events("openshift-controller-manager"), iInfraServiceAccountPullSecretsControllerServiceAccountName, &corev1.ObjectReference{})
	controller := newImagePullSecretControllerEnablerController(kubeClient, controllerContext.KubernetesInformers, controllerContext.ImageRegistryInformers, additionalRegistryURLs, recorder)
	go controller.Run(controllerContext.Context, 1)
	return true, nil
}
