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

	"github.com/google/uuid"
	"github.com/substratusai/kubeai/internal/apiutils"
)

// proxyRequest keeps track of the state of a request that is to be proxied.
type proxyRequest struct {
	// r is the original request. It is stored here so that is can be cloned
	// and sent to the backend while preserving the original request body.
	r *http.Request
	// body will be stored here if the request body needed to be read
	// in order to determine the model.
	body []byte

	selectors []string

	id             string
	status         int
	requestedModel string
	model          string
	adapter        string
	attempt        int
}

func newProxyRequest(r *http.Request) *proxyRequest {
	pr := &proxyRequest{
		r:      r,
		id:     uuid.New().String(),
		status: http.StatusOK,
	}

	return pr
}

// parse attempts to determine the model from the request.
// It first checks the "X-Model" header, and if that is not set, it
// attempts to unmarshal the request body as JSON and extract the
// .model field.
func (pr *proxyRequest) parse() error {
	pr.selectors = pr.r.Header.Values("X-Label-Selector")

	// Try to get the model from the header first
	if headerModel := pr.r.Header.Get("X-Model"); headerModel != "" {
		pr.model, pr.adapter = apiutils.SplitModelAdapter(headerModel)
		pr.requestedModel = headerModel
		// Save the body content (required to support retries of the proxy request)
		body, err := io.ReadAll(pr.r.Body)
		if err != nil {
			return fmt.Errorf("reading body: %w", err)
		}
		pr.body = body
		return nil
	}

	// Parse media type (with params - which are used for multipart form data)
	var (
		contentType = pr.r.Header.Get("Content-Type")
		mediaType   string
		mediaParams map[string]string
	)
	if contentType == "" {
		mediaType = "application/json"
		mediaParams = map[string]string{}
	} else {
		var err error
		mediaType, mediaParams, err = mime.ParseMediaType(contentType)
		if err != nil {
			return fmt.Errorf("parse media type: %w", err)
		}
	}

	switch mediaType {
	// Multipart form data is used for endpoints that accept file uploads:
	case "multipart/form-data":
		boundary := mediaParams["boundary"]
		if boundary == "" {
			return fmt.Errorf("no boundary specified in multipart form data")
		}

		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		// Keep the same boundary as the initial request (probably not necessary)
		mw.SetBoundary(boundary)

		// Iterate over the parts of the multipart form data:
		// - If the part is named "model", save the value to the proxy request.
		// - Otherwise, just copy the part to the new multipart writer.
		mr := multipart.NewReader(pr.r.Body, boundary)
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("interating over multipart form: %w", err)
			}

			if p.FormName() == "model" {
				value, err := io.ReadAll(p)
				if err != nil {
					return fmt.Errorf("reading multipart form value: %w", err)
				}
				pr.model, pr.adapter = apiutils.SplitModelAdapter(string(value))
				pr.requestedModel = string(value)
				// WORKAROUND ALERT:
				// Omit the "model" field from the proxy request to avoid FasterWhisper validation issues:
				// See https://github.com/fedirz/faster-whisper-server/issues/71
				continue
			}

			// Copy the part to the new multipart writer.
			pp, err := mw.CreatePart(p.Header)
			if err != nil {
				return fmt.Errorf("creating part: %w", err)
			}
			if _, err := io.Copy(pp, p); err != nil {
				return fmt.Errorf("copying part: %w", err)
			}
		}

		// Fully write to buffer.
		if err := mw.Close(); err != nil {
			return fmt.Errorf("closing multipart writer: %w", err)
		}
		pr.body = buf.Bytes()
		// Set a new content length based on the new body - which had the "model" field removed.
		pr.r.ContentLength = int64(len(pr.body))

	// Assume "application/json":
	default:
		if err := pr.readModelFromBody(pr.r.Body); err != nil {
			return fmt.Errorf("reading model from body: %w", err)
		}
	}

	return nil
}

func (pr *proxyRequest) readModelFromBody(r io.ReadCloser) error {
	var payload map[string]interface{}
	if err := json.NewDecoder(r).Decode(&payload); err != nil {
		return fmt.Errorf("decoding: %w", err)
	}
	modelInf, ok := payload["model"]
	if !ok {
		return fmt.Errorf("missing 'model' field")
	}
	modelStr, ok := modelInf.(string)
	if !ok {
		return fmt.Errorf("field 'model' should be a string")
	}

	pr.requestedModel = modelStr
	pr.model, pr.adapter = apiutils.SplitModelAdapter(modelStr)

	var bodyChanged bool
	if pr.adapter != "" {
		// vLLM expects the adapter to be in the model field.
		payload["model"] = pr.adapter
		bodyChanged = true
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("remarshalling: %w", err)
	}
	pr.body = body
	if bodyChanged {
		// Set a new content length based on the new body - which had the "model" field changed.
		pr.r.ContentLength = int64(len(pr.body))
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
