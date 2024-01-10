package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeHTTP(t *testing.T) {
	specs := map[string]struct {
		context    func() context.Context
		maxRetries int
		respStatus int
		expRetries int
	}{
		"no retry on 200": {
			context:    func() context.Context { return context.TODO() },
			maxRetries: 3,
			respStatus: http.StatusOK,
			expRetries: 0,
		},
		"no retry on 500": {
			context:    func() context.Context { return context.TODO() },
			maxRetries: 3,
			respStatus: http.StatusInternalServerError,
			expRetries: 0,
		},
		"max retries on 503": {
			context:    func() context.Context { return context.TODO() },
			maxRetries: 3,
			respStatus: http.StatusServiceUnavailable,
			expRetries: 3,
		},
		"max retries on 502": {
			context:    func() context.Context { return context.TODO() },
			maxRetries: 3,
			respStatus: http.StatusBadGateway,
			expRetries: 3,
		},
		"context cancelled": {
			context: func() context.Context {
				ctx, cancel := context.WithCancel(context.TODO())
				cancel()
				return ctx
			},
			maxRetries: 3,
			respStatus: http.StatusBadGateway,
			expRetries: 0,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			counterBefore := counterValue(t, totalRetries)
			req, err := http.NewRequestWithContext(spec.context(), "GET", "/test", nil)
			require.NoError(t, err)

			respRecorder := httptest.NewRecorder()

			var counter int
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				counter++
				w.WriteHeader(spec.respStatus)
			})

			// when
			middleware := NewRetryMiddleware(spec.maxRetries, testHandler)
			middleware.ServeHTTP(respRecorder, req)

			// then
			resp := respRecorder.Result()
			require.Equal(t, spec.respStatus, resp.StatusCode)
			assert.Equal(t, spec.expRetries, counter-1)

			assert.Equal(t, spec.expRetries, int(counterValue(t, totalRetries)-counterBefore))
		})
	}
}

func counterValue(t *testing.T, counter prometheus.Counter) float64 {
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(counter)
	gathered, err := registry.Gather()
	require.NoError(t, err)
	require.Len(t, gathered, 1)
	require.Len(t, gathered[0].Metric, 1)
	return gathered[0].Metric[0].GetCounter().GetValue()
}
