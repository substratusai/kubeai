package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	const (
		model1 = "model1"
		model2 = "model2"

		maxRetries = 3
	)
	models := map[string]string{
		model1: "deploy1",
		model2: "deploy2",
	}

	specs := map[string]struct {
		reqBody    string
		reqHeaders map[string]string

		backendPanic bool
		backendCode  int
		backendBody  string

		expCode                int
		expBody                string
		expLabels              map[string]string
		expBackendRequestCount int
	}{
		"no model": {
			reqBody: "{}",
			expCode: http.StatusBadRequest,
			expBody: `{"error":"unable to parse model: no model specified"}` + "\n",
			expLabels: map[string]string{
				"model":       "",
				"status_code": "400",
			},
			expBackendRequestCount: 0,
		},
		"model not found": {
			reqBody: `{"model":"does-not-exist"}`,
			expCode: http.StatusNotFound,
			expBody: `{"error":"model not found: does-not-exist"}` + "\n",
			expLabels: map[string]string{
				"model":       "does-not-exist",
				"status_code": "404",
			},
			expBackendRequestCount: 0,
		},
		"happy 200 model in body": {
			reqBody:     fmt.Sprintf(`{"model":%q}`, model1),
			backendCode: http.StatusOK,
			backendBody: `{"result":"ok"}`,
			expCode:     http.StatusOK,
			expBody:     `{"result":"ok"}`,
			expLabels: map[string]string{
				"model":       model1,
				"status_code": "200",
			},
			expBackendRequestCount: 1,
		},
		"happy 200 model in header": {
			reqBody:     "{}",
			reqHeaders:  map[string]string{"X-Model": model1},
			backendCode: http.StatusOK,
			backendBody: `{"result":"ok"}`,
			expCode:     http.StatusOK,
			expBody:     `{"result":"ok"}`,
			expLabels: map[string]string{
				"model":       model1,
				"status_code": "200",
			},
			expBackendRequestCount: 1,
		},
		"retryable 500": {
			reqBody:     fmt.Sprintf(`{"model":%q}`, model1),
			backendCode: http.StatusInternalServerError,
			backendBody: `{"err":"oh no!"}`,
			expCode:     http.StatusInternalServerError,
			expBody:     `{"err":"oh no!"}`,
			expLabels: map[string]string{
				"model":       model1,
				"status_code": "500",
			},
			expBackendRequestCount: 1 + maxRetries,
		},
		"not retryable 400": {
			reqBody:     fmt.Sprintf(`{"model":%q}`, model1),
			backendCode: http.StatusBadRequest,
			backendBody: `{"err":"bad request"}`,
			expCode:     http.StatusBadRequest,
			expBody:     `{"err":"bad request"}`,
			expLabels: map[string]string{
				"model":       model1,
				"status_code": "400",
			},
			expBackendRequestCount: 1,
		},
		"good request but dropped connection": {
			reqBody:      fmt.Sprintf(`{"model":%q}`, model1),
			backendPanic: true,
			expCode:      http.StatusBadGateway,
			expBody:      `{"error":"Bad Gateway"}` + "\n",
			expLabels: map[string]string{
				"model":       model1,
				"status_code": "502",
			},
			expBackendRequestCount: 1 + maxRetries,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// Register metrics from a clean slate.
			httpDuration.Reset()
			metricsRegistry := prometheus.NewPedanticRegistry()
			MustRegister(metricsRegistry)

			// Mock backend.
			var backendRequestCount int
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				backendRequestCount++

				bdy, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.Equal(t, spec.reqBody, string(bdy), "The request body should reach the backend")

				if spec.backendPanic {
					// Panic should close connection.
					// https://pkg.go.dev/net/http#Handler
					panic("panicing on purpose")
				}

				if spec.backendCode != 0 {
					w.WriteHeader(spec.backendCode)
				}
				if spec.backendBody != "" {
					_, _ = w.Write([]byte(spec.backendBody))
				}
			}))

			// Setup handler.
			deploys := &testDeploymentManager{models: models}
			endpoints := &testEndpointManager{address: backend.Listener.Addr().String()}
			queues := &testQueueManager{}
			h := NewHandler(deploys, endpoints, queues)
			h.MaxRetries = maxRetries
			server := httptest.NewServer(h)

			// Issue request.
			client := &http.Client{}
			req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(spec.reqBody))
			require.NoError(t, err)
			for k, v := range spec.reqHeaders {
				req.Header.Add(k, v)
			}
			resp, err := client.Do(req)
			require.NoError(t, err, "The client request should not fail")
			defer resp.Body.Close()
			respBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			// Assert on response.
			assert.Equal(t, spec.expCode, resp.StatusCode, "Unexpected response code to client")
			assert.Equal(t, spec.expBody, string(respBody), "Unexpected response body to client")
			assert.Equal(t, spec.expBackendRequestCount, backendRequestCount, "Unexpected number of requests sent to backend")
			assert.Equal(t, spec.expBackendRequestCount, endpoints.hostRequestCount, "Unexpected number of requests for backend hosts")

			// Assert on metrics.
			gathered, err := metricsRegistry.Gather()
			require.NoError(t, err)
			require.Len(t, gathered, 1)
			require.Len(t, gathered[0].Metric, 1)
			assert.NotEmpty(t, gathered[0].Metric[0].GetHistogram().GetSampleCount())
			assert.Equal(t, spec.expLabels, toMap(gathered[0].Metric[0].Label))
		})
	}
}

func TestMetricsViaLinter(t *testing.T) {
	registry := prometheus.NewPedanticRegistry()
	MustRegister(registry)

	problems, err := testutil.GatherAndLint(registry)
	require.NoError(t, err)
	require.Empty(t, problems)
}

type testDeploymentManager struct {
	models map[string]string
}

func (t *testDeploymentManager) ResolveDeployment(model string) (string, bool) {
	deploy, ok := t.models[model]
	return deploy, ok
}

func (t *testDeploymentManager) AtLeastOne(model string) {

}

type testEndpointManager struct {
	address string

	requestedService string
	requestedPort    string

	hostRequestCount int
}

func (t *testEndpointManager) AwaitHostAddress(ctx context.Context, service, portName string) (string, error) {
	t.hostRequestCount++
	t.requestedService = service
	t.requestedPort = portName
	return t.address, nil
}

type testQueueManager struct {
	requestedDeploymentName string
	requestedID             string
}

func (t *testQueueManager) EnqueueAndWait(ctx context.Context, deploymentName, id string) func() {
	t.requestedDeploymentName = deploymentName
	t.requestedID = id
	return func() {}
}

func toMap(s []*io_prometheus_client.LabelPair) map[string]string {
	r := make(map[string]string, len(s))
	for _, v := range s {
		r[v.GetName()] = v.GetValue()
	}
	return r
}
