package proxy

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

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

type DeploymentManager interface {
	ResolveDeployment(model string) (string, bool)
	AtLeastOne(model string)
}

type EndpointManager interface {
	AwaitHostAddress(ctx context.Context, service, portName string) (string, error)
}

type QueueManager interface {
	EnqueueAndWait(ctx context.Context, deploymentName, id string) func()
}

// Handler serves http requests for end-clients.
// It is also responsible for triggering scale-from-zero.
type Handler struct {
	Deployments DeploymentManager
	Endpoints   EndpointManager
	Queues      QueueManager

	MaxRetries int
	RetryCodes map[int]struct{}
}

func NewHandler(
	deployments DeploymentManager,
	endpoints EndpointManager,
	queues QueueManager,
) *Handler {
	return &Handler{
		Deployments: deployments,
		Endpoints:   endpoints,
		Queues:      queues,
	}
}

var defaultRetryCodes = map[int]struct{}{
	http.StatusInternalServerError: {},
	http.StatusBadGateway:          {},
	http.StatusServiceUnavailable:  {},
	http.StatusGatewayTimeout:      {},
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("url: %v", r.URL)

	w.Header().Set("X-Proxy", "lingo")

	pr := newProxyRequest(r)
	defer pr.done()

	// TODO: Only parse model for paths that would have a model.
	if err := pr.parseModel(); err != nil {
		pr.sendErrorResponse(w, http.StatusBadRequest, "unable to parse model: %v", err)
		return
	}

	log.Println("model:", pr.model)

	var backendExists bool
	pr.backendDeployment, backendExists = h.Deployments.ResolveDeployment(pr.model)
	if !backendExists {
		pr.sendErrorResponse(w, http.StatusNotFound, "model not found: %v", pr.model)
		return
	}

	// Ensure the backend is scaled to at least one Pod.
	h.Deployments.AtLeastOne(pr.backendDeployment)

	log.Printf("Entering queue: %v", pr.id)

	// Wait to until the request is admitted into the queue before proceeding with
	// serving the request.
	complete := h.Queues.EnqueueAndWait(r.Context(), pr.backendDeployment, pr.id)
	defer complete()

	log.Printf("Admitted into queue: %v", pr.id)

	// After waiting for the request to be admitted, double check that the model
	// still exists. It's possible that the model was deleted while waiting.
	// This would lead to a long subequent wait with the host lookup.
	pr.backendDeployment, backendExists = h.Deployments.ResolveDeployment(pr.model)
	if !backendExists {
		pr.sendErrorResponse(w, http.StatusNotFound, "model not found after being dequeued: %v", pr.model)
		return
	}

	h.proxyHTTP(w, pr)
}

// AdditionalProxyRewrite is an injection point for modifying proxy requests.
// Used in tests.
var AdditionalProxyRewrite = func(*httputil.ProxyRequest) {}

func (h *Handler) proxyHTTP(w http.ResponseWriter, pr *proxyRequest) {
	log.Printf("Waiting for host: %v", pr.id)

	host, err := h.Endpoints.AwaitHostAddress(pr.r.Context(), pr.backendDeployment, "http")
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			pr.sendErrorResponse(w, http.StatusInternalServerError, "request cancelled while finding host: %v", err)
			return
		case errors.Is(err, context.DeadlineExceeded):
			pr.sendErrorResponse(w, http.StatusGatewayTimeout, "request timeout while finding host: %v", err)
			return
		default:
			pr.sendErrorResponse(w, http.StatusGatewayTimeout, "unable to find host: %v", err)
			return
		}
	}

	log.Printf("Got host: %v, id: %v\n", host, pr.id)

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

	proxy.ModifyResponse = func(r *http.Response) error {
		// Record the response for metrics.
		pr.status = r.StatusCode

		if h.isRetryCode(r.StatusCode) {
			// Returning an error will trigger the ErrorHandler.
			return errors.New("retry")
		}

		return nil
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if err != nil && pr.attempt < h.MaxRetries {
			pr.attempt++

			log.Printf("Retrying request (%v/%v): %v", pr.attempt, h.MaxRetries, pr.id)
			h.proxyHTTP(w, pr)
			return
		}

		pr.sendErrorResponse(w, http.StatusBadGateway, "proxy: exceeded retries: %v/%v", pr.attempt, h.MaxRetries)
	}

	log.Printf("Proxying request to host %v: %v\n", host, pr.id)
	proxy.ServeHTTP(w, pr.httpRequest())
}

func (h *Handler) isRetryCode(status int) bool {
	var retry bool
	// TODO: avoid the nil check here and set a default map in the constructor.
	if h.RetryCodes != nil {
		_, retry = h.RetryCodes[status]
	} else {
		_, retry = defaultRetryCodes[status]
	}
	return retry
}
