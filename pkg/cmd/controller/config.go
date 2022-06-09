package controller

var ControllerInitializers = map[string]InitFunc{
	"openshift.io/ingress-ip":       RunIngressIPController,
	"openshift.io/ingress-to-route": RunIngressToRouteController,
}

const (
	infraOriginNamespaceServiceAccountName                      = "origin-namespace-controller"
	infraServiceAccountControllerServiceAccountName             = "serviceaccount-controller"
	iInfraServiceAccountPullSecretsControllerServiceAccountName = "serviceaccount-pull-secrets-controller"
	infraBuildControllerServiceAccountName                      = "build-controller"
	infraBuildConfigChangeControllerServiceAccountName          = "build-config-change-controller"
	infraDeploymentConfigControllerServiceAccountName           = "deploymentconfig-controller"
	infraDeployerControllerServiceAccountName                   = "deployer-controller"
	infraImageTriggerControllerServiceAccountName               = "image-trigger-controller"
	infraImageImportControllerServiceAccountName                = "image-import-controller"
	infraSDNControllerServiceAccountName                        = "sdn-controller"
	infraUnidlingControllerServiceAccountName                   = "unidling-controller"
	infraServiceIngressIPControllerServiceAccountName           = "service-ingress-ip-controller"
	infraDefaultRoleBindingsControllerServiceAccountName        = "default-rolebindings-controller"
	infraIngressToRouteControllerServiceAccountName             = "ingress-to-route-controller"

	// template instance controller watches for TemplateInstance object creation
	// and instantiates templates as a result.
	infraTemplateInstanceControllerServiceAccountName          = "template-instance-controller"
	infraTemplateInstanceFinalizerControllerServiceAccountName = "template-instance-finalizer-controller"

	deployerServiceAccountName = "deployer"

	defaultOpenShiftInfraNamespace = "openshift-infra"
)
