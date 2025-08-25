package metrics_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"istio.io/istio/pkg/kube/krt"
	"istio.io/istio/pkg/kube/krt/krttest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	. "github.com/kgateway-dev/kgateway/v2/internal/kgateway/krtcollections/metrics"
	"github.com/kgateway-dev/kgateway/v2/pkg/metrics"
	"github.com/kgateway-dev/kgateway/v2/pkg/metrics/metricstest"
)

func setupTest() {
	ResetMetrics()
}

func TestCollectionMetrics(t *testing.T) {
	testCases := []struct {
		name   string
		inputs []any
	}{
		{
			name: "HTTPRoute",
			inputs: []any{
				&gwv1.HTTPRoute{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ns",
						Labels:    map[string]string{"a": "b"},
					},
					Spec: gwv1.HTTPRouteSpec{
						CommonRouteSpec: gwv1.CommonRouteSpec{
							ParentRefs: []gwv1.ParentReference{{
								Name: "test-gateway",
								Kind: ptr.To(gwv1.Kind("Gateway")),
							}},
						},
						Hostnames: []gwv1.Hostname{"example.com"},
						Rules: []gwv1.HTTPRouteRule{{
							Matches: []gwv1.HTTPRouteMatch{{
								Path: &gwv1.HTTPPathMatch{
									Type:  ptr.To(gwv1.PathMatchPathPrefix),
									Value: ptr.To("/"),
								},
							}},
						}},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupTest()

			done := make(chan struct{})
			mock := krttest.NewMock(t, tc.inputs)
			mockHTTPRoutes := krttest.GetMockCollection[*gwv1.HTTPRoute](mock)

			eventHandler := GetResourceMetricEventHandler[*gwv1.HTTPRoute]()

			metrics.RegisterEvents(mockHTTPRoutes, func(o krt.Event[*gwv1.HTTPRoute]) {
				eventHandler(o)

				done <- struct{}{}
			})

			<-done

			gathered := metricstest.MustGatherMetrics(t)

			gathered.AssertMetric("kgateway_resources_managed", &metricstest.ExpectedMetric{
				Labels: []metrics.Label{
					{Name: "namespace", Value: "ns"},
					{Name: "parent", Value: "test-gateway"},
					{Name: "resource", Value: "HTTPRoute"},
				},
				Value: 1,
			})
		})
	}
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
