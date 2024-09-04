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

	// Error is ignored because we default to JSON handling if content-type is not set.
	mediaType, params, _ := mime.ParseMediaType(pr.r.Header.Get("Content-Type"))
	if mediaType == "multipart/form-data" {
		// Parse the form data to get the model.
		if params["boundary"] == "" {
			return fmt.Errorf("no boundary specified in multipart form data")
		}
		mpReader := multipart.NewReader(bytes.NewReader(pr.body), params["boundary"])
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
		// Remove the "model" field from the form after processing.
		// This is needed because Faster Whisper does strong validation
		// on the model field. See https://github.com/fedirz/faster-whisper-server/issues/71
		delete(form.Value, "model")

		// Re-add the form fields back, without "model".
		var buffer bytes.Buffer
		writer := multipart.NewWriter(&buffer)
		for key, values := range form.Value {
			for _, value := range values {
				if err := writer.WriteField(key, value); err != nil {
					return fmt.Errorf("write field: %w", err)
				}
			}
		}

		// Re-add the files back into the form.
		for key, files := range form.File {
			for _, fileHeader := range files {
				// Open the file
				file, err := fileHeader.Open()
				if err != nil {
					return fmt.Errorf("file open: %w", err)
				}
				defer file.Close()

				// Create a new form file in the writer
				part, err := writer.CreateFormFile(key, fileHeader.Filename)
				if err != nil {
					return fmt.Errorf("create form file: %w", err)
				}

				// Copy the file content into the form
				if _, err = io.Copy(part, file); err != nil {
					return fmt.Errorf("copy file: %w", err)
				}
			}
		}
		writer.Close()
		pr.body = buffer.Bytes()
		// Update the Content-Type header to reflect the new boundary.
		pr.r.Header.Set("Content-Type", writer.FormDataContentType())
		// The Content-Length header must be updated to reflect the new body length.
		pr.r.ContentLength = int64(buffer.Len())
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
