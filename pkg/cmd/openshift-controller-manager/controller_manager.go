package openshift_controller_manager

import (
	"context"
	"fmt"
	"net/http"
	"time"

	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	origincontrollers "github.com/openshift/openshift-controller-manager/pkg/cmd/controller"
	"github.com/openshift/openshift-controller-manager/pkg/cmd/imageformat"
)

func RunOpenShiftControllerManager(config *openshiftcontrolplanev1.OpenShiftControllerManagerConfig, clientConfig *rest.Config, ctx context.Context) error {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
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
	controllerContext, err := origincontrollers.NewControllerContext(ctx, *config, clientConfig)
	if err != nil {
		return err
	}
	if err := startControllers(controllerContext); err != nil {
		return err
	}
	controllerContext.StartInformers(ctx.Done())
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
		return fmt.Errorf("server unhealthy: %v: %v", healthzContent, err)
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
			klog.Fatalf("Error starting %q (%v)", controllerName, err)
			return err
		}
		if !started {
			klog.Warningf("Skipping %q", controllerName)
			continue
		}
		klog.Infof("Started %q", controllerName)
	}
	klog.Infof("Started Origin Controllers")

	return nil
}
