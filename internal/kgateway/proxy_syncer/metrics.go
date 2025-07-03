package proxy_syncer

import (
	"strings"
	"sync"
	"time"

	"github.com/kgateway-dev/kgateway/v2/pkg/metrics"
)

const (
	statusSubsystem    = "status_syncer"
	syncerNameLabel    = "syncer"
	snapshotSubsystem  = "xds_snapshot"
	resourcesSubsystem = "resources"
)

var (
	statusSyncsTotal = metrics.NewCounter(
		metrics.CounterOpts{
			Subsystem: statusSubsystem,
			Name:      "status_syncs_total",
			Help:      "Total status syncs",
		},
		[]string{syncerNameLabel, "result"},
	)
	statusSyncDuration = metrics.NewHistogram(
		metrics.HistogramOpts{
			Subsystem:                       statusSubsystem,
			Name:                            "status_sync_duration_seconds",
			Help:                            "Status sync duration",
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: time.Hour,
		},
		[]string{syncerNameLabel},
	)
	statusSyncResources = metrics.NewGauge(
		metrics.GaugeOpts{
			Subsystem: statusSubsystem,
			Name:      "resources",
			Help:      "Current number of resources managed by the status syncer",
		},
		[]string{syncerNameLabel, "name", "namespace", "resource"},
	)

	snapshotTransformsTotal = metrics.NewCounter(
		metrics.CounterOpts{
			Subsystem: snapshotSubsystem,
			Name:      "client_transforms_total",
			Help:      "Total client snapshot transforms",
		},
		[]string{"client", "namespace", "result"},
	)
	snapshotTransformDuration = metrics.NewHistogram(
		metrics.HistogramOpts{
			Subsystem:                       snapshotSubsystem,
			Name:                            "client_transform_duration_seconds",
			Help:                            "Client snapshot transform duration",
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: time.Hour,
		},
		[]string{"client", "namespace"},
	)
	snapshotResources = metrics.NewGauge(
		metrics.GaugeOpts{
			Subsystem: snapshotSubsystem,
			Name:      "resources",
			Help:      "Current number of gateway resources currently managed",
		},
		[]string{"client", "namespace", "resource"},
	)

	resourcesSyncsCompletedTotal = metrics.NewCounter(
		metrics.CounterOpts{
			Subsystem: resourcesSubsystem,
			Name:      "syncs_completed_total",
			Help:      "The total number of syncs completed for the specified gateway resource",
		},
		[]string{"name", "namespace", "resource"})
	resourcesSyncDuration = metrics.NewHistogram(
		metrics.HistogramOpts{
			Subsystem:                       resourcesSubsystem,
			Name:                            "sync_duration_seconds",
			Help:                            "Resource status sync duration",
			Buckets:                         []float64{0.1, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600, 1200, 1800},
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: time.Hour,
		},
		[]string{"name", "namespace", "resource"},
	)
)

// snapshotResourcesMetricLabels defines the labels for XDS snapshot resources metrics.
type snapshotResourcesMetricLabels struct {
	Client    string
	Namespace string
	Resource  string
}

func (r snapshotResourcesMetricLabels) toMetricsLabels() []metrics.Label {
	return []metrics.Label{
		{Name: "client", Value: r.Client},
		{Name: "namespace", Value: r.Namespace},
		{Name: "resource", Value: r.Resource},
	}
}

// resourcesMetricLabels defines the labels for the gateway resources metrics.
type resourcesMetricLabels struct {
	Name      string
	Namespace string
	Resource  string
}

func (r resourcesMetricLabels) toMetricsLabels() []metrics.Label {
	return []metrics.Label{
		{Name: "name", Value: r.Name},
		{Name: "namespace", Value: r.Namespace},
		{Name: "resource", Value: r.Resource},
	}
}

// statusSyncResourcesMetricLabels defines the labels for the syncer resources metrics.
type statusSyncResourcesMetricLabels struct {
	Name      string
	Namespace string
	Resource  string
}

func (r statusSyncResourcesMetricLabels) toMetricsLabels(syncer string) []metrics.Label {
	return []metrics.Label{
		{Name: syncerNameLabel, Value: syncer},
		{Name: "name", Value: r.Name},
		{Name: "namespace", Value: r.Namespace},
		{Name: "resource", Value: r.Resource},
	}
}

// statusSyncMetricsRecorder defines the interface for recording status syncer metrics.
type statusSyncMetricsRecorder interface {
	StatusSyncStart() func(error)
	ResetResources(resource string)
	SetResources(labels statusSyncResourcesMetricLabels, count int)
	IncResources(labels statusSyncResourcesMetricLabels)
	DecResources(labels statusSyncResourcesMetricLabels)
}

// statusSyncMetrics records metrics for status syncer operations.
type statusSyncMetrics struct {
	syncerName         string
	statusSyncsTotal   metrics.Counter
	statusSyncDuration metrics.Histogram
	resources          metrics.Gauge
	resourceNames      map[string]map[string]map[string]struct{}
	resourcesLock      sync.Mutex
}

// newStatusSyncMetricsRecorder creates a new recorder for status syncer metrics.
func newStatusSyncMetricsRecorder(syncerName string) statusSyncMetricsRecorder {
	if !metrics.Active() {
		return &nullStatusSyncMetricsRecorder{}
	}

	m := &statusSyncMetrics{
		syncerName:         syncerName,
		statusSyncsTotal:   statusSyncsTotal,
		statusSyncDuration: statusSyncDuration,
		resources:          statusSyncResources,
		resourceNames:      make(map[string]map[string]map[string]struct{}),
		resourcesLock:      sync.Mutex{},
	}

	return m
}

type nullStatusSyncMetricsRecorder struct{}

