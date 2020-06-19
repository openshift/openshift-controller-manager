package metrics

import (
	"k8s.io/client-go/util/workqueue"
	k8smetrics "k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
)

// Per recommendation from the openshift monitoring team, this module has been
// copied from https://github.com/openshift/openshift-controller-manager/blob/master/vendor/k8s.io/component-base/metrics/prometheus/workqueue/metrics.go
// and then adjusted for use within OCM for adding metrics for the named OCM workqueues; as you can tell from the URL,
// the module exists in upstream k8s, but only at versions 1.18 and greater, so
// we are approximating the function / intent for versions of openshift based on earlier
// versions of k8s, where we still want those metrics for named workqueues

// Metrics subsystem and keys used by the workqueue.
const (
	WorkQueueSubsystem         = "workqueue"
	DepthKey                   = "depth"
	AddsKey                    = "adds_total"
	QueueLatencyKey            = "queue_duration_seconds"
	WorkDurationKey            = "work_duration_seconds"
	UnfinishedWorkKey          = "unfinished_work_seconds"
	LongestRunningProcessorKey = "longest_running_processor_seconds"
	RetriesKey                 = "retries_total"
)

var (
	depth = k8smetrics.NewGaugeVec(&k8smetrics.GaugeOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      DepthKey,
		Help:      "Current depth of workqueue",
	}, []string{"name"})

	adds = k8smetrics.NewCounterVec(&k8smetrics.CounterOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      AddsKey,
		Help:      "Total number of adds handled by workqueue",
	}, []string{"name"})

	latency = k8smetrics.NewHistogramVec(&k8smetrics.HistogramOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      QueueLatencyKey,
		Help:      "How long in seconds an item stays in workqueue before being requested.",
		Buckets:   k8smetrics.ExponentialBuckets(10e-9, 10, 10),
	}, []string{"name"})

	workDuration = k8smetrics.NewHistogramVec(&k8smetrics.HistogramOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      WorkDurationKey,
		Help:      "How long in seconds processing an item from workqueue takes.",
		Buckets:   k8smetrics.ExponentialBuckets(10e-9, 10, 10),
	}, []string{"name"})

	unfinished = k8smetrics.NewGaugeVec(&k8smetrics.GaugeOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      UnfinishedWorkKey,
		Help: "How many seconds of work has done that " +
			"is in progress and hasn't been observed by work_duration. Large " +
			"values indicate stuck threads. One can deduce the number of stuck " +
			"threads by observing the rate at which this increases.",
	}, []string{"name"})

	longestRunningProcessor = k8smetrics.NewGaugeVec(&k8smetrics.GaugeOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      LongestRunningProcessorKey,
		Help: "How many seconds has the longest running " +
			"processor for workqueue been running.",
	}, []string{"name"})

	retries = k8smetrics.NewCounterVec(&k8smetrics.CounterOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      RetriesKey,
		Help:      "Total number of retries handled by workqueue",
	}, []string{"name"})

	metrics = []k8smetrics.Registerable{
		depth, adds, latency, workDuration, unfinished, longestRunningProcessor, retries,
	}
)

type prometheusMetricsProvider struct {
}

var ocmNamedWorkQueueMetricsProvider = prometheusMetricsProvider{}

func init() {
	for _, m := range metrics {
		legacyregistry.MustRegister(m)
	}
	workqueue.SetProvider(ocmNamedWorkQueueMetricsProvider)
}

func NewNamedWorkQueueMetrics(name string) {
	ocmNamedWorkQueueMetricsProvider.NewDepthMetric(name)
	ocmNamedWorkQueueMetricsProvider.NewAddsMetric(name)
	ocmNamedWorkQueueMetricsProvider.NewLatencyMetric(name)
	ocmNamedWorkQueueMetricsProvider.NewWorkDurationMetric(name)
	ocmNamedWorkQueueMetricsProvider.NewUnfinishedWorkSecondsMetric(name)
	ocmNamedWorkQueueMetricsProvider.NewLongestRunningProcessorSecondsMetric(name)
	ocmNamedWorkQueueMetricsProvider.NewRetriesMetric(name)
}

func (prometheusMetricsProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	return depth.WithLabelValues(name)
}

func (prometheusMetricsProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	return adds.WithLabelValues(name)
}

func (prometheusMetricsProvider) NewLatencyMetric(name string) workqueue.HistogramMetric {
	return latency.WithLabelValues(name)
}

func (prometheusMetricsProvider) NewWorkDurationMetric(name string) workqueue.HistogramMetric {
	return workDuration.WithLabelValues(name)
}

func (prometheusMetricsProvider) NewUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return unfinished.WithLabelValues(name)
}

func (prometheusMetricsProvider) NewLongestRunningProcessorSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return longestRunningProcessor.WithLabelValues(name)
}

func (prometheusMetricsProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	return retries.WithLabelValues(name)
}
