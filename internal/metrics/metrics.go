package metrics

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	MeterName = "kubeai.org"
)

// Metrics used to autoscale models:
var (
	InferenceRequestsActiveMetricName               = "kubeai.inference.requests.active"
	InferenceRequestsActive                         metric.Int64UpDownCounter
	InferenceRequestsHashLookupIterationsMetricName = "kubeai.inference.requests.hash.lookup.iterations"
	InferenceRequestsHashLookupIterations           metric.Int64Histogram
	InferenceRequestsHashLookupInitialMetricName    = "kubeai.inference.requests.hash.lookup.initial"
	InferenceRequestsHashLookupInitial              metric.Int64Counter
	InferenceRequestsHashLookupFinalMetricName      = "kubeai.inference.requests.hash.lookup.final"
	InferenceRequestsHashLookupFinal                metric.Int64Counter
	InferenceRequestsHashLookupDefaultMetricName    = "kubeai.inference.requests.hash.lookup.default"
	InferenceRequestsHashLookupDefault              metric.Int64Counter
)

// Attributes:
var (
	AttrRequestModel = attribute.Key("request.model")
	AttrRequestType  = attribute.Key("request.type")
	AttrEndpoint     = attribute.Key("endpoint")
)

// Attribute values:
const (
	AttrRequestTypeHTTP    = "http"
	AttrRequestTypeMessage = "message"
)

// Init sets up global metric variables.
func Init(meter metric.Meter) error {
	var err error
	InferenceRequestsActive, err = meter.Int64UpDownCounter(InferenceRequestsActiveMetricName,
		metric.WithDescription("The number of active requests by model"),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", InferenceRequestsActiveMetricName, err)
	}
	InferenceRequestsHashLookupIterations, err = meter.Int64Histogram(InferenceRequestsHashLookupIterationsMetricName,
		metric.WithDescription("The number of vnodes considered while searching for the best endpoint for a request"),
		metric.WithExplicitBucketBoundaries(1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", InferenceRequestsHashLookupIterationsMetricName, err)
	}

	InferenceRequestsHashLookupInitial, err = meter.Int64Counter(InferenceRequestsHashLookupInitialMetricName,
		metric.WithDescription("The number of times an endpoint was the initial preference of the load balancer"),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", InferenceRequestsHashLookupInitialMetricName, err)
	}
	InferenceRequestsHashLookupFinal, err = meter.Int64Counter(InferenceRequestsHashLookupFinalMetricName,
		metric.WithDescription("The number of times an endpoint was the final choice of the load balancer"),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", InferenceRequestsHashLookupFinalMetricName, err)
	}
	InferenceRequestsHashLookupDefault, err = meter.Int64Counter(InferenceRequestsHashLookupDefaultMetricName,
		metric.WithDescription("The number of times an endpoint was the default choice of the load balancer"),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", InferenceRequestsHashLookupDefaultMetricName, err)
	}

	return nil
}

func OtelNameToPromName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

func OtelAttrToPromLabel(k attribute.Key) string {
	return OtelNameToPromName(string(k))
}
