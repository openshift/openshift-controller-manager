package controller

var ControllerInitializers = map[string]InitFunc{
	"openshift.io/namespace-security-allocation": RunNamespaceSecurityAllocationController,
	"openshift.io/resourcequota":                 RunResourceQuotaManager,
	"openshift.io/cluster-quota-reconciliation":  RunClusterQuotaReconciliationController,
}

const (
	infraClusterQuotaReconciliationControllerServiceAccountName  = "cluster-quota-reconciliation-controller"
	infraNamespaceSecurityAllocationControllerServiceAccountName = "namespace-security-allocation-controller"
	defaultOpenShiftInfraNamespace                               = "openshift-infra"
)
