package openshift_controller_manager

import (
	"context"

	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift/openshift-controller-manager/pkg/version"
)

var longDescription = templates.LongDesc(`
	Start the OpenShift controllers`)

func NewOpenShiftControllerManagerCommand(ctx context.Context) *cobra.Command {
	config := controllercmd.NewControllerCommandConfig("openshift-controller-manager", version.Get(), RunOperator)
	cmd := config.NewCommandWithContext(ctx)
	cmd.Use = "start"
	cmd.Long = longDescription
	return cmd
}

func RunOperator(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
	config := &openshiftcontrolplanev1.OpenShiftControllerManagerConfig{}
	if controllerContext.ComponentConfig != nil {
		cc := controllerContext.ComponentConfig.DeepCopy()
		cc.SetGroupVersionKind(openshiftcontrolplanev1.GroupVersion.WithKind("OpenShiftControllerManagerConfig"))
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(cc.Object, config)
		if err != nil {
			return err
		}
	}
	setRecommendedOpenShiftControllerConfigDefaults(config)
	return RunOpenShiftControllerManager(config, controllerContext.KubeConfig, ctx)
}
