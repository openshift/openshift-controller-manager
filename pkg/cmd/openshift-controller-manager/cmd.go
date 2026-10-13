package openshift_controller_manager

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/utils/clock"

	corev1 "k8s.io/api/core/v1"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/openshift-controller-manager/pkg/version"
)

const (
	podNameEnv      = "POD_NAME"
	podNamespaceEnv = "POD_NAMESPACE"
)

func NewOpenShiftControllerManagerCommand(name string) *cobra.Command {
	// We need to set custom owner reference, because the default implementation uses in-cluster config only.
	// That doesn't work in Hypershift where the current namespace must be passed in via an environment variable.
	cmd := controllercmd.NewControllerCommandConfig(
		"openshift-controller-manager", version.Get(), RunOpenShiftControllerManager, clock.RealClock{},
	).WithComponentOwnerReference(&corev1.ObjectReference{
		Kind:      "Pod",
		Name:      os.Getenv(podNameEnv),
		Namespace: getNamespace(),
	}).NewCommandWithContext(context.Background())

	cmd.Use = name
	cmd.Short = "Start the OpenShift controllers"
	cmd.MarkFlagRequired("config")
	return cmd
}

// getNamespace returns the in-cluster namespace, falling back to an environment variable or constant value.
func getNamespace() string {
	if nsBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		return string(nsBytes)
	}
	if podNamespace := os.Getenv(podNamespaceEnv); len(podNamespace) > 0 {
		return podNamespace
	}
	return "openshift-controller-manager"
}
