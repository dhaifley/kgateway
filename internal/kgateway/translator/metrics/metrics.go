package metrics

import (
	"sync"
	"time"

	"github.com/kgateway-dev/kgateway/v2/pkg/metrics"
)

const (
	translatorSubsystem = "translator"
	translatorNameLabel = "translator"
	resourcesSubsystem  = "resources"
)

var (
	translationsTotal = metrics.NewCounter(
		metrics.CounterOpts{
			Subsystem: translatorSubsystem,
			Name:      "translations_total",
			Help:      "Total translations",
		},
		[]string{translatorNameLabel, "result"},
	)
	translationDuration = metrics.NewHistogram(
		metrics.HistogramOpts{
			Subsystem:                       translatorSubsystem,
			Name:                            "translation_duration_seconds",
			Help:                            "Translation duration",
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: time.Hour,
		},
		[]string{translatorNameLabel},
	)
	translationsRunning = metrics.NewGauge(
		metrics.GaugeOpts{
			Subsystem: translatorSubsystem,
			Name:      "translations_running",
			Help:      "Number of translations currently running",
		},
		[]string{translatorNameLabel},
	)

	resourcesSyncsStartedTotal = metrics.NewCounter(metrics.CounterOpts{
		Subsystem: resourcesSubsystem,
		Name:      "syncs_started_total",
		Help:      "The total number of syncs started for the specified gateway resource",
	},
		[]string{"name", "namespace", "resource"})
)

type ResourceMetricLabels struct {
	Name      string
	Namespace string
	Resource  string
}

func (r ResourceMetricLabels) toMetricsLabels() []metrics.Label {
	return []metrics.Label{
		{Name: "name", Value: r.Name},
		{Name: "namespace", Value: r.Namespace},
		{Name: "resource", Value: r.Resource},
	}
}

// TranslatorMetricsRecorder defines the interface for recording translator metrics.
type TranslatorMetricsRecorder interface {
	TranslationStart() func(error)
}

// translatorMetrics records metrics for translator operations.
type translatorMetrics struct {
	translatorName      string
	translationsTotal   metrics.Counter
	translationDuration metrics.Histogram
	translationsRunning metrics.Gauge
}

// NewTranslatorMetricsRecorder creates a new recorder for translator metrics.
func NewTranslatorMetricsRecorder(translatorName string) TranslatorMetricsRecorder {
	if !metrics.Active() {
		return &nullTranslatorMetricsRecorder{}
	}

	m := &translatorMetrics{
		translatorName:      translatorName,
		translationsTotal:   translationsTotal,
		translationDuration: translationDuration,
		translationsRunning: translationsRunning,
	}

	return m
}

// TranslationStart is called at the start of a translation function to begin metrics
// collection and returns a function called at the end to complete metrics recording.
func (m *translatorMetrics) TranslationStart() func(error) {
	start := time.Now()

	m.translationsRunning.Add(1,
		metrics.Label{Name: translatorNameLabel, Value: m.translatorName})

	return func(err error) {
		duration := time.Since(start)

		m.translationDuration.Observe(duration.Seconds(),
			metrics.Label{Name: translatorNameLabel, Value: m.translatorName})

		result := "success"
		if err != nil {
			result = "error"
		}

		m.translationsTotal.Inc([]metrics.Label{
			{Name: translatorNameLabel, Value: m.translatorName},
			{Name: "result", Value: result},
		}...)

		m.translationsRunning.Sub(1,
			metrics.Label{Name: translatorNameLabel, Value: m.translatorName})
	}
}

var _ TranslatorMetricsRecorder = &translatorMetrics{}

type nullTranslatorMetricsRecorder struct{}

func (m *nullTranslatorMetricsRecorder) TranslationStart() func(error) {
	return func(err error) {}
}

// IncTranslationsTotal increments the total translations counter.
func IncResourcesSyncsStartedTotal(resourceName string, labels ResourceMetricLabels) {
	resourceKey := GetResourceKey(labels.Namespace, labels.Name, labels.Resource, resourceName)

	StartResourceSync(resourceKey)

	resourcesSyncsStartedTotal.Inc(labels.toMetricsLabels()...)
}

// resourceSyncStartTimes tracks the start times of resource syncs.
type resourceSyncStartTimes struct {
	sync.RWMutex
	times map[string]time.Time
}

var startTimes = &resourceSyncStartTimes{}

// StartResourceSync records the start time of a sync for a given resource key.
func StartResourceSync(resourceKey string) {
	startTimes.Lock()
	defer startTimes.Unlock()

	if startTimes.times == nil {
		startTimes.times = make(map[string]time.Time)
	}

	startTimes.times[resourceKey] = time.Now()
}

// GetResourceSyncStartTime retrieves the start time of a sync for a given resource key.
func GetResourceSyncStartTime(resourceKey string) time.Time {
	startTimes.RLock()
	defer startTimes.RUnlock()

	if st, exists := startTimes.times[resourceKey]; exists {
		return st
	}

	return time.Now() // Return current time if not found.
}

// GetResourceKey constructs a resource key from the provided parameters.
func GetResourceKey(namespace, gatewayName, resourceType, resourceName string) string {
	return namespace + "/" + gatewayName + "/" + resourceType + "/" + resourceName
}

// ResetMetrics resets the metrics from this package.
// This is provided for testing purposes only.
func ResetMetrics() {
	translationsTotal.Reset()
	translationDuration.Reset()
	translationsRunning.Reset()
	resourcesSyncsStartedTotal.Reset()

	startTimes.Lock()
	defer startTimes.Unlock()
	startTimes.times = make(map[string]time.Time)
}
