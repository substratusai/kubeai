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
		reqMethod string
		reqPath   string
		reqBody   string

		backendCode int
		backendBody string

		expCode                int
		expBody                string
		expLabels              map[string]string
		expBackendRequestCount int
	}{
		"no model": {
			reqMethod: http.MethodPost,
			reqPath:   "/",
			reqBody:   "{}",
			expCode:   http.StatusBadRequest,
			expBody:   `{"error":"unable to parse model: no model specified"}` + "\n",
			expLabels: map[string]string{
				"model":       "",
				"status_code": "400",
			},
			expBackendRequestCount: 0,
		},
		"model not found": {
			reqMethod: http.MethodPost,
			reqPath:   "/",
			reqBody:   `{"model":"does-not-exist"}`,
			expCode:   http.StatusNotFound,
			expBody:   `{"error":"model not found: does-not-exist"}` + "\n",
			expLabels: map[string]string{
				"model":       "does-not-exist",
				"status_code": "404",
			},
			expBackendRequestCount: 0,
		},
		"happy 200": {
			reqMethod:   http.MethodPost,
			reqPath:     "/",
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
		"retryable 500": {
			reqMethod:   http.MethodPost,
			reqPath:     "/",
			reqBody:     fmt.Sprintf(`{"model":%q}`, model1),
			backendCode: http.StatusInternalServerError,
			backendBody: `{"err":"oh no!"}`,
			expCode:     http.StatusBadGateway,
			expBody:     `{"error":"Bad Gateway"}` + "\n",
			expLabels: map[string]string{
				"model":       model1,
				"status_code": "502",
			},
			expBackendRequestCount: 1 + maxRetries,
		},
		"not retryable 400": {
			reqMethod:   http.MethodPost,
			reqPath:     "/",
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
			req, err := http.NewRequest(spec.reqMethod, server.URL+spec.reqPath, strings.NewReader(spec.reqBody))
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			respBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			// Assert on response.
			assert.Equal(t, spec.expCode, resp.StatusCode)
			assert.Equal(t, spec.expBody, string(respBody))
			assert.Equal(t, spec.expBackendRequestCount, backendRequestCount)

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
}

func (t *testEndpointManager) AwaitHostAddress(ctx context.Context, service, portName string) (string, error) {
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
