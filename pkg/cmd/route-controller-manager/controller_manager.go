package openshift_controller_manager

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/serviceability"

	origincontrollers "github.com/openshift/openshift-controller-manager/pkg/cmd/controller"
	routecontrollers "github.com/openshift/openshift-controller-manager/pkg/cmd/controller/route"
	"github.com/openshift/openshift-controller-manager/pkg/routeversion"
)

func RunRouteControllerManager(config *openshiftcontrolplanev1.OpenShiftControllerManagerConfig, clientConfig *rest.Config) error {
	serviceability.InitLogrusFromKlog()
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	// only serve if we have serving information.
	if config.ServingInfo != nil {
		klog.Infof("Starting controllers on %s (%s)", config.ServingInfo.BindAddress, routeversion.Get().String())

		if err := routecontrollers.RunControllerServer(*config.ServingInfo, kubeClient); err != nil {
			return err
		}
	}
	_, err = origincontrollers.RunRouteControllerManager(config, kubeClient, clientConfig)
	if err != nil {
		return err
	}
	return nil
}
