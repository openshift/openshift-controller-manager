package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	"k8s.io/api/core/v1"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/library-go/pkg/security/uid"
	sccallocation "github.com/openshift/openshift-controller-manager/pkg/security/controller"
	"github.com/openshift/openshift-controller-manager/pkg/security/mcs"
)

func RunNamespaceSecurityAllocationController(ctx *ControllerContext) (bool, error) {
	uidRange, err := uid.ParseRange(ctx.OpenshiftControllerConfig.SecurityAllocator.UIDAllocatorRange)
	if err != nil {
		return true, fmt.Errorf("unable to describe UID range: %v", err)
	}
	mcsRange, err := mcs.ParseRange(ctx.OpenshiftControllerConfig.SecurityAllocator.MCSAllocatorRange)
	if err != nil {
		return true, fmt.Errorf("unable to describe MCS category range: %v", err)
	}

	kubeClient, err := ctx.ClientBuilder.Client(infraNamespaceSecurityAllocationControllerServiceAccountName)
	if err != nil {
		return true, err
	}
	securityClient, err := ctx.ClientBuilder.OpenshiftSecurityClient(infraNamespaceSecurityAllocationControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	controller := sccallocation.NewNamespaceSCCAllocationController(
		ctx.KubernetesInformers.Core().V1().Namespaces(),
		kubeClient.CoreV1().Namespaces(),
		securityClient.SecurityV1(),
		uidRange,
		sccallocation.DefaultMCSAllocation(uidRange, mcsRange, ctx.OpenshiftControllerConfig.SecurityAllocator.MCSLabelsPerProject),
	)
	controllerRun := func(cntx context.Context) {
		controller.Run(cntx.Done())
	}
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "cluster-policy-controller"})
	id, err := os.Hostname()
	if err != nil {
		return false, err
	}
	rl, err := resourcelock.New(
		"configmaps",
		// namespace where cluster-policy-controller container runs in static pod
		"openshift-kube-controller-manager",
		"cluster-policy-controller",
		kubeClient.CoreV1(),
		kubeClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: eventRecorder,
		})
	if err != nil {
		return false, err
	}
	go leaderelection.RunOrDie(context.Background(),
		leaderelection.LeaderElectionConfig{
			Lock:          rl,
			LeaseDuration: 60 * time.Second,
			RenewDeadline: 15 * time.Second,
			RetryPeriod:   5 * time.Second,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: controllerRun,
				OnStoppedLeading: func() {
					klog.Fatalf("leaderelection lost")
				},
			},
		})

	return true, nil
}
