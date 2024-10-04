package metrics

import (
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	MeterName = "kubeai.org"
)

// Metrics used to autoscale models:
var (
	InferenceRequestsActiveMetricName = "kubeai.inference.requests.active"
	InferenceRequestsActive           metric.Int64UpDownCounter
)

// Attributes:
var (
	AttrRequestModel = attribute.Key("request.model")
	AttrRequestType  = attribute.Key("request.type")
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
		return err
	}

	return nil
}

func OtelNameToPromName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

func OtelAttrToPromLabel(k attribute.Key) string {
	return OtelNameToPromName(string(k))
}
