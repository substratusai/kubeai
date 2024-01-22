package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/substratusai/lingo/pkg/endpoints"
	"github.com/substratusai/lingo/pkg/queue"
)

func TestProxy(t *testing.T) {
	specs := map[string]struct {
		request *http.Request
		expCode int
	}{
		"ok": {
			request: httptest.NewRequest(http.MethodGet, "/", strings.NewReader(`{"model":"my_model"}`)),
			expCode: http.StatusOK,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			deplMgr := mockDeploymentSource{
				ResolveDeploymentFn: func(model string) (string, bool) {
					return "my-deployment", true
				},
				AtLeastOneFn: func(deploymentName string) {},
			}
			em, err := endpoints.NewManager(&fakeManager{}, func(deploymentName string, replicas int) {})
			require.NoError(t, err)
			em.SetEndpoints("my-deployment", map[string]struct{}{"my-ip": {}}, map[string]int32{"my-port": 8080})
			h := NewHandler(deplMgr, em, queue.NewManager(10), 1)

			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				em.SetEndpoints("my-deployment", map[string]struct{}{"my-other-ip": {}}, map[string]int32{"my-other-port": 8080})
				w.WriteHeader(http.StatusOK)
			}))
			recorder := httptest.NewRecorder()

			AdditionalProxyRewrite = func(r *httputil.ProxyRequest) {
				r.SetURL(&url.URL{Scheme: "http", Host: svr.Listener.Addr().String()})
			}

			// when
			h.ServeHTTP(recorder, spec.request)
			// then
			assert.Equal(t, spec.expCode, recorder.Code)
		})
	}
}

type mockDeploymentSource struct {
	ResolveDeploymentFn func(model string) (string, bool)
	AtLeastOneFn        func(deploymentName string)
}

func (m mockDeploymentSource) ResolveDeployment(model string) (string, bool) {
	if m.ResolveDeploymentFn == nil {
		panic("not expected to be called")
	}
	return m.ResolveDeploymentFn(model)
}

func (m mockDeploymentSource) AtLeastOne(deploymentName string) {
	if m.AtLeastOneFn == nil {
		panic("not expected to be called")
	}
	m.AtLeastOneFn(deploymentName)
}
