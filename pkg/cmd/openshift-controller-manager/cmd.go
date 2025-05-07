package openshift_controller_manager

import (
	"context"

	"github.com/spf13/cobra"
	"k8s.io/utils/clock"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/openshift-controller-manager/pkg/version"
)

func NewOpenShiftControllerManagerCommand(name string) *cobra.Command {
	cmd := controllercmd.NewControllerCommandConfig(
		"openshift-controller-manager", version.Get(), RunOpenShiftControllerManager, clock.RealClock{},
	).NewCommandWithContext(context.Background())
	cmd.Use = name
	cmd.Short = "Start the OpenShift controllers"
	cmd.MarkFlagRequired("config")
	return cmd
}
