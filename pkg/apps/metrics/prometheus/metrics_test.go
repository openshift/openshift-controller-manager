package prometheus

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	appsv1 "github.com/openshift/api/apps/v1"
	appslisters "github.com/openshift/client-go/apps/listers/apps/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	kcorelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/component-base/metrics"
)

var (
	timeNow          = metav1.NewTime(time.Unix(42, 42))
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

func mockDCWithStrategy(t appsv1.DeploymentStrategyType) *appsv1.DeploymentConfig {
	return &appsv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			// we need unique keys for the indexer
			Namespace: rand.SafeEncodeString(rand.String(10)),
			Name:      rand.SafeEncodeString(rand.String(10)),
		},
		Spec: appsv1.DeploymentConfigSpec{
			Strategy: appsv1.DeploymentStrategy{
				Type: t,
			},
		},
	}
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

func completedAvailableMetric(v float64) prometheus.Metric {
	return prometheus.MustNewConstMetric(
		completeRolloutCountDesc,
		prometheus.GaugeValue,
		v,
		[]string{
			"available",
		}...,
	)
}

func completedFailedMetric(v float64) prometheus.Metric {
	return prometheus.MustNewConstMetric(
		completeRolloutCountDesc,
		prometheus.GaugeValue,
		v,
		[]string{
			"failed",
		}...,
	)
}

func completedCancelledMetric(v float64) prometheus.Metric {
	return prometheus.MustNewConstMetric(
		completeRolloutCountDesc,
		prometheus.GaugeValue,
		v,
		[]string{
			"cancelled",
		}...,
	)
}

func lastFailedMetric(timestamp float64, namespace, name, latestVersion string) prometheus.Metric {
	return prometheus.MustNewConstMetric(
		lastFailedRolloutTimeDesc,
		prometheus.GaugeValue,
		timestamp,
		[]string{
			namespace,
			name,
			latestVersion,
		}...,
	)
}

func activeRolloutsDurationMetric(durationSec float64, namespace, name, phase, latestVersion string) prometheus.Metric {
	return prometheus.MustNewConstMetric(
		activeRolloutDurationSecondsDesc,
		prometheus.CounterValue,
		durationSec,
		[]string{
			namespace,
			name,
			phase,
			latestVersion,
		}...,
	)
}

func strategyMetric(count float64, t string) prometheus.Metric {
	return prometheus.MustNewConstMetric(
		strategyCountDesc,
		prometheus.GaugeValue,
		count,
		[]string{
			t,
		}...,
	)
}

