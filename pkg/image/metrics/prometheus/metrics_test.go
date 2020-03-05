package prometheus

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/component-base/metrics/legacyregistry"
)

func importFetcher() (ImportSuccessCounts, ImportErrorCounts, error) {
	success := map[string]uint64{
		"registry.redhat.io": 5,
		"quay.io":            15,
	}
	errors := map[ImportErrorInfo]uint64{
		{
			Registry: "registry.redhat.io",
			Reason:   "Unauthorized",
		}: 5,
	}
	return success, errors, nil
}

func scheduledImportFetcher() (ImportSuccessCounts, ImportErrorCounts, error) {
	success := map[string]uint64{
		"registry.redhat.io": 12,
		"quay.io":            6,
	}
	errors := map[ImportErrorInfo]uint64{
		{
			Registry: "quay.io",
			Reason:   "Missing",
		}: 5,
	}
	return success, errors, nil
}

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
		"# HELP openshift_imagestreamcontroller_success_count Counts successful image stream imports - both scheduled and not scheduled - per image registry",
		"# TYPE openshift_imagestreamcontroller_success_count counter",
		"openshift_imagestreamcontroller_success_count{registry=\"quay.io\",scheduled=\"false\"} 15",
		"openshift_imagestreamcontroller_success_count{registry=\"quay.io\",scheduled=\"true\"} 6",
		"openshift_imagestreamcontroller_success_count{registry=\"registry.redhat.io\",scheduled=\"false\"} 5",
		"openshift_imagestreamcontroller_success_count{registry=\"registry.redhat.io\",scheduled=\"true\"} 12",
		"# HELP openshift_imagestreamcontroller_error_count Counts number of failed image stream imports - both scheduled and not scheduled - per image registry and failure reason",
		"# TYPE openshift_imagestreamcontroller_error_count counter",
		"openshift_imagestreamcontroller_error_count{reason=\"Missing\",registry=\"quay.io\",scheduled=\"true\"} 5",
		"openshift_imagestreamcontroller_error_count{reason=\"Unauthorized\",registry=\"registry.redhat.io\",scheduled=\"false\"} 5",
	}

	is := importStatusCollector{
		cbCollectISCounts:        importFetcher,
		cbCollectScheduledCounts: scheduledImportFetcher,
	}

	legacyregistry.MustRegister(&is)

	h := promhttp.HandlerFor(legacyregistry.DefaultGatherer, promhttp.HandlerOpts{ErrorHandling: promhttp.PanicOnError})
	rw := &fakeResponseWriter{header: http.Header{}}
	h.ServeHTTP(rw, &http.Request{})

	respStr := rw.String()

	t.Logf("response: %s", respStr)
	for _, s := range expectedResponse {
		if !strings.Contains(respStr, s) {
			t.Errorf("expected string %s did not appear in %s", s, respStr)
		}
	}
}
