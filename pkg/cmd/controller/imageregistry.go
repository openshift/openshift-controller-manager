package controller

import "github.com/openshift/openshift-controller-manager/pkg/internalregistry/controllers"

// RunInternalImageRegistryPullSecretsController starts the control loops that manage
// the image pull secrets for the internal image registry.
func RunInternalImageRegistryPullSecretsController(ctx *ControllerContext) (bool, error) {
	kc := ctx.HighRateLimitClientBuilder.ClientOrDie(iInfraServiceAccountPullSecretsControllerServiceAccountName)
	secrets := ctx.KubernetesInformers.Core().V1().Secrets()
	serviceAccounts := ctx.KubernetesInformers.Core().V1().ServiceAccounts()
	services := ctx.KubernetesInformers.Core().V1().Services()
	additionalRegistryURLs := ctx.OpenshiftControllerConfig.DockerPullSecret.RegistryURLs

	serviceAccountController := controllers.NewServiceAccountController(kc, serviceAccounts, secrets)
	imagePullSecretController, kids, urls := controllers.NewImagePullSecretController(kc, secrets)
	keyIDObservationController := controllers.NewKeyIDObservationController(kc, secrets, kids)
	registryURLObservationController := controllers.NewRegistryURLObservationController(services, additionalRegistryURLs, urls)
	legacyTokenSecretController := controllers.NewLegacyTokenSecretController(kc, secrets)
	legacyImagePullSecretController := controllers.NewLegacyImagePullSecretController(kc, secrets)

	go serviceAccountController.Run(ctx.Context, 5)
	go keyIDObservationController.Run(ctx.Context, 1)
	go registryURLObservationController.Run(ctx.Context, 1)
	go imagePullSecretController.Run(ctx.Context, 5)
	go legacyTokenSecretController.Run(ctx.Context, 5)
	go legacyImagePullSecretController.Run(ctx.Context, 5)
	return true, nil
}
