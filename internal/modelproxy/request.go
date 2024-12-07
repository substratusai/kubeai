package modelproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/substratusai/kubeai/internal/apiutils"
)

// proxyRequest keeps track of the state of a request that is to be proxied.
type proxyRequest struct {
	*apiutils.Request

	// r is the original request. It is stored here so that is can be cloned
	// and sent to the backend while preserving the original request body.
	http    *http.Request
	status  int
	attempt int
}

func newProxyRequest(r *http.Request) (*proxyRequest, error) {
	pr := &proxyRequest{
		http:   r,
		status: http.StatusOK,
	}

	apiReq, err := apiutils.ParseRequest(r.Body, r.Header)
	if err != nil {
		return pr, err
	}
	// The content length might have changed after the body was read and rewritten.
	r.ContentLength = apiReq.ContentLength
	pr.Request = apiReq

	return pr, nil
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
	clone := pr.http.Clone(pr.http.Context())
	if pr.Body != nil {
		clone.Body = io.NopCloser(bytes.NewReader(pr.Body))
	}
	return clone
}
