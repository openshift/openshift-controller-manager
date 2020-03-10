package prometheus

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"

	appsv1 "github.com/openshift/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kcorelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/component-base/metrics/legacyregistry"
)

var (
	timeNow          = metav1.Now()
	defaultTimeNowFn = func() time.Time { return timeNow.Time }
)

func mockRC(name string, version int, annotations map[string]string, generation int64, creationTime metav1.Time) *corev1.ReplicationController {
	r := &corev1.ReplicationController{}
	annotations[appsv1.DeploymentConfigAnnotation] = name
	r.SetName(name + fmt.Sprintf("-%d", version))
	r.SetNamespace("test")
	r.SetCreationTimestamp(creationTime)
	r.SetAnnotations(annotations)
	return r
}

type fakeLister []*corev1.ReplicationController

func (f fakeLister) List(labels.Selector) ([]*corev1.ReplicationController, error) {
	return f, nil
}

func (fakeLister) ReplicationControllers(string) kcorelisters.ReplicationControllerNamespaceLister {
	return nil
}

func (fakeLister) GetPodControllers(*corev1.Pod) ([]*corev1.ReplicationController, error) {
	return nil, nil
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

func TestCollect(t *testing.T) {
	tests := []struct {
		name  string
		count int
		rcs   []*corev1.ReplicationController
		// expected values
		available     float64
		failed        float64
		cancelled     float64
		timestamp     float64
		latestVersion string
	}{
		{
			name:      "no deployments",
			count:     3,
			available: 0,
			failed:    0,
			cancelled: 0,
			rcs:       []*corev1.ReplicationController{},
		},
		{
			name:      "single successful deployment",
			count:     3,
			available: 1,
			failed:    0,
			cancelled: 0,
			rcs: []*corev1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsv1.DeploymentStatusAnnotation: string(appsv1.DeploymentStatusComplete),
				}, 0, timeNow),
			},
		},
		{
			name:          "single cancelled deployment",
			count:         3,
			available:     0,
			failed:        0,
			cancelled:     1,
			latestVersion: "1",
			timestamp:     float64(timeNow.Unix()),
			rcs: []*corev1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsv1.DeploymentCancelledAnnotation: "true",
					appsv1.DeploymentStatusAnnotation:    string(appsv1.DeploymentStatusFailed),
					appsv1.DeploymentVersionAnnotation:   "1",
				}, 0, timeNow),
			},
		},
		{
			name:          "single failed deployment",
			count:         4,
			available:     0,
			failed:        1,
			cancelled:     0,
			latestVersion: "1",
			timestamp:     float64(timeNow.Unix()),
			rcs: []*corev1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusFailed),
					appsv1.DeploymentVersionAnnotation: "1",
				}, 0, timeNow),
			},
		},
		{
			name:          "multiple failed deployment",
			count:         4,
			available:     0,
			failed:        4,
			cancelled:     0,
			latestVersion: "4",
			timestamp:     float64(timeNow.Unix()),
			rcs: []*corev1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusFailed),
					appsv1.DeploymentVersionAnnotation: "1",
				}, 0, timeNow),
				mockRC("foo", 2, map[string]string{
					appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusFailed),
					appsv1.DeploymentVersionAnnotation: "2",
				}, 0, timeNow),
				mockRC("foo", 3, map[string]string{
					appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusFailed),
					appsv1.DeploymentVersionAnnotation: "3",
				}, 0, timeNow),
				mockRC("foo", 4, map[string]string{
					appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusFailed),
					appsv1.DeploymentVersionAnnotation: "4",
				}, 0, timeNow),
			},
		},
		{
			name:          "single failed deployment within successful deployments",
			count:         3,
			available:     2,
			failed:        1,
			cancelled:     0,
			latestVersion: "2",
			timestamp:     float64(timeNow.Unix()),
			rcs: []*corev1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusComplete),
					appsv1.DeploymentVersionAnnotation: "1",
				}, 0, timeNow),
				mockRC("foo", 2, map[string]string{
					appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusFailed),
					appsv1.DeploymentVersionAnnotation: "2",
				}, 0, timeNow),
				mockRC("foo", 3, map[string]string{
					appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusComplete),
					appsv1.DeploymentVersionAnnotation: "3",
				}, 0, timeNow),
			},
		},
		{
			name:          "single active deployment",
			count:         4,
			available:     0,
			failed:        0,
			cancelled:     0,
			latestVersion: "1",
			// the timestamp is duration in this case, which is 0 as the creation time
			// and current time are the same.
			timestamp: 0,
			rcs: []*corev1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusRunning),
					appsv1.DeploymentVersionAnnotation: "1",
				}, 0, timeNow),
			},
		},
		{
			name:          "single active deployment with history",
			count:         4,
			available:     2,
			failed:        0,
			cancelled:     0,
			latestVersion: "3",
			// the timestamp is duration in this case, which is 0 as the creation time
			// and current time are the same.
			timestamp: 0,
			rcs: []*corev1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusComplete),
					appsv1.DeploymentVersionAnnotation: "1",
				}, 0, timeNow),
				mockRC("foo", 2, map[string]string{
					appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusComplete),
					appsv1.DeploymentVersionAnnotation: "2",
				}, 0, timeNow),
				mockRC("foo", 3, map[string]string{
					appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusRunning),
					appsv1.DeploymentVersionAnnotation: "3",
				}, 0, timeNow),
			},
		},
	}

	for _, c := range tests {
		rcCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		fakeCollector := appsCollector{
			lister: kcorelisters.NewReplicationControllerLister(rcCache),
			nowFn:  defaultTimeNowFn,
		}

		for _, rc := range c.rcs {
			if err := rcCache.Add(rc); err != nil {
				t.Fatalf("unable to add rc %s: %v", rc.Name, err)
			}
		}

		collectedMetrics := []prometheus.Metric{}
		collectionChan := make(chan prometheus.Metric)
		stopChan := make(chan struct{})
		go func() {
			defer close(collectionChan)
			fakeCollector.Collect(collectionChan)
			<-stopChan
		}()

		for {
			select {
			case m := <-collectionChan:
				collectedMetrics = append(collectedMetrics, m)
			case <-time.After(time.Second * 5):
				t.Fatalf("[%s] timeout receiving expected results (got %d, want %d)", c.name, len(collectedMetrics), c.count)
			}
			if len(collectedMetrics) == c.count {
				close(stopChan)
				break
			}
		}

		if len(collectedMetrics) == 0 {
			continue
		}

		for _, m := range collectedMetrics {
			var out dto.Metric
			m.Write(&out)

			// last_failed_rollout_time
			if strings.Contains(m.Desc().String(), nameToQuery(lastFailedRolloutTime)) {
				gaugeValue := out.GetGauge().GetValue()
				labels := out.GetLabel()
				if gaugeValue != c.timestamp {
					t.Errorf("[%s][last_failed_rollout_time] creation timestamp %f does not match expected timestamp: %f", c.name, gaugeValue, c.timestamp)
				}
				for _, l := range labels {
					if l.GetName() == "latest_version" && l.GetValue() != c.latestVersion {
						t.Errorf("[%s][last_failed_rollout_time] latest_version %q does not match expected version %q", c.name, l.GetValue(), c.latestVersion)
					}
				}
				continue
			}

			// active_rollouts_duration_seconds
			if strings.Contains(m.Desc().String(), nameToQuery(activeRolloutDurationSeconds)) {
				gaugeValue := out.GetGauge().GetValue()
				labels := out.GetLabel()
				if gaugeValue != c.timestamp {
					t.Errorf("[%s][active_rollouts_duration_seconds] creation timestamp %f does not match expected timestamp: %f", c.name, gaugeValue, c.timestamp)
				}
				for _, l := range labels {
					if l.GetName() == "latest_version" && l.GetValue() != c.latestVersion {
						t.Errorf("[%s][active_rollouts_duration_seconds] latest_version %q does not match expected version %q", c.name, l.GetValue(), c.latestVersion)
					}
				}
				continue
			}

			// complete_rollouts_total
			if strings.Contains(m.Desc().String(), nameToQuery(completeRolloutCount)) {
				gaugeValue := out.GetGauge().GetValue()
				switch out.GetLabel()[0].GetValue() {
				case availablePhase:
					if c.available != gaugeValue {
						t.Errorf("[%s][complete_rollouts_total] expected available %f, got %f", c.name, c.available, gaugeValue)
					}
				case failedPhase:
					if c.failed != gaugeValue {
						t.Errorf("[%s][complete_rollouts_total] expected failed %f, got %f", c.name, c.failed, gaugeValue)
					}
				case cancelledPhase:
					if c.cancelled != gaugeValue {
						t.Errorf("[%s][]complete_rollouts_total expected cancelled %f, got %f", c.name, c.cancelled, gaugeValue)
					}
				}
				continue
			}

			t.Errorf("[%s] unexpected metric recorded: %s", c.name, m.Desc().String())
		}
	}
}