func (m *nullStatusSyncMetricsRecorder) StatusSyncStart() func(error) {
	return func(err error) {}
}

func (m *nullStatusSyncMetricsRecorder) ResetResources(resource string) {}

func (m *nullStatusSyncMetricsRecorder) SetResources(labels statusSyncResourcesMetricLabels, count int) {
}

func (m *nullStatusSyncMetricsRecorder) IncResources(labels statusSyncResourcesMetricLabels) {}

func (m *nullStatusSyncMetricsRecorder) DecResources(labels statusSyncResourcesMetricLabels) {}

// StatusSyncStart is called at the start of a status sync function to begin metrics
// collection and returns a function called at the end to complete metrics recording.
func (m *statusSyncMetrics) StatusSyncStart() func(error) {
	start := time.Now()

	return func(err error) {
		duration := time.Since(start)

		m.statusSyncDuration.Observe(duration.Seconds(),
			metrics.Label{Name: syncerNameLabel, Value: m.syncerName})

		result := "success"
		if err != nil {
			result = "error"
		}

		m.statusSyncsTotal.Inc([]metrics.Label{
			{Name: syncerNameLabel, Value: m.syncerName},
			{Name: "result", Value: result},
		}...)
	}
}

// ResetResources resets the resource count gauge for a specified resource.
func (m *statusSyncMetrics) ResetResources(resource string) {
	m.resourcesLock.Lock()

	namespaces, exists := m.resourceNames[resource]
	if !exists {
		m.resourcesLock.Unlock()

		return
	}

	delete(m.resourceNames, resource)

	m.resourcesLock.Unlock()

	for namespace, names := range namespaces {
		for name := range names {
			m.resources.Set(0, []metrics.Label{
				{Name: syncerNameLabel, Value: m.syncerName},
				{Name: "name", Value: name},
				{Name: "namespace", Value: namespace},
				{Name: "resource", Value: resource},
			}...)
		}
	}
}

// updateResourceNames updates the internal map of resource names.
func (m *statusSyncMetrics) updateResourceNames(labels statusSyncResourcesMetricLabels) {
	m.resourcesLock.Lock()

	if _, exists := m.resourceNames[labels.Resource]; !exists {
		m.resourceNames[labels.Resource] = make(map[string]map[string]struct{})
	}

	if _, exists := m.resourceNames[labels.Resource][labels.Namespace]; !exists {
		m.resourceNames[labels.Resource][labels.Namespace] = make(map[string]struct{})
	}

	m.resourceNames[labels.Resource][labels.Namespace][labels.Name] = struct{}{}

	m.resourcesLock.Unlock()
}

// SetResources updates the resource count gauge.
func (m *statusSyncMetrics) SetResources(labels statusSyncResourcesMetricLabels, count int) {
	m.updateResourceNames(labels)

	m.resources.Set(float64(count), labels.toMetricsLabels(m.syncerName)...)
}

// IncResources increments the resource count gauge.
func (m *statusSyncMetrics) IncResources(labels statusSyncResourcesMetricLabels) {
	m.updateResourceNames(labels)

	m.resources.Add(1, labels.toMetricsLabels(m.syncerName)...)
}

// DecResources decrements the resource count gauge.
func (m *statusSyncMetrics) DecResources(labels statusSyncResourcesMetricLabels) {
	m.updateResourceNames(labels)

	m.resources.Sub(1, labels.toMetricsLabels(m.syncerName)...)
}

// snapshotMetricsRecorder defines the interface for recording XDS snapshot metrics.
type snapshotMetricsRecorder interface {
	transformStart(string) func(error)
}

// snapshotMetrics records metrics for collection operations.
type snapshotMetrics struct {
	transformsTotal   metrics.Counter
	transformDuration metrics.Histogram
}

var _ snapshotMetricsRecorder = &snapshotMetrics{}

// newSnapshotMetricsRecorder creates a new recorder for XDS snapshot metrics.
func newSnapshotMetricsRecorder() snapshotMetricsRecorder {
	if !metrics.Active() {
		return &nullSnapshotMetricsRecorder{}
	}

	m := &snapshotMetrics{
		transformsTotal:   snapshotTransformsTotal,
		transformDuration: snapshotTransformDuration,
	}

	return m
}

// transformStart is called at the start of a transform function to begin metrics
// collection and returns a function called at the end to complete metrics recording.
func (m *snapshotMetrics) transformStart(clientKey string) func(error) {
	start := time.Now()

	name := clientKey
	namespace := "unknown"

	pks := strings.SplitN(name, "~", 5)
	if len(pks) > 1 {
		namespace = pks[1]
	}

	if len(pks) > 2 {
		name = pks[2]
	}

	return func(err error) {
		result := "success"
		if err != nil {
			result = "error"
		}

		m.transformsTotal.Inc([]metrics.Label{
			{Name: "client", Value: name},
			{Name: "namespace", Value: namespace},
			{Name: "result", Value: result},
		}...)

		duration := time.Since(start)

		m.transformDuration.Observe(duration.Seconds(), []metrics.Label{
			{Name: "client", Value: name},
			{Name: "namespace", Value: namespace},
		}...)
	}
}

type nullSnapshotMetricsRecorder struct{}

func (m *nullSnapshotMetricsRecorder) transformStart(string) func(error) {
	return func(err error) {}
}

// ResetMetrics resets the metrics from this package.
// This is provided for testing purposes only.
func ResetMetrics() {
	statusSyncDuration.Reset()
	statusSyncsTotal.Reset()
	statusSyncResources.Reset()
	snapshotTransformsTotal.Reset()
	snapshotTransformDuration.Reset()
	snapshotResources.Reset()
	resourcesSyncsCompletedTotal.Reset()
}
