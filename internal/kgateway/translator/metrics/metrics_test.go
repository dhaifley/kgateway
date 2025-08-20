package metrics_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	. "github.com/kgateway-dev/kgateway/v2/internal/kgateway/translator/metrics"
	"github.com/kgateway-dev/kgateway/v2/pkg/metrics"
	"github.com/kgateway-dev/kgateway/v2/pkg/metrics/metricstest"
)

const (
	testTranslatorName string = "test-translator"
	testGatewayName    string = "test-gateway"
	testNamespace      string = "test-namespace"
)

func setupTest() {
	ResetMetrics()
}

func assertTranslationsRunning(currentMetrics metricstest.GatheredMetrics, translatorName string, count int) {
	currentMetrics.AssertMetric("kgateway_translator_translations_running", &metricstest.ExpectedMetric{
		Labels: []metrics.Label{
			{Name: "name", Value: testGatewayName},
			{Name: "namespace", Value: testNamespace},
			{Name: "translator", Value: translatorName},
		},
		Value: float64(count),
	})
}

func TestCollectTranslationMetrics_Success(t *testing.T) {
	setupTest()

	// Start translation
	finishFunc := CollectTranslationMetrics(TranslatorMetricLabels{
		Name:       testGatewayName,
		Namespace:  testNamespace,
		Translator: testTranslatorName,
	})

	// Check that the translations_running metric is 1
	currentMetrics := metricstest.MustGatherMetrics(t)
	assertTranslationsRunning(currentMetrics, testTranslatorName, 1)

	// Finish translation
	finishFunc(nil)
	currentMetrics = metricstest.MustGatherMetrics(t)

	// Check the translations_running metric
	assertTranslationsRunning(currentMetrics, testTranslatorName, 0)

	currentMetrics.AssertMetric("kgateway_translator_translations_total", &metricstest.ExpectedMetric{
		Labels: []metrics.Label{
			{Name: "name", Value: testGatewayName},
			{Name: "namespace", Value: testNamespace},
			{Name: "result", Value: "success"},
			{Name: "translator", Value: testTranslatorName},
		},
		Value: 1,
	})

	// Check the translation_duration_seconds metric
	currentMetrics.AssertMetricLabels("kgateway_translator_translation_duration_seconds", []metrics.Label{
		{Name: "name", Value: testGatewayName},
		{Name: "namespace", Value: testNamespace},
		{Name: "translator", Value: testTranslatorName},
	})
	currentMetrics.AssertHistogramPopulated("kgateway_translator_translation_duration_seconds")
}

func TestCollectTranslationMetrics_Error(t *testing.T) {
	setupTest()

	finishFunc := CollectTranslationMetrics(TranslatorMetricLabels{
		Name:       testGatewayName,
		Namespace:  testNamespace,
		Translator: testTranslatorName,
	})

	currentMetrics := metricstest.MustGatherMetrics(t)
	assertTranslationsRunning(currentMetrics, testTranslatorName, 1)

	finishFunc(assert.AnError)
	currentMetrics = metricstest.MustGatherMetrics(t)
	assertTranslationsRunning(currentMetrics, testTranslatorName, 0)

	currentMetrics.AssertMetric(
		"kgateway_translator_translations_total",
		&metricstest.ExpectedMetric{
			Labels: []metrics.Label{
				{Name: "name", Value: testGatewayName},
				{Name: "namespace", Value: testNamespace},
				{Name: "result", Value: "error"},
				{Name: "translator", Value: testTranslatorName},
			},
			Value: 1,
		},
	)

	currentMetrics.AssertMetricLabels("kgateway_translator_translation_duration_seconds", []metrics.Label{
		{Name: "name", Value: testGatewayName},
		{Name: "namespace", Value: testNamespace},
		{Name: "translator", Value: testTranslatorName},
	})
	currentMetrics.AssertHistogramPopulated("kgateway_translator_translation_duration_seconds")
}

