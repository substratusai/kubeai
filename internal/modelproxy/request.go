package modelproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
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

	id      string
	status  int
	model   string
	attempt int

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

// done should be called when the original client request is complete.
func (pr *proxyRequest) done() {
	pr.timer.ObserveDuration()
}

// parseModel attempts to determine the model from the request.
// It first checks the "X-Model" header, and if that is not set, it
// attempts to unmarshal the request body as JSON and extract the
// .model field.
func (pr *proxyRequest) parseModel() error {
	pr.model = pr.r.Header.Get("X-Model")
	if pr.model != "" {
		return nil
	}

	var err error
	// TODO (samos123): Improve this to not read the entire body into memory for files.
	pr.body, err = io.ReadAll(pr.r.Body)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	if pr.r.Header.Get("Content-Type") == "multipart/form-data" {
		// Parse the form data to get the model.
		mpReader := multipart.NewReader(bytes.NewReader(pr.body), pr.r.Header.Get("Content-Type"))
		form, err := mpReader.ReadForm(1 << 20)
		if err != nil {
			return fmt.Errorf("read form: %w", err)
		}
		if len(form.Value["model"]) == 0 {
			return fmt.Errorf("no model specified")
		}
		pr.model = form.Value["model"][0]
		if pr.model == "" {
			return fmt.Errorf("model was empty")
		}
		return nil
	} else {
		// Default to parsing model from JSON body.
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
	}

	return nil
}

// sendErrorResponse sends an error response to the client and
// records the status code. If the status code is 5xx, the error
// message is not included in the response body.
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

// httpRequest returns a new http.Request that is a clone of the original
// request, preserving the original request body even if it was already
// read (i.e. if the body was inspected to determine the model).
func (pr *proxyRequest) httpRequest() *http.Request {
	clone := pr.r.Clone(pr.r.Context())
	if pr.body != nil {
		clone.Body = io.NopCloser(bytes.NewReader(pr.body))
	}
	return clone
}