func TestLegacyRegistry(t *testing.T) {
	expectedResponse := []string{
		"# HELP openshift_apps_deploymentconfigs_active_rollouts_duration_seconds Tracks the active rollout duration in seconds",
		"# TYPE openshift_apps_deploymentconfigs_active_rollouts_duration_seconds counter",
		"openshift_apps_deploymentconfigs_active_rollouts_duration_seconds{latest_version=\"1\",name=\"active\",namespace=\"test\",phase=\"running\"} 0",
		"# HELP openshift_apps_deploymentconfigs_complete_rollouts_total Counts total complete rollouts",
		"# TYPE openshift_apps_deploymentconfigs_complete_rollouts_total gauge",
		"openshift_apps_deploymentconfigs_complete_rollouts_total{phase=\"available\"} 2",
		"openshift_apps_deploymentconfigs_complete_rollouts_total{phase=\"cancelled\"} 1",
		"openshift_apps_deploymentconfigs_complete_rollouts_total{phase=\"failed\"} 2",
		"# HELP openshift_apps_deploymentconfigs_last_failed_rollout_time Tracks the time of last failure rollout per deployment config",
		"# TYPE openshift_apps_deploymentconfigs_last_failed_rollout_time gauge",
		"openshift_apps_deploymentconfigs_last_failed_rollout_time{latest_version=\"1\",name=\"failed\",namespace=\"test\"}",
	}
	lister := &fakeLister{
		mockRC("foo", 1, map[string]string{
			appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusComplete),
			appsv1.DeploymentVersionAnnotation: "1",
		}, 0, timeNow),
		mockRC("foo", 2, map[string]string{
			appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusFailed),
			appsv1.DeploymentVersionAnnotation: "2",
		}, 0, timeNow),
		mockRC("foo", 3, map[string]string{
			appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusComplete),
			appsv1.DeploymentVersionAnnotation: "3",
		}, 0, timeNow),
		mockRC("active", 1, map[string]string{
			appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusRunning),
			appsv1.DeploymentVersionAnnotation: "1",
		}, 0, timeNow),
		mockRC("failed", 1, map[string]string{
			appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusFailed),
			appsv1.DeploymentVersionAnnotation: "1",
		}, 0, timeNow),
		mockRC("cancel", 1, map[string]string{
			appsv1.DeploymentCancelledAnnotation: "true",
			appsv1.DeploymentStatusAnnotation:    string(appsv1.DeploymentStatusFailed),
			appsv1.DeploymentVersionAnnotation:   "1",
		}, 0, timeNow),
	}

	ac := appsCollector{
		lister: lister,
		nowFn:  defaultTimeNowFn,
	}

	legacyregistry.MustRegister(&ac)

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