func TestCollect(t *testing.T) {
	tt := []struct {
		name            string
		rcs             []*corev1.ReplicationController
		dcs             []*appsv1.DeploymentConfig
		expectedMetrics []prometheus.Metric
	}{
		{
			name: "no deployments",
			rcs:  []*corev1.ReplicationController{},
			expectedMetrics: []prometheus.Metric{
				strategyMetric(0, "custom"),
				strategyMetric(0, "recreate"),
				strategyMetric(0, "rolling"),
				completedAvailableMetric(0),
				completedFailedMetric(0),
				completedCancelledMetric(0),
			},
		},
		{
			name: "single successful deployment",
			rcs: []*corev1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsv1.DeploymentStatusAnnotation: string(appsv1.DeploymentStatusComplete),
				}, 0, timeNow),
			},
			expectedMetrics: []prometheus.Metric{
				strategyMetric(0, "custom"),
				strategyMetric(0, "recreate"),
				strategyMetric(0, "rolling"),
				completedAvailableMetric(1),
				completedFailedMetric(0),
				completedCancelledMetric(0),
			},
		},
		{
			name: "single cancelled deployment",
			rcs: []*corev1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsv1.DeploymentCancelledAnnotation: "true",
					appsv1.DeploymentStatusAnnotation:    string(appsv1.DeploymentStatusFailed),
					appsv1.DeploymentVersionAnnotation:   "1",
				}, 0, timeNow),
			},
			expectedMetrics: []prometheus.Metric{
				strategyMetric(0, "custom"),
				strategyMetric(0, "recreate"),
				strategyMetric(0, "rolling"),
				completedAvailableMetric(0),
				completedFailedMetric(0),
				completedCancelledMetric(1),
			},
		},
		{
			name: "single failed deployment",
			rcs: []*corev1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusFailed),
					appsv1.DeploymentVersionAnnotation: "1",
				}, 0, timeNow),
			},
			expectedMetrics: []prometheus.Metric{
				strategyMetric(0, "custom"),
				strategyMetric(0, "recreate"),
				strategyMetric(0, "rolling"),
				lastFailedMetric(float64(timeNow.Unix()), "test", "foo", "1"),
				completedAvailableMetric(0),
				completedFailedMetric(1),
				completedCancelledMetric(0),
			},
		},
		{
			name: "multiple failed deployment",
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
			expectedMetrics: []prometheus.Metric{
				strategyMetric(0, "custom"),
				strategyMetric(0, "recreate"),
				strategyMetric(0, "rolling"),
				lastFailedMetric(float64(timeNow.Unix()), "test", "foo", "4"),
				completedAvailableMetric(0),
				completedFailedMetric(4),
				completedCancelledMetric(0),
			},
		},
		{
			name: "single failed deployment within successful deployments",
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
			expectedMetrics: []prometheus.Metric{
				strategyMetric(0, "custom"),
				strategyMetric(0, "recreate"),
				strategyMetric(0, "rolling"),
				completedAvailableMetric(2),
				completedFailedMetric(1),
				completedCancelledMetric(0),
			},
		},
		{
			name: "single active deployment",
			rcs: []*corev1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsv1.DeploymentStatusAnnotation:  string(appsv1.DeploymentStatusRunning),
					appsv1.DeploymentVersionAnnotation: "1",
				}, 0, timeNow),
			},
			expectedMetrics: []prometheus.Metric{
				strategyMetric(0, "custom"),
				strategyMetric(0, "recreate"),
				strategyMetric(0, "rolling"),
				activeRolloutsDurationMetric(0, "test", "foo", "running", "1"),
				completedAvailableMetric(0),
				completedFailedMetric(0),
				completedCancelledMetric(0),
			},
		},
		{
			name: "single active deployment with history",
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
			expectedMetrics: []prometheus.Metric{
				strategyMetric(0, "custom"),
				strategyMetric(0, "recreate"),
				strategyMetric(0, "rolling"),
				activeRolloutsDurationMetric(0, "test", "foo", "running", "3"),
				completedAvailableMetric(2),
				completedFailedMetric(0),
				completedCancelledMetric(0),
			},
		},
		{
			name: "dc strategy count",
			dcs: []*appsv1.DeploymentConfig{
				mockDCWithStrategy(appsv1.DeploymentStrategyTypeCustom),
				mockDCWithStrategy(appsv1.DeploymentStrategyTypeRecreate),
				mockDCWithStrategy(appsv1.DeploymentStrategyTypeRecreate),
				mockDCWithStrategy(appsv1.DeploymentStrategyTypeRolling),
				mockDCWithStrategy(appsv1.DeploymentStrategyTypeRolling),
				mockDCWithStrategy(appsv1.DeploymentStrategyTypeRolling),
			},
			expectedMetrics: []prometheus.Metric{
				strategyMetric(1, "custom"),
				strategyMetric(2, "recreate"),
				strategyMetric(3, "rolling"),
				completedAvailableMetric(0),
				completedFailedMetric(0),
				completedCancelledMetric(0),
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			dcCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			rcCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			collector := appsCollector{
				dcLister: appslisters.NewDeploymentConfigLister(dcCache),
				rcLister: kcorelisters.NewReplicationControllerLister(rcCache),
				nowFn:    defaultTimeNowFn,
			}

			for _, dc := range tc.dcs {
				err := dcCache.Add(dc)
				if err != nil {
					t.Fatalf("unable to add dc %s: %v", dc.Name, err)
				}
			}

			for _, rc := range tc.rcs {
				err := rcCache.Add(rc)
				if err != nil {
					t.Fatalf("unable to add rc %s: %v", rc.Name, err)
				}
			}

			var collectedMetrics []prometheus.Metric
			collectionChan := make(chan prometheus.Metric, 1000) // big enough buffer to contain all created metrics
			collector.Collect(collectionChan)
			close(collectionChan)
			for m := range collectionChan {
				collectedMetrics = append(collectedMetrics, m)
			}

			d := cmp.Diff(tc.expectedMetrics, collectedMetrics, cmp.Comparer(func(lhs prometheus.Metric, rhs prometheus.Metric) bool {
				var lhsOut dto.Metric
				err := lhs.Write(&lhsOut)
				if err != nil {
					t.Fatal(nil)
				}
				var rhsOut dto.Metric
				err = rhs.Write(&rhsOut)
				if err != nil {
					t.Fatal(nil)
				}

				return reflect.DeepEqual(lhs.Desc(), rhs.Desc()) && reflect.DeepEqual(lhsOut, rhsOut)
			}))
			if len(d) > 0 {
				t.Errorf("expected and collected metrics differ: %s", d)
			}
		})
	}
}

