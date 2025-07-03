package krtcollections

import (
	"istio.io/istio/pkg/kube/controllers"
	"istio.io/istio/pkg/kube/krt"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kgateway-dev/kgateway/v2/pkg/metrics"
)

const (
	resourcesSubsystem = "resources"
)

var (
	resourcesManaged = metrics.NewGauge(
		metrics.GaugeOpts{
			Subsystem: resourcesSubsystem,
			Name:      "managed",
			Help:      "Current number of gateway resources currently managed",
		},
		[]string{"name", "namespace", "resource"},
	)
)

func ResourceMetricEventHandler[T client.Object](o krt.Event[T], resourceName, gatewayName string) {
	switch o.Event {
	case controllers.EventAdd:
		resourcesManaged.Add(1, resourceMetricLabels{
			Name:      gatewayName,
			Namespace: o.Latest().GetNamespace(),
			Resource:  resourceName,
		}.toMetricsLabels()...)
	case controllers.EventDelete:
		resourcesManaged.Sub(1, resourceMetricLabels{
			Name:      gatewayName,
			Namespace: o.Latest().GetNamespace(),
			Resource:  resourceName,
		}.toMetricsLabels()...)
	}
}

type resourceMetricLabels struct {
	Name      string
	Namespace string
	Resource  string
}

func (r resourceMetricLabels) toMetricsLabels() []metrics.Label {
	return []metrics.Label{
		{Name: "name", Value: r.Name},
		{Name: "namespace", Value: r.Namespace},
		{Name: "resource", Value: r.Resource},
	}
}

// ResetMetrics resets the metrics from this package.
// This is provided for testing purposes only.
func ResetMetrics() {
	resourcesManaged.Reset()
}
