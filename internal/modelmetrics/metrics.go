package modelmetrics

import (
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	meter = otel.Meter("kubeai.org")
)

// Metrics used to autoscale models
var (
	metricNameInferenceRequestsActive     = "kubeai.inference.requests.active"
	PromMetricNameInferenceRequestsActive = strings.ReplaceAll(metricNameInferenceRequestsActive, ".", "_")
	InferenceRequestsActive               metric.Int64UpDownCounter
)

// Attributes
var (
	AttrRequestModel = attribute.Key("request.model")
	AttrRequestType  = attribute.Key("request.type")
)

const (
	AttrRequestTypeHTTP    = "http"
	AttrRequestTypeMessage = "message"
)

func init() {
	var err error
	InferenceRequestsActive, err = meter.Int64UpDownCounter(metricNameInferenceRequestsActive,
		metric.WithDescription("The number of active requests by model"),
	)
	if err != nil {
		panic(err)
	}
}
