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

// CaptureStatusCodeResponseWriter is an interface that extends the http.ResponseWriter interface and provides a method for reading the status code of an HTTP response.
type CaptureStatusCodeResponseWriter interface {
	http.ResponseWriter
	StatusCodeCapturer
}

// captureStatusResponseWriter is a custom HTTP response writer that implements CaptureStatusCodeResponseWriter
type captureStatusResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func NewCaptureStatusCodeResponseWriter(responseWriter http.ResponseWriter) CaptureStatusCodeResponseWriter {
	if o, ok := responseWriter.(CaptureStatusCodeResponseWriter); ok { // nothing to do as code is captured already
		return o
	}
	c := &captureStatusResponseWriter{ResponseWriter: responseWriter}
	if _, ok := responseWriter.(io.ReaderFrom); ok {
		return &captureStatusResponseWriterWithReadFrom{captureStatusResponseWriter: c}
	}
	return c
}

func (c *captureStatusResponseWriter) CapturedStatusCode() int {
	return c.statusCode
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

type captureStatusResponseWriterWithReadFrom struct {
	*captureStatusResponseWriter
}

func (c *captureStatusResponseWriterWithReadFrom) ReadFrom(re io.Reader) (int64, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	return c.ResponseWriter.(io.ReaderFrom).ReadFrom(re)
}
