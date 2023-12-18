package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/google/uuid"
	"github.com/substratusai/lingo/pkg/deployments"
	"github.com/substratusai/lingo/pkg/endpoints"
	"github.com/substratusai/lingo/pkg/queue"
)

// Handler serves http requests for end-clients.
// It is also responsible for triggering scale-from-zero.
type Handler struct {
	Deployments *deployments.Manager
	Endpoints   *endpoints.Manager
	Queues      *queue.Manager
}

func NewHandler(deployments *deployments.Manager, endpoints *endpoints.Manager, queues *queue.Manager) *Handler {
	return &Handler{Deployments: deployments, Endpoints: endpoints, Queues: queues}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := uuid.New().String()
	log.Printf("request: %v", r.URL)

	w.Header().Set("X-Proxy", "lingo")

	// TODO: Only parse model for paths that would have a model.
	modelName, proxyRequest, err := parseModel(r)
	if err != nil || modelName == "" {
		log.Printf("error reading model from request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad request: unable to parse .model from JSON payload"))
		return
	}
	log.Println("model:", modelName)

	deploy, found := h.Deployments.ResolveDeployment(modelName)
	if !found {
		log.Printf("deployment not found for model: %v", err)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(fmt.Sprintf("Deployment for model not found: %v", modelName)))
		return
	}

	h.Deployments.AtLeastOne(deploy)

	log.Println("Entering queue", id)
	complete := h.Queues.EnqueueAndWait(r.Context(), deploy, id)
	log.Println("Admitted into queue", id)
	defer complete()

	log.Println("Waiting for IPs", id)
	host := h.Endpoints.GetHost(r.Context(), deploy, "http")
	log.Printf("Got host: %v, id: %v\n", host, id)

	// TODO: Avoid creating new reverse proxies for each request.
	// TODO: Consider implementing a round robin scheme.
	log.Printf("Proxying request to host %v: %v\n", host, id)
	newReverseProxy(host).ServeHTTP(w, proxyRequest)
}

// parseModel parses the model name from the request
// returns empty string when none found or an error for failures on the proxy request object
func parseModel(r *http.Request) (string, *http.Request, error) {
	// check context first, in case that we have parsed the request before or that some middleware has
	// set the model
	if model, ok := ModelName(r.Context()); ok {
		return model, r, nil
	}
	if model := r.Header.Get("X-Model"); model != "" {
		return model, r.WithContext(WithModelName(r.Context(), model)), nil
	}
	// parse request body for model name, ignore errors
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", r, nil
	}

	var payload struct {
		Model string `json:"model"`
	}
	var model string
	if err := json.Unmarshal(body, &payload); err == nil {
		model = payload.Model
	}

	// create new request object with model context
	proxyReq, err := http.NewRequestWithContext(WithModelName(r.Context(), model), r.Method, r.URL.String(), bytes.NewReader(body))
	if err != nil {
		return "", nil, fmt.Errorf("create proxy request: %w", err)
	}
	proxyReq.Header = r.Header
	if err := proxyReq.ParseForm(); err != nil {
		return "", nil, fmt.Errorf("parse proxy form: %w", err)
	}
	return model, proxyReq, nil
}

// AdditionalProxyRewrite is an injection point for modifying proxy requests.
// Used in tests.
var AdditionalProxyRewrite = func(*httputil.ProxyRequest) {}

func newReverseProxy(host string) *httputil.ReverseProxy {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(&url.URL{
				Scheme: "http",
				Host:   host,
			})
			r.Out.Host = r.In.Host
			AdditionalProxyRewrite(r)
		},
	}
	return proxy
}
