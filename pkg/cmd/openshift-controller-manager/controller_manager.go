package openshift_controller_manager

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	origincontrollers "github.com/openshift/openshift-controller-manager/pkg/cmd/controller"
	"github.com/openshift/openshift-controller-manager/pkg/cmd/imageformat"
)

func RunOpenShiftControllerManager(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
	config, err := asOpenshiftControllerManagerConfig(controllerContext.ComponentConfig)
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	{
		imageTemplate := imageformat.NewDefaultImageTemplate()
		imageTemplate.Format = config.Deployer.ImageTemplateFormat.Format
		imageTemplate.Latest = config.Deployer.ImageTemplateFormat.Latest
		klog.Infof("DeploymentConfig controller using images from %q", imageTemplate.ExpandOrDie("<component>"))
	}
	{
		imageTemplate := imageformat.NewDefaultImageTemplate()
		imageTemplate.Format = config.Build.ImageTemplateFormat.Format
		imageTemplate.Latest = config.Build.ImageTemplateFormat.Latest
		klog.Infof("Build controller using images from %q", imageTemplate.ExpandOrDie("<component>"))
	}

	if err := WaitForHealthyAPIServer(kubeClient.Discovery().RESTClient()); err != nil {
		return err
	}

	ocmControllerContext, err := origincontrollers.NewControllerContext(ctx, *config, controllerContext.KubeConfig)
	if err != nil {
		return err
	}
	if err := startControllers(ocmControllerContext); err != nil {
		return err
	}
	ocmControllerContext.StartInformers(ctx.Done())

	<-ctx.Done()
	return nil
}

func WaitForHealthyAPIServer(client rest.Interface) error {
	var healthzContent string
	// If apiserver is not running we should wait for some time and fail only then. This is particularly
	// important when we start apiserver and controller manager at the same time.
	err := wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		healthStatus := 0
		resp := client.Get().AbsPath("/healthz").Do(context.TODO()).StatusCode(&healthStatus)
		if healthStatus != http.StatusOK {
			klog.Errorf("Server isn't healthy yet. Waiting a little while.")
			return false, nil
		}

		content, _ := resp.Raw()
		healthzContent = string(content)
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("server unhealthy: %v: %w", healthzContent, err)
	}
	return nil
}

// startControllers launches the controllers
// allocation controller is passed in because it wants direct etcd access.  Naughty.
func startControllers(controllerContext *origincontrollers.ControllerContext) error {
	for controllerName, initFn := range origincontrollers.ControllerInitializers {
		if !controllerContext.IsControllerEnabled(string(controllerName)) {
			klog.Warningf("%q is disabled", controllerName)
			continue
		}

		klog.V(1).Infof("Starting %q", controllerName)
		started, err := initFn(controllerContext)
		if err != nil {
			return fmt.Errorf("failed to start controller %s: %w", controllerName, err)
		}
		if !started {
			klog.Warningf("Skipping %q", controllerName)
			continue
		}
		klog.Infof("Started %q", controllerName)
	}
	if err := startRollbackControllers(controllerContext); err != nil {
		return err
	}
	klog.Infof("Started Origin Controllers")
	return nil
}

func startRollbackControllers(ctx *origincontrollers.ControllerContext) error {
	for name, start := range origincontrollers.RollbackControllers {
		if ctx.IsControllerEnabled(string(name)) {
			// rollback controller should never run if corresponding origin controller is enabled
			continue
		}
		name = name + "-rollback"
		if slices.Contains(ctx.OpenshiftControllerConfig.Controllers, string("-"+name)) {
			// rollback controller was explicitly disabled in the config
			klog.Warningf("%q is disabled", name)
			continue
		}
		klog.V(1).Infof("Starting %q", name)
		ok, err := start(ctx)
		if err != nil {
			klog.Fatalf("Error starting %q (%v)", name, err)
			return err
		}
		if !ok {
			klog.Warningf("Skipping %q", name)
			continue
		}
		klog.Infof("Started %q", name)
	}
	return nil
}
