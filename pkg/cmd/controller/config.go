package controller

import (
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
)

var ControllerInitializers = map[openshiftcontrolplanev1.OpenShiftControllerName]InitFunc{
	openshiftcontrolplanev1.OpenShiftServiceAccountController: RunServiceAccountController,

	openshiftcontrolplanev1.OpenShiftDefaultRoleBindingsController: RunDefaultRoleBindingController,

	openshiftcontrolplanev1.OpenShiftServiceAccountPullSecretsController: RunInternalImageRegistryPullSecretsController,
	openshiftcontrolplanev1.OpenShiftOriginNamespaceController:           RunOriginNamespaceController,

	openshiftcontrolplanev1.OpenShiftBuilderServiceAccountController: RunBuilderServiceAccountController,
	openshiftcontrolplanev1.OpenShiftBuildController:                 RunBuildController,
	openshiftcontrolplanev1.OpenShiftBuildConfigChangeController:     RunBuildConfigChangeController,

	openshiftcontrolplanev1.OpenShiftDeployerServiceAccountController: RunDeployerServiceAccountController,
	openshiftcontrolplanev1.OpenShiftDeployerController:               RunDeployerController,
	openshiftcontrolplanev1.OpenShiftDeploymentConfigController:       RunDeploymentConfigController,

	openshiftcontrolplanev1.OpenShiftImageTriggerController:         RunImageTriggerController,
	openshiftcontrolplanev1.OpenShiftImageImportController:          RunImageImportController,
	openshiftcontrolplanev1.OpenShiftImageSignatureImportController: RunImageSignatureImportController,

	openshiftcontrolplanev1.OpenShiftTemplateInstanceController:          RunTemplateInstanceController,
	openshiftcontrolplanev1.OpenShiftTemplateInstanceFinalizerController: RunTemplateInstanceFinalizerController,

	openshiftcontrolplanev1.OpenShiftUnidlingController: RunUnidlingController,
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
