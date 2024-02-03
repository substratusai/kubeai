package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
)

// proxyRequest keeps track of the state of a request that is to be proxied.
type proxyRequest struct {
	// r is the original request. It is stored here so that is can be cloned
	// and sent to the backend while preserving the original request body.
	r *http.Request
	// body will be stored here if the request body needed to be read
	// in order to determine the model.
	body []byte

	// metadata:

	id                string
	status            int
	model             string
	backendDeployment string
	attempt           int

	// metrics:

	timer *prometheus.Timer
}

func newProxyRequest(r *http.Request) *proxyRequest {
	pr := &proxyRequest{
		r:      r,
		id:     uuid.New().String(),
		status: http.StatusOK,
	}

	pr.timer = prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		httpDuration.WithLabelValues(pr.model, strconv.Itoa(pr.status)).Observe(v)
	}))

	return pr

}

func (p *proxyRequest) done() {
	p.timer.ObserveDuration()
}

func (pr *proxyRequest) parseModel() error {
	pr.model = pr.r.Header.Get("X-Model")
	if pr.model != "" {
		return nil
	}

	var err error
	pr.body, err = io.ReadAll(pr.r.Body)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(pr.body, &payload); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}
	pr.model = payload.Model

	if pr.model == "" {
		return fmt.Errorf("no model specified")
	}

	return nil
}

func (pr *proxyRequest) sendErrorResponse(w http.ResponseWriter, status int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("sending error response: %v: %v", status, msg)

	pr.setStatus(w, status)

	if status >= 500 {
		// Don't leak internal error messages to the client.
		msg = http.StatusText(status)
	}

	if err := json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{
		Error: msg,
	}); err != nil {
		log.Printf("error encoding error response: %v", err)
	}
}

func (pr *proxyRequest) setStatus(w http.ResponseWriter, code int) {
	pr.status = code
	w.WriteHeader(code)
}

func (pr *proxyRequest) httpRequest() *http.Request {
	clone := pr.r.Clone(pr.r.Context())
	if pr.body != nil {
		clone.Body = io.NopCloser(bytes.NewReader(pr.body))
	}
	return clone
}
