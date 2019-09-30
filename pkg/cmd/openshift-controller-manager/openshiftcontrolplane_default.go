package openshift_controller_manager

import (
	"time"

	"k8s.io/klog"

	configv1 "github.com/openshift/api/config/v1"
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/config/configdefaults"
	"github.com/openshift/library-go/pkg/config/helpers"
	leaderelectionconverter "github.com/openshift/library-go/pkg/config/leaderelection"
)

func setRecommendedOpenShiftControllerConfigDefaults(config *openshiftcontrolplanev1.OpenShiftControllerManagerConfig) {
	if config.ServingInfo == nil {
		// apply recommended HTTPS serving info if none were provided
		config.ServingInfo = &configv1.HTTPServingInfo{}
		configdefaults.SetRecommendedHTTPServingInfoDefaults(config.ServingInfo)
	} else if config.ServingInfo != nil && len(config.ServingInfo.BindAddress) == 0 {
		// disable serving if already set with empty bind address, used in dev setup
		klog.Warning("config.ServingInfo will be ignored as it contains an empty BindAddress")
		config.ServingInfo = nil
	}
	configdefaults.SetRecommendedKubeClientConfigDefaults(&config.KubeClientConfig)
	config.LeaderElection = leaderelectionconverter.LeaderElectionDefaulting(config.LeaderElection, "kube-system", "openshift-master-controllers")

	configdefaults.DefaultStringSlice(&config.Controllers, []string{"*"})

	configdefaults.DefaultString(&config.Network.ServiceNetworkCIDR, "10.0.0.0/24")

	if config.ImageImport.MaxScheduledImageImportsPerMinute == 0 {
		config.ImageImport.MaxScheduledImageImportsPerMinute = 60
	}
	if config.ImageImport.ScheduledImageImportMinimumIntervalSeconds == 0 {
		config.ImageImport.ScheduledImageImportMinimumIntervalSeconds = 15 * 60
	}

	configdefaults.DefaultString(&config.SecurityAllocator.UIDAllocatorRange, "1000000000-1999999999/10000")
	configdefaults.DefaultString(&config.SecurityAllocator.MCSAllocatorRange, "s0:/2")
	if config.SecurityAllocator.MCSLabelsPerProject == 0 {
		config.SecurityAllocator.MCSLabelsPerProject = 5
	}

	if config.ResourceQuota.MinResyncPeriod.Duration == 0 {
		config.ResourceQuota.MinResyncPeriod.Duration = 5 * time.Minute
	}
	if config.ResourceQuota.SyncPeriod.Duration == 0 {
		config.ResourceQuota.SyncPeriod.Duration = 12 * time.Hour
	}
	if config.ResourceQuota.ConcurrentSyncs == 0 {
		config.ResourceQuota.ConcurrentSyncs = 5
	}

	if config.ImageImport.MaxScheduledImageImportsPerMinute == 0 {
		config.ImageImport.MaxScheduledImageImportsPerMinute = 60
	}
	if config.ImageImport.ScheduledImageImportMinimumIntervalSeconds == 0 {
		config.ImageImport.ScheduledImageImportMinimumIntervalSeconds = 15 * 60 // 15 minutes
	}

	configdefaults.DefaultStringSlice(&config.ServiceAccount.ManagedNames, []string{"builder", "deployer"})

	// TODO this default is WRONG, but it appears to work
	configdefaults.DefaultString(&config.Deployer.ImageTemplateFormat.Format, "quay.io/openshift/origin-${component}:${version}")

	// TODO this default is WRONG, but it appears to work
	configdefaults.DefaultString(&config.Build.ImageTemplateFormat.Format, "quay.io/openshift/origin-${component}:${version}")
}

func getOpenShiftControllerConfigFileReferences(config *openshiftcontrolplanev1.OpenShiftControllerManagerConfig) []*string {
	if config == nil {
		return []*string{}
	}

	refs := []*string{}

	if config.ServingInfo != nil {
		refs = append(refs, helpers.GetHTTPServingInfoFileReferences(config.ServingInfo)...)
	}
	refs = append(refs, helpers.GetKubeClientConfigFileReferences(&config.KubeClientConfig)...)

	return refs
}
