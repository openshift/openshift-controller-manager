package controllers

import (
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const clusterDNSSuffix = "cluster.local"

type serviceLocation struct {
	namespace string
	name      string
}

var serviceLocations = []serviceLocation{
	{namespace: "default", name: "docker-registry"},
	{namespace: "openshift-image-registry", name: "image-registry"},
}

type registryURLObservation struct {
	services               listers.ServiceLister
	additionalRegistryURLs []string
	cacheSyncs             []cache.InformerSynced
	queue                  workqueue.RateLimitingInterface
	ch                     urlsChan
}

func NewRegistryURLObservationController(services informers.ServiceInformer, additionalRegistryURLs []string, ch urlsChan) *registryURLObservation {
	c := &registryURLObservation{
		services:               services.Lister(),
		additionalRegistryURLs: additionalRegistryURLs,
		ch:                     ch,
		cacheSyncs:             []cache.InformerSynced{services.Informer().HasSynced},
		queue:                  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "internal-registry-services"),
	}
	services.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj any) bool {
			service := obj.(*corev1.Service)
			for _, l := range serviceLocations {
				if l.name == service.Name && l.namespace == service.Namespace {
					return true
				}
			}
			return false
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj any) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err == nil {
					c.queue.Add(key)
				}
			},
			UpdateFunc: func(_, obj any) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err == nil {
					c.queue.Add(key)
				}
			},
		},
	})
	return c
}

func (c *registryURLObservation) sync(ctx context.Context, key string) error {
	klog.V(4).InfoS("sync", "key", key)
	// urlsForInternalRegistry returns the dns form and the ip form of the service
	urls := append([]string{}, c.additionalRegistryURLs...)
	for _, location := range serviceLocations {
		urls = append(urls, urlsForInternalRegistryService(c.services, location)...)
	}
	slices.Sort(urls)
	select {
	case c.ch <- urls:
	case <-ctx.Done():
	}
	return nil
}

func urlsForInternalRegistryService(services listers.ServiceLister, location serviceLocation) []string {
	service, err := services.Services(location.namespace).Get(location.name)
	if err != nil {
		return []string{}
	}

	ip := net.ParseIP(service.Spec.ClusterIP)
	if ip == nil {
		return []string{}
	}

	if len(service.Spec.Ports) == 0 {
		return []string{}
	}

	svcPort := service.Spec.Ports[0].Port
	ret := []string{
		net.JoinHostPort(fmt.Sprintf("%s.%s.svc", service.Name, service.Namespace), fmt.Sprintf("%d", svcPort)),
	}

	// Bug 1780376: add ClusterIP as a location if service supports IPv4
	// IPv6 addresses are not valid locations in an image pull spec
	ipv4 := ip.To4()
	if ipv4 != nil {
		ret = append(ret, net.JoinHostPort(ipv4.String(), fmt.Sprintf("%d", svcPort)))
	}
	// Bug 1701422: if using HTTP/S default ports, add locations without the port number
	if svcPort == 80 || svcPort == 443 {
		ret = append(ret, fmt.Sprintf("%s.%s.svc", service.Name, service.Namespace))
		if ipv4 != nil {
			ret = append(ret, ipv4.String())
		}
	}
	ret = append(ret, net.JoinHostPort(fmt.Sprintf("%s.%s.svc."+clusterDNSSuffix, service.Name, service.Namespace), fmt.Sprintf("%d", svcPort)))
	// Bug 1701422: if using HTTP/S default ports, add locations without the port number
	if svcPort == 80 || svcPort == 443 {
		ret = append(ret, fmt.Sprintf("%s.%s.svc."+clusterDNSSuffix, service.Name, service.Namespace))
	}
	return ret
}

func (c *registryURLObservation) Run(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()
	const name = "openshift.io/internal-image-registry-pull-secrets_urls"
	klog.InfoS("Starting controller", "name", name)
	if !cache.WaitForNamedCacheSync(name, ctx.Done(), c.cacheSyncs...) {
		return
	}
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}
	klog.InfoS("Started controller", "name", name)
	<-ctx.Done()
	klog.InfoS("Shutting down controller", "name", name)
}

func (c *registryURLObservation) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *registryURLObservation) processNextWorkItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)
	err := c.sync(ctx, key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}
	runtime.HandleError(fmt.Errorf("%v failed with : %v", key, err))
	c.queue.AddRateLimited(key)
	return true
}