func TestResourceSync(t *testing.T) {
	setupTest()

	details := ResourceSyncDetails{
		Gateway:      "test-gateway",
		Namespace:    "test-namespace",
		ResourceType: "Gateway",
		ResourceName: "test-gateway",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	StartResourceSyncMetricsProcessing(ctx)

	// Test for resource status sync metrics.
	StartResourceStatusSync(ResourceSyncDetails{
		Gateway:      details.Gateway,
		Namespace:    details.Namespace,
		ResourceType: details.ResourceType,
		ResourceName: details.ResourceName,
	})

	EndResourceStatusSync(details)

	gathered := metricstest.MustGatherMetricsContext(ctx, t,
		"kgateway_resources_status_syncs_started_total",
		"kgateway_resources_status_syncs_completed_total",
		"kgateway_resources_status_sync_duration_seconds",
	)

	gathered.AssertMetric("kgateway_resources_status_syncs_started_total", &metricstest.ExpectedMetric{
		Labels: []metrics.Label{
			{Name: "gateway", Value: details.Gateway},
			{Name: "namespace", Value: details.Namespace},
			{Name: "resource", Value: details.ResourceType},
		},
		Value: 1,
	})

	gathered.AssertMetric("kgateway_resources_status_syncs_completed_total", &metricstest.ExpectedMetric{
		Labels: []metrics.Label{
			{Name: "gateway", Value: details.Gateway},
			{Name: "namespace", Value: details.Namespace},
			{Name: "resource", Value: details.ResourceType},
		},
		Value: 1,
	})

	gathered.AssertMetricsLabels("kgateway_resources_status_sync_duration_seconds", [][]metrics.Label{{
		{Name: "gateway", Value: details.Gateway},
		{Name: "namespace", Value: details.Namespace},
		{Name: "resource", Value: details.ResourceType},
	}})
	gathered.AssertHistogramPopulated("kgateway_resources_status_sync_duration_seconds")

	// Test for resource XDS snapshot sync metrics.
	StartResourceXDSSync(ResourceSyncDetails{
		Gateway:      details.Gateway,
		Namespace:    details.Namespace,
		ResourceType: details.ResourceType,
		ResourceName: details.ResourceName,
	})

	EndResourceXDSSync(details)

	gathered = metricstest.MustGatherMetricsContext(ctx, t,
		"kgateway_xds_snapshot_syncs_total",
		"kgateway_xds_snapshot_sync_duration_seconds",
	)

	gathered.AssertMetric("kgateway_xds_snapshot_syncs_total", &metricstest.ExpectedMetric{
		Labels: []metrics.Label{
			{Name: "gateway", Value: details.Gateway},
			{Name: "namespace", Value: details.Namespace},
		},
		Value: 1,
	})

	gathered.AssertMetricsLabels("kgateway_xds_snapshot_sync_duration_seconds", [][]metrics.Label{{
		{Name: "gateway", Value: details.Gateway},
		{Name: "namespace", Value: details.Namespace},
	}})
	gathered.AssertHistogramPopulated("kgateway_xds_snapshot_sync_duration_seconds")
}

func TestSyncChannelFull(t *testing.T) {
	setupTest()

	details := ResourceSyncDetails{
		Gateway:      "test-gateway",
		Namespace:    "test-namespace",
		ResourceType: "test",
		ResourceName: "test-resource",
	}

	for i := 0; i < 1024; i++ {
		success := EndResourceXDSSync(details)
		assert.True(t, success)
	}

	// Channel will be full. Validate that EndResourceSync returns and logs an error and that the kgateway_resources_updates_dropped_total metric is incremented.
	c := make(chan struct{})
	defer close(c)

	overflowCount := 0
	numOverflows := 20

	for overflowCount < numOverflows {
		go func() {
			success := EndResourceXDSSync(details)
			assert.False(t, success)
			c <- struct{}{}
		}()

		select {
		case <-c: // Expect to return quickly
		case <-time.After(10 * time.Millisecond):
			t.Fatal("Expected EndResourceSync to return and log an error")
		}

		overflowCount++

		currentMetrics := metricstest.MustGatherMetrics(t)
		currentMetrics.AssertMetric("kgateway_resources_updates_dropped_total", &metricstest.ExpectedMetric{
			Labels: []metrics.Label{},
			Value:  float64(overflowCount),
		})
	}
}

func TestTranslationMetricsNotActive(t *testing.T) {
	metrics.SetActive(false)
	defer metrics.SetActive(true)

	setupTest()

	assert.False(t, metrics.Active())

	finishFunc := CollectTranslationMetrics(TranslatorMetricLabels{
		Name:       testGatewayName,
		Namespace:  testNamespace,
		Translator: testTranslatorName,
	})

	currentMetrics := metricstest.MustGatherMetrics(t)

	currentMetrics.AssertMetricNotExists("kgateway_translator_translations_running")

	finishFunc(nil)

	currentMetrics = metricstest.MustGatherMetrics(t)

	currentMetrics.AssertMetricNotExists("kgateway_translator_translations_running")
	currentMetrics.AssertMetricNotExists("kgateway_translator_translations_total")
	currentMetrics.AssertMetricNotExists("kgateway_translator_translation_duration_seconds")
}
