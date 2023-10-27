package controller

import (
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/openshift-controller-manager/pkg/internalregistry"
)

func RunServiceAccountPullSecretsController(ctx *ControllerContext) (bool, error) {
	// Bug 1785023: Increase the rate limit for the SA Pull Secrets controller.
	// The pull secrets controller needs to create new dockercfg secrets at the same rate as the
	// upstream token secret controller.
	kc := ctx.HighRateLimitClientBuilder.ClientOrDie(iInfraServiceAccountPullSecretsControllerServiceAccountName)
	ref, _ := events.GetControllerReferenceForCurrentPod(ctx.Context, kc, "openshift-controller-manager", nil)
	recorder := events.NewKubeRecorder(kc.CoreV1().Events("openshift-controller-manager"), string(openshiftcontrolplanev1.OpenShiftServiceAccountPullSecretsController), ref)

	go internalregistry.NewImagePullSecretsController(
		kc,
		ctx.KubernetesInformers.Core().V1().ServiceAccounts(),
		ctx.KubernetesInformers.Core().V1().Secrets(),
		ctx.KubernetesInformers.Core().V1().Services(),
		ctx.OpenshiftControllerConfig.DockerPullSecret.RegistryURLs,
		recorder,
	).Run(ctx.Context, 1)

	return true, nil
}
