package modelproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/substratusai/kubeai/internal/modelmetrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
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

		expRewrittenReqBody    string
		expCode                int
		expBody                string
		expModel               string
		expBackendRequestCount int
	}{
		"no model": {
			reqBody:                "{}",
			expCode:                http.StatusBadRequest,
			expBody:                `{"error":"unable to parse model: no model specified"}` + "\n",
			expModel:               "",
			expBackendRequestCount: 0,
		},
		"model not found": {
			reqBody:                `{"model":"does-not-exist"}`,
			expCode:                http.StatusNotFound,
			expBody:                `{"error":"model not found: does-not-exist"}` + "\n",
			expModel:               "does-not-exist",
			expBackendRequestCount: 0,
		},
		"happy 200 model in body": {
			reqBody:                fmt.Sprintf(`{"model":%q}`, model1),
			backendCode:            http.StatusOK,
			backendBody:            `{"result":"ok"}`,
			expCode:                http.StatusOK,
			expBody:                `{"result":"ok"}`,
			expModel:               model1,
			expBackendRequestCount: 1,
		},
		"happy 200 model in header": {
			reqBody:                "{}",
			reqHeaders:             map[string]string{"X-Model": model1},
			backendCode:            http.StatusOK,
			backendBody:            `{"result":"ok"}`,
			expCode:                http.StatusOK,
			expBody:                `{"result":"ok"}`,
			expModel:               model1,
			expBackendRequestCount: 1,
		},
		"happy 200 only model in form data": {
			reqHeaders: map[string]string{"Content-Type": "multipart/form-data; boundary=12345"},
			reqBody: fmt.Sprintf(
				"--12345\r\nContent-Disposition: form-data; name=\"model\"\r\n\r\n%s\r\n--12345--\r\n",
				model1,
			),
			// Proxied request should have the model omitted from the body.
			expRewrittenReqBody:    "\r\n--12345--\r\n",
			backendCode:            http.StatusOK,
			backendBody:            `{"result":"ok"}`,
			expCode:                http.StatusOK,
			expBody:                `{"result":"ok"}`,
			expModel:               model1,
			expBackendRequestCount: 1,
		},
		"happy 200 model with other content in form data": {
			reqHeaders: map[string]string{"Content-Type": "multipart/form-data; boundary=12345"},
			reqBody: fmt.Sprintf(""+
				"--12345\r\nContent-Disposition: form-data; name=\"model\"\r\n\r\n%s\r\n"+
				"--12345\r\nContent-Disposition: form-data; name=\"otherField\"\r\n\r\notherFieldValue\r\n--12345--\r\n",
				model1,
			),
			// Proxied request should have the model omitted from the body.
			expRewrittenReqBody: fmt.Sprintf("" +
				"--12345\r\nContent-Disposition: form-data; name=\"otherField\"\r\n\r\notherFieldValue\r\n--12345--\r\n",
			),
			backendCode:            http.StatusOK,
			backendBody:            `{"result":"ok"}`,
			expCode:                http.StatusOK,
			expBody:                `{"result":"ok"}`,
			expModel:               model1,
			expBackendRequestCount: 1,
		},
		"retryable 500": {
			reqBody:                fmt.Sprintf(`{"model":%q}`, model1),
			backendCode:            http.StatusInternalServerError,
			backendBody:            `{"err":"oh no!"}`,
			expCode:                http.StatusInternalServerError,
			expBody:                `{"err":"oh no!"}`,
			expModel:               model1,
			expBackendRequestCount: 1 + maxRetries,
		},
		"not retryable 400": {
			reqBody:                fmt.Sprintf(`{"model":%q}`, model1),
			backendCode:            http.StatusBadRequest,
			backendBody:            `{"err":"bad request"}`,
			expCode:                http.StatusBadRequest,
			expBody:                `{"err":"bad request"}`,
			expModel:               model1,
			expBackendRequestCount: 1,
		},
		"good request but dropped connection": {
			reqBody:                fmt.Sprintf(`{"model":%q}`, model1),
			backendPanic:           true,
			expCode:                http.StatusBadGateway,
			expBody:                `{"error":"Bad Gateway"}` + "\n",
			expModel:               model1,
			expBackendRequestCount: 1 + maxRetries,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			reader := metric.NewManualReader()
			mp := metric.NewMeterProvider(
				metric.WithReader(reader),
			)

			otel.SetMeterProvider(mp)
			require.NoError(t, modelmetrics.Init(mp.Meter(modelmetrics.MeterName)))

			// Mock backend.
			var backendRequestCount int
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				backendRequestCount++

				bdy, err := io.ReadAll(r.Body)
				assert.NoError(t, err, "The request body should be readable")

				if spec.expRewrittenReqBody != "" {
					assert.Equal(t, spec.expRewrittenReqBody, string(bdy), "The rewritten request body should reach the backend")
				} else {
					assert.Equal(t, spec.reqBody, string(bdy), "The exact request body should reach the backend")
				}

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
			testInf := &testModelInterface{
				models:  models,
				address: backend.Listener.Addr().String(),
			}
			h := NewHandler(testInf, testInf, maxRetries, nil)
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
			assert.Equal(t, spec.expBackendRequestCount, testInf.hostRequestCount, "Unexpected number of requests for backend hosts")

			// Assert on metrics.
			if spec.expModel != "" {
				resMet := metricdata.ResourceMetrics{}
				require.NoError(t, reader.Collect(context.Background(), &resMet))

				// TODO: Assert on the existance of the expected metric!!!

				// Assert on the model attribute. Note: This does not assert on the existance of a
				// metric with that attribute.
				metricdatatest.AssertHasAttributes(t, resMet, modelmetrics.AttrRequestModel.String(spec.expModel))
			}
		})
	}
}

type testModelInterface struct {
	address string

	requestedModel string

	hostRequestCount int

	models map[string]string
}

func (t *testModelInterface) ModelExists(ctx context.Context, model string) (bool, error) {
	_, ok := t.models[model]
	return ok, nil
}

func (t *testModelInterface) ScaleAtLeastOneReplica(ctx context.Context, model string) error {
	return nil
}

func (t *testModelInterface) AwaitBestAddress(ctx context.Context, model string) (string, func(), error) {
	t.hostRequestCount++
	t.requestedModel = model
	return t.address, func() {}, nil
}

func toMap(s []*io_prometheus_client.LabelPair) map[string]string {
	r := make(map[string]string, len(s))
	for _, v := range s {
		r[v.GetName()] = v.GetValue()
	}
	return r
}
