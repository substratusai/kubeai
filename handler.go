package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Handler serves http requests.
// It is also responsible for triggering scale-from-zero.
type Handler struct {
	Deployments *DeploymentManager
	Endpoints   *EndpointsManager
	FIFO        *FIFOQueueManager
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("request: %v", r.URL)

	w.Header().Set("X-Proxy", "lingo")

	// TODO: Only parse model for paths that would have a model.
	modelName, proxyRequest, err := parseModel(r)
	if err != nil {
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

	log.Println("Entering queue")
	unqueue := h.FIFO.Enqueue(deploy)
	log.Println("Admitted into queue")
	defer unqueue()

	log.Println("Waiting for IPs")
	host := h.Endpoints.GetHost(r.Context(), deploy)
	log.Printf("Got host: %v \n", host)

	// TODO: Avoid creating new reverse proxies for each request.
	// TODO: Consider implementing a round robin scheme.
	newReverseProxy(host).ServeHTTP(w, proxyRequest)
}

func parseModel(r *http.Request) (string, *http.Request, error) {
	if model := r.Header.Get("X-Model"); model != "" {
		return model, r, nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", nil, fmt.Errorf("reading body: %w", err)
	}

	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", nil, fmt.Errorf("parsing body as json: %w", err)
	}

	if payload.Model == "" {
		return "", nil, fmt.Errorf("missing .model in request body")
	}

	proxyReq, _ := http.NewRequest(r.Method, r.URL.String(), bytes.NewReader(body))
	proxyReq.Header = r.Header
	proxyReq.ParseForm()
	return payload.Model, proxyReq, nil
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
