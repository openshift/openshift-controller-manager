package controller

import (
	"github.com/openshift/openshift-controller-manager/pkg/serviceaccounts/controllers"
	"github.com/openshift/openshift-controller-manager/pkg/serviceaccounts/controllers/rollback"
)

func RunServiceAccountPullSecretsController(ctx *ControllerContext) (bool, error) {
	// Bug 1785023: Increase the rate limit for the SA Pull Secrets controller.
	// The pull secrets controller needs to create new dockercfg secrets at the same rate as the
	// upstream token secret controller.
	kc := ctx.HighRateLimitClientBuilder.ClientOrDie(iInfraServiceAccountPullSecretsControllerServiceAccountName)

	go controllers.NewDockercfgDeletedController(
		ctx.KubernetesInformers.Core().V1().Secrets(),
		kc,
		controllers.DockercfgDeletedControllerOptions{},
	).Run(ctx.Stop)

	go controllers.NewDockercfgTokenDeletedController(
		ctx.KubernetesInformers.Core().V1().Secrets(),
		kc,
		controllers.DockercfgTokenDeletedControllerOptions{},
	).Run(ctx.Stop)

	dockerURLsInitialized := make(chan struct{})
	dockercfgController := controllers.NewDockercfgController(
		ctx.KubernetesInformers.Core().V1().ServiceAccounts(),
		ctx.KubernetesInformers.Core().V1().Secrets(),
		kc,
		controllers.DockercfgControllerOptions{DockerURLsInitialized: dockerURLsInitialized},
	)
	go dockercfgController.Run(5, ctx.Stop)

	dockerRegistryControllerOptions := controllers.DockerRegistryServiceControllerOptions{
		DockercfgController:    dockercfgController,
		DockerURLsInitialized:  dockerURLsInitialized,
		ClusterDNSSuffix:       "cluster.local",
		AdditionalRegistryURLs: ctx.OpenshiftControllerConfig.DockerPullSecret.RegistryURLs,
	}
	go controllers.NewDockerRegistryServiceController(
		ctx.KubernetesInformers.Core().V1().Secrets(),
		ctx.KubernetesInformers.Core().V1().Services(),
		kc,
		dockerRegistryControllerOptions,
	).Run(10, ctx.Stop)

	go rollback.NewServiceAccountRollbackController(
		kc,
		ctx.KubernetesInformers.Core().V1().ServiceAccounts(),
		ctx.KubernetesInformers.Core().V1().Secrets(),
	).Run(ctx.Context, 1)

	return true, nil
}
