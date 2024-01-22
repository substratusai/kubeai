package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/substratusai/lingo/pkg/deployments"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func TestMetrics(t *testing.T) {
	specs := map[string]struct {
		request   *http.Request
		expCode   int
		expLabels map[string]string
	}{
		"with mode name": {
			request: httptest.NewRequest(http.MethodGet, "/", strings.NewReader(`{"model":"my_model"}`)),
			expCode: http.StatusNotFound,
			expLabels: map[string]string{
				"model":       "my_model",
				"status_code": "404",
			},
		},
		"unknown model name": {
			request: httptest.NewRequest(http.MethodGet, "/", strings.NewReader("{}")),
			expCode: http.StatusBadRequest,
			expLabels: map[string]string{
				"model":       "unknown",
				"status_code": "400",
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			httpDuration.Reset()
			registry := prometheus.NewPedanticRegistry()
			MustRegister(registry)

			deplMgr, err := deployments.NewManager(&fakeManager{})
			require.NoError(t, err)
			h := NewHandler(deplMgr, nil, nil)
			recorder := httptest.NewRecorder()

			// when
			h.ServeHTTP(recorder, spec.request)

			// then
			assert.Equal(t, spec.expCode, recorder.Code)
			gathered, err := registry.Gather()
			require.NoError(t, err)
			require.Len(t, gathered, 2)
			require.Equal(t, "http_response_time_seconds", *gathered[1].Name)
			require.Len(t, gathered[1].Metric, 1)
			assert.NotEmpty(t, gathered[1].Metric[0].GetHistogram().GetSampleCount())
			assert.Equal(t, spec.expLabels, toMap(gathered[1].Metric[0].Label))
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

func TestCaptureStatusCodeResponseWriters(t *testing.T) {
	specs := map[string]struct {
		rspWriter http.ResponseWriter
		expType   any
		write     func(t *testing.T, r http.ResponseWriter, content string)
	}{
		"implements statusCodeCapturer": {
			rspWriter: &discardableResponseWriter{
				headerBuf:      make(http.Header),
				ResponseWriter: httptest.NewRecorder(),
				isDiscardable:  func(status int) bool { return false },
			},
			expType: &discardableResponseWriter{},
			write: func(t *testing.T, r http.ResponseWriter, content string) {
				r.WriteHeader(200)
			},
		},
		"implements io.ReaderFrom": {
			rspWriter: &testResponseWriter{ResponseRecorder: httptest.NewRecorder()},
			expType:   &captureStatusResponseWriterWithReadFrom{},
			write: func(t *testing.T, r http.ResponseWriter, content string) {
				n, err := r.(io.ReaderFrom).ReadFrom(strings.NewReader(content))
				require.NoError(t, err)
				assert.Equal(t, len(content), int(n))
			},
		},
		"default": {
			rspWriter: httptest.NewRecorder(),
			expType:   &captureStatusResponseWriter{},
			write: func(t *testing.T, r http.ResponseWriter, content string) {
				n, err := r.Write([]byte(content))
				require.NoError(t, err)
				assert.Equal(t, len(content), n)
			},
		},
	}

	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			instance := NewCaptureStatusCodeResponseWriter(spec.rspWriter)
			require.IsType(t, spec.expType, instance)
			spec.write(t, instance, "foo")
			gotCode := instance.CapturedStatusCode()
			assert.Equal(t, http.StatusOK, gotCode)
		})
	}
}

func toMap(s []*io_prometheus_client.LabelPair) map[string]string {
	r := make(map[string]string, len(s))
	for _, v := range s {
		r[v.GetName()] = v.GetValue()
	}
	return r
}

// for test setup only
type fakeManager struct {
	ctrl.Manager
}

func (m *fakeManager) GetCache() cache.Cache {
	return nil
}

func (m *fakeManager) GetScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	return s
}

func (m *fakeManager) Add(_ manager.Runnable) error {
	return nil
}

func (m *fakeManager) GetLogger() logr.Logger {
	return logr.Discard()
}

func (m *fakeManager) GetControllerOptions() config.Controller {
	return config.Controller{}
}

func (m *fakeManager) GetClient() client.Client {
	return nil
}
