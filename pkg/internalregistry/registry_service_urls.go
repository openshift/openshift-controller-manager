package internalregistry

import (
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	listers "k8s.io/client-go/listers/core/v1"
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

func servicesFilter(obj interface{}) bool {
	service, ok := obj.(*corev1.Service)
	if !ok {
		panic("expected Service")
	}
	for _, l := range serviceLocations {
		if l.name == service.Name && l.namespace == service.Namespace {
			return true
		}
	}
	return false
}

// urlsForInternalRegistry returns the dns form and the ip form of the service
func (c *imagePullSecretsController) urlsForInternalRegistry() []string {
	ret := append([]string{}, c.additionalRegistryURLs...)
	for _, location := range serviceLocations {
		ret = append(ret, urlsForInternalRegistryService(c.serviceLister, location)...)
	}
	return ret
}

func urlsForInternalRegistryService(lister listers.ServiceLister, location serviceLocation) []string {
	service, err := lister.Services(location.namespace).Get(location.name)
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
