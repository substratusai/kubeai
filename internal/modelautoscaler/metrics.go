package modelautoscaler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/substratusai/kubeai/internal/modelmetrics"
)

type metricsAggregation struct {
	activeRequestsByModel map[string]int64
}

func newMetricsAggregation() *metricsAggregation {
	return &metricsAggregation{
		activeRequestsByModel: make(map[string]int64),
	}
}

func scrapeAndAggregateMetrics(agg *metricsAggregation, url string) error {
	// Perform the HTTP GET request
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to scrape metrics: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	// Use the expfmt library to parse the Prometheus metrics
	parser := expfmt.TextParser{}
	metricFamilies, err := parser.TextToMetricFamilies(strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to parse metrics: %v", err)
	}

	if fam, ok := metricFamilies[modelmetrics.PromMetricNameInferenceRequestsActive]; ok {
		for _, m := range fam.Metric {
			for _, label := range m.Label {
				if label.GetName() == modelmetrics.PromMetricNameInferenceRequestsActive {
					agg.activeRequestsByModel[label.GetValue()] = getMetricsValue(fam, m)
				}
			}
		}
	}

	return nil
}

func getMetricsValue(mf *io_prometheus_client.MetricFamily, m *io_prometheus_client.Metric) int64 {
	if mf.GetType() == io_prometheus_client.MetricType_GAUGE && m.Gauge != nil {
		return int64(m.GetGauge().GetValue())
	} else if mf.GetType() == io_prometheus_client.MetricType_COUNTER && m.Counter != nil {
		return int64(m.GetCounter().GetValue())
	}
	return 0
}

//type vLLMMetrics struct {
//	// Metric name: "vllm:num_requests_waiting"
//	numRequestsWaiting float64
//	// Metric name: "vllm:num_requests_running"
//	numRequestsRunning float64
//}
//
//func (v *vLLMMetrics) CurrentRequests() float64 {
//	return v.numRequestsWaiting + v.numRequestsRunning
//}
//
//func (v *vLLMMetrics) Aggregate(name string, mf *io_prometheus_client.MetricFamily, m *io_prometheus_client.Metric) {
//	var val float64
//	if mf.GetType() == io_prometheus_client.MetricType_GAUGE && m.Gauge != nil {
//		val = m.GetGauge().GetValue()
//	} else if mf.GetType() == io_prometheus_client.MetricType_COUNTER && m.Counter != nil {
//		val = m.GetCounter().GetValue()
//	}
//
//	switch name {
//	case "vllm:num_requests_waiting":
//		v.numRequestsWaiting += val
//	case "vllm:num_requests_running":
//		v.numRequestsRunning += val
//	}
//}
//
