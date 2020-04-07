package controller

import (
	"github.com/openshift/openshift-controller-manager/pkg/resources"
)

// RunResourceMetrics starts and registers the metrics for resource usage on a cluster.
func RunResourceMetrics(ctx *ControllerContext) (bool, error) {
	return true, resources.RegisterMetrics(ctx.KubernetesInformers.Core().V1().Pods().Lister())
}
