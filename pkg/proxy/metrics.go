package proxy

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

var httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_response_time_seconds",
	Help:    "Duration of HTTP requests.",
	Buckets: prometheus.DefBuckets,
}, []string{"model", "status_code"})

var totalRetries = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "http_request_retry_total",
	Help: "Number of HTTP request retries.",
})

func MustRegister(r prometheus.Registerer) {
	r.MustRegister(httpDuration, totalRetries)
}

// captureStatusResponseWriter is a custom HTTP response writer that captures the status code.
type captureStatusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newCaptureStatusCodeResponseWriter(responseWriter http.ResponseWriter) *captureStatusResponseWriter {
	return &captureStatusResponseWriter{ResponseWriter: responseWriter}
}

func (srw *captureStatusResponseWriter) WriteHeader(code int) {
	srw.statusCode = code
	srw.ResponseWriter.WriteHeader(code)
}
