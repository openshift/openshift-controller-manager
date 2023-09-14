package controller

import (
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
)

var ControllerInitializers = map[openshiftcontrolplanev1.OpenShiftControllerName]InitFunc{
	openshiftcontrolplanev1.OpenShiftServiceAccountController: RunServiceAccountController,

	openshiftcontrolplanev1.OpenShiftDefaultRoleBindingsController: RunDefaultRoleBindingController,

	openshiftcontrolplanev1.OpenShiftServiceAccountPullSecretsController: RunServiceAccountPullSecretsController,
	openshiftcontrolplanev1.OpenshiftOriginNamespaceController:           RunOriginNamespaceController,

	openshiftcontrolplanev1.OpenShiftBuilderServiceAccountController: RunBuilderServiceAccountController,
	openshiftcontrolplanev1.OpenshiftBuildController:                 RunBuildController,
	openshiftcontrolplanev1.OpenshiftBuildConfigChangeController:     RunBuildConfigChangeController,

	openshiftcontrolplanev1.OpenShiftDeployerServiceAccountController: RunDeployerServiceAccountController,
	openshiftcontrolplanev1.OpenshiftDeployerController:               RunDeployerController,
	openshiftcontrolplanev1.OpenshiftDeploymentConfigController:       RunDeploymentConfigController,

	openshiftcontrolplanev1.OpenshiftImageTriggerController:         RunImageTriggerController,
	openshiftcontrolplanev1.OpenshiftImageImportController:          RunImageImportController,
	openshiftcontrolplanev1.OpenshiftImageSignatureImportController: RunImageSignatureImportController,

	openshiftcontrolplanev1.OpenshiftTemplateInstanceController:          RunTemplateInstanceController,
	openshiftcontrolplanev1.OpenshiftTemplateInstanceFinalizerController: RunTemplateInstanceFinalizerController,

	openshiftcontrolplanev1.OpenshiftUnidlingController: RunUnidlingController,
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
	infraUnidlingControllerServiceAccountName                   = "unidling-controller"
	infraDefaultRoleBindingsControllerServiceAccountName        = "default-rolebindings-controller"

	// template instance controller watches for TemplateInstance object creation
	// and instantiates templates as a result.
	infraTemplateInstanceControllerServiceAccountName          = "template-instance-controller"
	infraTemplateInstanceFinalizerControllerServiceAccountName = "template-instance-finalizer-controller"

	deployerServiceAccountName = "deployer"

	defaultOpenShiftInfraNamespace = "openshift-infra"
)
