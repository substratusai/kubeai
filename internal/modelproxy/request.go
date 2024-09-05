package modelproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
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
	var bodyBuffer bytes.Buffer
	// Create a TeeReader to read the body content and save it in bodyBuffer.
	multiReader := io.TeeReader(pr.r.Body, &bodyBuffer)

	// Try to get the model from the header first
	pr.model = pr.r.Header.Get("X-Model")
	if pr.model != "" {
		// Save the body content
		pr.body = bodyBuffer.Bytes()
		return nil
	}

	// Error is ignored because we default to JSON handling if content-type is not set.
	mediaType, params, _ := mime.ParseMediaType(pr.r.Header.Get("Content-Type"))
	if mediaType == "multipart/form-data" {
		if params["boundary"] == "" {
			return fmt.Errorf("no boundary specified in multipart form data")
		}

		// Parse multipart form data while streaming and saving the body content
		mpReader := multipart.NewReader(multiReader, params["boundary"])
		form, err := mpReader.ReadForm(25 * 1024 * 1024)
		if err != nil {
			return fmt.Errorf("read form: %w", err)
		}
		defer form.RemoveAll()

		if len(form.Value["model"]) == 0 {
			return fmt.Errorf("no model specified")
		}
		pr.model = form.Value["model"][0]
		if pr.model == "" {
			return fmt.Errorf("model was empty")
		}

		// Remove the "model" field after processing to avoid validation issues.
		// This is a workaround for Faster Whisper.
		// See https://github.com/fedirz/faster-whisper-server/issues/71.
		delete(form.Value, "model")
		pr.body = pr.rebuildMultipartForm(form, params["boundary"]).Bytes()

		// Set the new body and headers
		pr.r.Body = io.NopCloser(bytes.NewReader(pr.body))
		pr.r.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", params["boundary"]))
		pr.r.ContentLength = int64(len(pr.body))
	} else {
		// Stream the body content to save it in pr.body
		bodyContent, err := io.ReadAll(multiReader)
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		pr.body = bodyContent

		// Use a streaming approach for JSON handling
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

// Helper function to rebuild the multipart form and return a byte buffer
func (pr *proxyRequest) rebuildMultipartForm(form *multipart.Form, boundary string) *bytes.Buffer {
	prBuffer := new(bytes.Buffer)
	writer := multipart.NewWriter(prBuffer)
	writer.SetBoundary(boundary)

	// Re-add form fields without "model"
	for key, values := range form.Value {
		for _, value := range values {
			writer.WriteField(key, value)
		}
	}

	// Re-add files
	for key, files := range form.File {
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				continue // Handle error as needed
			}
			defer file.Close()

			part, err := writer.CreateFormFile(key, fileHeader.Filename)
			if err != nil {
				continue // Handle error as needed
			}
			io.Copy(part, file) // Stream file content
		}
	}
	writer.Close()

	return prBuffer
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
