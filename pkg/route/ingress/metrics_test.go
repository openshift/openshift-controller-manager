package ingress

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/metrics/legacyregistry"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type fakeResponseWriter struct {
	bytes.Buffer
	statusCode int
	header     http.Header
}

func (f *fakeResponseWriter) Header() http.Header {
	return f.header
}

func (f *fakeResponseWriter) WriteHeader(statusCode int) {
	f.statusCode = statusCode
}

func TestMetrics(t *testing.T) {
	expectedResponse := []string{
		"# HELP openshift_ingress_to_route_controller_ingress_without_class_name Report the number of ingresses that do not specify ingressClassName.",
		"# TYPE openshift_ingress_to_route_controller_ingress_without_class_name gauge",
		"openshift_ingress_to_route_controller_ingress_without_class_name{name=\"without-ingressclassname\"} 1",
		"openshift_ingress_to_route_controller_ingress_without_class_name{name=\"not-managed\"} 0",
		"openshift_ingress_to_route_controller_ingress_without_class_name{name=\"managed\"} 0",
		"# HELP openshift_ingress_to_route_controller_route_with_unmanaged_owner Report the number of routes owned by ingresses no longer managed.",
		"# TYPE openshift_ingress_to_route_controller_route_with_unmanaged_owner gauge",
		"openshift_ingress_to_route_controller_route_with_unmanaged_owner{host=\"test.com\",name=\"owned-by-unmanaged\"} 1",
		"openshift_ingress_to_route_controller_route_with_unmanaged_owner{host=\"test.com\",name=\"owned-by-managed\"} 0",
	}

	boolTrue := true
	customIngressClassName := "custom"
	openshiftDefaultIngressClassName := "openshift-default"

	r := &routeLister{
		Items: []*routev1.Route{
			{ // Route owned by an Ingress that is not managed.
				ObjectMeta: metav1.ObjectMeta{
					Name:            "owned-by-unmanaged",
					Namespace:       "test",
					OwnerReferences: []metav1.OwnerReference{{APIVersion: "networking.k8s.io/v1", Kind: "Ingress", Name: "not-managed", Controller: &boolTrue}},
				},
				Spec: routev1.RouteSpec{
					Host: "test.com",
				},
			},
			{ // Route owned by an Ingress that is managed.
				ObjectMeta: metav1.ObjectMeta{
					Name:            "owned-by-managed",
					Namespace:       "test",
					OwnerReferences: []metav1.OwnerReference{{APIVersion: "networking.k8s.io/v1", Kind: "Ingress", Name: "managed", Controller: &boolTrue}},
				},
				Spec: routev1.RouteSpec{
					Host: "test.com",
				},
			},
		},
	}
	i := &ingressLister{
		Items: []*networkingv1.Ingress{
			{ // Ingress without IngressClassName.
				ObjectMeta: metav1.ObjectMeta{
					Name:      "without-ingressclassname",
					Namespace: "test",
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: nil,
				},
			},
			{ // Ingress with IngressClassName that is not managed.
				ObjectMeta: metav1.ObjectMeta{
					Name:      "not-managed",
					Namespace: "test",
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: &customIngressClassName,
				},
			},
			{ // Ingress with IngressClassName that is managed.
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managed",
					Namespace: "test",
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: &openshiftDefaultIngressClassName,
				},
			},
		},
	}

	ic := &ingressclassLister{
		Items: []*networkingv1.IngressClass{
			{ // IngressClass specifying "openshift.io/ingress-to-route" controller
				ObjectMeta: metav1.ObjectMeta{
					Name: openshiftDefaultIngressClassName,
				},
				Spec: networkingv1.IngressClassSpec{
					Controller: "openshift.io/ingress-to-route",
				},
			},
			{ // IngressClass specifying "acme.io/ingress-controller" controller
				ObjectMeta: metav1.ObjectMeta{
					Name: customIngressClassName,
				},
				Spec: networkingv1.IngressClassSpec{
					Controller: "acme.io/ingress-controller",
				},
			},
		},
	}

	c := &Controller{
		routeLister:        r,
		ingressLister:      i,
		ingressclassLister: ic,
	}

	legacyregistry.MustRegister(c)
	h := promhttp.HandlerFor(legacyregistry.DefaultGatherer, promhttp.HandlerOpts{ErrorHandling: promhttp.PanicOnError})

	rw := &fakeResponseWriter{header: http.Header{}}
	h.ServeHTTP(rw, &http.Request{})

	respStr := rw.String()
	for _, s := range expectedResponse {
		if !strings.Contains(respStr, s) {
			t.Errorf("expected string %s did not appear in %s", s, respStr)
		}
	}
}
