package ingress

import (
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/blang/semver/v4"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	routeController               = "openshift_ingress_to_route_controller"
	metricRouteWithUnmanagedOwner = routeController + "_route_with_unmanaged_owner"
	metricIngressWithoutClassName = routeController + "_ingress_without_class_name"
)

var (
	unmanagedRoutes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricRouteWithUnmanagedOwner,
		Help: "Report the number of routes owned by ingresses no longer managed.",
	}, []string{"name", "host"})

	ingressesWithoutClassName = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricIngressWithoutClassName,
		Help: "Report the number of ingresses that do not specify ingressClassName.",
	}, []string{"name"})
)

func (c *Controller) Create(v *semver.Version) bool {
	c.metricsCreateOnce.Do(func() {
		c.metricsCreateLock.Lock()
		defer c.metricsCreateLock.Unlock()
		c.metricsCreated = true
	})
	return c.MetricsCreated()
}

func (c *Controller) MetricsCreated() bool {
	return c.metricsCreated
}

func (c *Controller) ClearState() {
	c.metricsCreateLock.Lock()
	defer c.metricsCreateLock.Unlock()
	c.metricsCreated = false
}

// FQName returns the fully-qualified metric name of the collector.
func (c *Controller) FQName() string {
	return routeController
}

func (c *Controller) Describe(ch chan<- *prometheus.Desc) {
	unmanagedRoutes.Describe(ch)
	ingressesWithoutClassName.Describe(ch)
}

func (c *Controller) Collect(ch chan<- prometheus.Metric) {
	// collect ingresses that do not specify ingressClassName
	ingressInstances, err := c.ingressLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	for _, ingressInstance := range ingressInstances {
		labelVal := 0
		if ingressInstance.Spec.IngressClassName == nil {
			labelVal = 1
		}
		ingressesWithoutClassName.WithLabelValues(ingressInstance.Name).Set(float64(labelVal))
	}

	ingressesWithoutClassName.Collect(ch)

	// collect routes owned by ingresses no longer managed
	routeInstances, err := c.routeLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	for _, routeInstance := range routeInstances {
		labelVal := 0
		if owner, have := hasIngressOwnerRef(routeInstance.OwnerReferences); have {
			for _, ingressInstance := range ingressInstances {
				if ingressInstance.Name == owner {
					managed, err := c.ingressManaged(ingressInstance)
					if err != nil {
						utilruntime.HandleError(err)
						return
					}
					if !managed {
						labelVal = 1
					}
				}
			}
		}
		unmanagedRoutes.WithLabelValues(routeInstance.Name, routeInstance.Spec.Host).Set(float64(labelVal))
	}

	unmanagedRoutes.Collect(ch)
}
