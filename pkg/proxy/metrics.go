package proxy

import (
	"log"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

var httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_response_time_seconds",
	Help:    "Duration of HTTP requests.",
	Buckets: prometheus.DefBuckets,
}, []string{"model", "status_code"})

func MustRegister(r prometheus.Registerer) {
	r.MustRegister(httpDuration)
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (srw *statusResponseWriter) WriteHeader(code int) {
	srw.statusCode = code
	srw.ResponseWriter.WriteHeader(code)
}

func WithMetricsMiddleware(other http.Handler) http.Handler {
	return InstrumentHandlerFunc(other.ServeHTTP)
}

func InstrumentHandlerFunc(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		modelName, proxyRequest, err := parseModel(r)
		if err != nil {
			log.Printf("skipping request: error parsing model: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Bad request"))
			return
		}
		if modelName == "" {
			modelName = "unknown"
		}
		captureStatusResponse := &statusResponseWriter{ResponseWriter: w}
		timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
			httpDuration.WithLabelValues(modelName, strconv.Itoa(captureStatusResponse.statusCode)).Observe(v)
		}))
		defer timer.ObserveDuration()

		handlerFunc(captureStatusResponse, proxyRequest)

		timer.ObserveDuration()
	}
}