func TestRegistry(t *testing.T) {
	expectedResponse := `# HELP openshift_apps_deploymentconfigs_active_rollouts_duration_seconds Tracks the active rollout duration in seconds
# TYPE openshift_apps_deploymentconfigs_active_rollouts_duration_seconds counter
openshift_apps_deploymentconfigs_active_rollouts_duration_seconds{latest_version="1",name="active",namespace="test",phase="running"} 0
# HELP openshift_apps_deploymentconfigs_complete_rollouts_total Counts total complete rollouts
# TYPE openshift_apps_deploymentconfigs_complete_rollouts_total gauge
openshift_apps_deploymentconfigs_complete_rollouts_total{phase="available"} 2
openshift_apps_deploymentconfigs_complete_rollouts_total{phase="cancelled"} 1
openshift_apps_deploymentconfigs_complete_rollouts_total{phase="failed"} 2
# HELP openshift_apps_deploymentconfigs_last_failed_rollout_time Tracks the time of last failure rollout per deployment config
# TYPE openshift_apps_deploymentconfigs_last_failed_rollout_time gauge
openshift_apps_deploymentconfigs_last_failed_rollout_time{latest_version="1",name="failed",namespace="test"} 42
# HELP openshift_apps_deploymentconfigs_strategy_total Counts strategy usage
# TYPE openshift_apps_deploymentconfigs_strategy_total gauge
openshift_apps_deploymentconfigs_strategy_total{type="custom"} 0
openshift_apps_deploymentconfigs_strategy_total{type="recreate"} 0
openshift_apps_deploymentconfigs_strategy_total{type="rolling"} 0
`

	dcCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	rcCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	collector := appsCollector{
		dcLister: appslisters.NewDeploymentConfigLister(dcCache),
		rcLister: kcorelisters.NewReplicationControllerLister(rcCache),
		nowFn:    defaultTimeNowFn,
	}

	for _, dc := range []*appsv1.DeploymentConfig{} {
		err := dcCache.Add(dc)
		if err != nil {
			t.Fatalf("unable to add dc %s: %v", dc.Name, err)
		}
	}

	for _, rc := range []*corev1.ReplicationController{
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
	} {
		err := rcCache.Add(rc)
		if err != nil {
			t.Fatalf("unable to add rc %s: %v", rc.Name, err)
		}
	}

	registry := metrics.NewKubeRegistry()
	err := registry.Register(&collector)
	if err != nil {
		t.Fatal(err)
	}

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorHandling: promhttp.PanicOnError})
	rw := &fakeResponseWriter{header: http.Header{}}
	h.ServeHTTP(rw, &http.Request{})

	respStr := rw.String()

	var builder strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(respStr))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "openshift_apps_deploymentconfigs_") {
			_, err := builder.WriteString(line)
			if err != nil {
				t.Fatal(err)
			}
			_, err = builder.WriteRune('\n')
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	d := cmp.Diff(expectedResponse, builder.String())
	if len(d) > 0 {
		t.Errorf("expected and received metrics differ: %s", d)
	}
}
