package main

import (
	"os"
	"runtime"

	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/cli"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/library-go/pkg/serviceability"

	"github.com/openshift/api/apps"
	"github.com/openshift/api/authorization"
	"github.com/openshift/api/build"
	"github.com/openshift/api/image"
	"github.com/openshift/api/oauth"
	"github.com/openshift/api/project"
	"github.com/openshift/api/template"
	"github.com/openshift/api/user"

	openshift_controller_manager "github.com/openshift/openshift-controller-manager/pkg/cmd/openshift-controller-manager"
	route_controller_manager "github.com/openshift/openshift-controller-manager/pkg/cmd/route-controller-manager"
	"github.com/openshift/openshift-controller-manager/pkg/version"
)

func init() {
	// TODO: these references to the legacy scheme must go
	//  They are only here because we have controllers referencing it, and inside hypershift this worked fine as openshift-apiserver was installing the API into the legacy scheme.
	utilruntime.Must(apps.Install(legacyscheme.Scheme))
	utilruntime.Must(authorization.Install(legacyscheme.Scheme))
	utilruntime.Must(build.Install(legacyscheme.Scheme))
	utilruntime.Must(image.Install(legacyscheme.Scheme))
	utilruntime.Must(oauth.Install(legacyscheme.Scheme))
	utilruntime.Must(project.Install(legacyscheme.Scheme))
	utilruntime.Must(template.Install(legacyscheme.Scheme))
	utilruntime.Must(user.Install(legacyscheme.Scheme))
}

func main() {
	stopCh := genericapiserver.SetupSignalHandler()

	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"), version.Get())()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	command := NewOpenShiftControllerManagerCommand(stopCh)
	code := cli.Run(command)
	os.Exit(code)
}

func NewOpenShiftControllerManagerCommand(stopCh <-chan struct{}) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "openshift-controller-manager",
		Short: "Command for the OpenShift Controllers",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}
	start := openshift_controller_manager.NewOpenShiftControllerManagerCommand("start", os.Stdout, os.Stderr, stopCh)
	cmd.AddCommand(start)

	routeStart := route_controller_manager.NewRouteControllerManagerCommand("route-controller-manager-start", os.Stdout, os.Stderr, stopCh)
	cmd.AddCommand(routeStart)

	return cmd
}
