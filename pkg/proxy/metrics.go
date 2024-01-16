package proxy

import (
	"io"
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
	statusCode  int
	wroteHeader bool
}

func newCaptureStatusCodeResponseWriter(responseWriter http.ResponseWriter) *captureStatusResponseWriter {
	return &captureStatusResponseWriter{ResponseWriter: responseWriter}
}

func (c *captureStatusResponseWriter) WriteHeader(code int) {
	c.wroteHeader = true
	c.statusCode = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *captureStatusResponseWriter) Write(b []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	return c.ResponseWriter.Write(b)
}

func (c *captureStatusResponseWriter) ReadFrom(re io.Reader) (int64, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	return c.ResponseWriter.(io.ReaderFrom).ReadFrom(re)
}
