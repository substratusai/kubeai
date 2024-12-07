package apiutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const (
	// adapterSeparator is the separator used to split model and adapter names
	// in API requests.
	//
	// Alternatives considered:
	//
	// "-" (hyphen): This is a common separator in Kubernetes resource names.
	// "." (dot): This is a common separator in model versions "llama-3.2".
	// "/" (slash): This would be incompatible with specifying model names inbetween slashes in URL paths (i.e. "/some-api/models/<model-id>/details").
	// ":" (colon): This might cause problems when specifying model names before colons in URL paths (see example below).
	//
	// See example of a path used in the Gemini API (https://ai.google.dev/gemini-api/docs/text-generation?lang=rest):
	// "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=$GOOGLE_API_KEY"
	adapterSeparator = "_"
)

// SplitModelAdapter splits a requested model name into KubeAI
// Model.metadata.name and Model.spec.adapters[].name.
func SplitModelAdapter(s string) (model, adapter string) {
	parts := strings.SplitN(s, adapterSeparator, 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

// MergeModelAdapter merges a model and adapter name into a single string.
func MergeModelAdapter(model, adapter string) string {
	if adapter == "" {
		return model
	}
	return model + adapterSeparator + adapter
}

type Request struct {
	Body []byte

	Selectors []string

	ID string

	// RequestedModel is the model name requested by the client.
	// This might contain the adapter name as well.
	RequestedModel string

	Model   string
	Adapter string

	ContentLength int64
}

func ParseRequest(body io.Reader, headers http.Header) (*Request, error) {
	pr := &Request{
		ID: uuid.New().String(),
	}

	pr.Selectors = headers.Values("X-Label-Selector")

	// Parse media type (with params - which are used for multipart form data)
	var (
		contentType = headers.Get("Content-Type")
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
			return nil, fmt.Errorf("parse media type: %w", err)
		}
	}

	switch mediaType {
	// Multipart form data is used for endpoints that accept file uploads:
	case "multipart/form-data":
		boundary := mediaParams["boundary"]
		if boundary == "" {
			return nil, fmt.Errorf("no boundary specified in multipart form data")
		}

		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		// Keep the same boundary as the initial request (probably not necessary)
		mw.SetBoundary(boundary)

		// Iterate over the parts of the multipart form data:
		// - If the part is named "model", save the value to the proxy request.
		// - Otherwise, just copy the part to the new multipart writer.
		mr := multipart.NewReader(body, boundary)
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("interating over multipart form: %w", err)
			}

			if p.FormName() == "model" {
				value, err := io.ReadAll(p)
				if err != nil {
					return nil, fmt.Errorf("reading multipart form value: %w", err)
				}
				pr.Model, pr.Adapter = SplitModelAdapter(string(value))
				pr.RequestedModel = string(value)
				// WORKAROUND ALERT:
				// Omit the "model" field from the proxy request to avoid FasterWhisper validation issues:
				// See https://github.com/fedirz/faster-whisper-server/issues/71
				continue
			}

			// Copy the part to the new multipart writer.
			pp, err := mw.CreatePart(p.Header)
			if err != nil {
				return nil, fmt.Errorf("creating part: %w", err)
			}
			if _, err := io.Copy(pp, p); err != nil {
				return nil, fmt.Errorf("copying part: %w", err)
			}
		}

		// Fully write to buffer.
		if err := mw.Close(); err != nil {
			return nil, fmt.Errorf("closing multipart writer: %w", err)
		}
		pr.Body = buf.Bytes()
		// Set a new content length based on the new body - which had the "model" field removed.
		pr.ContentLength = int64(len(pr.Body))

	// Assume "application/json":
	default:
		if err := pr.readModelFromBody(body); err != nil {
			return nil, fmt.Errorf("reading model from body: %w", err)
		}
	}

	return pr, nil
}

func (pr *Request) readModelFromBody(r io.Reader) error {
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

	pr.RequestedModel = modelStr
	pr.Model, pr.Adapter = SplitModelAdapter(modelStr)

	if pr.Adapter != "" {
		// vLLM expects the adapter to be in the model field.
		payload["model"] = pr.Adapter
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("remarshalling: %w", err)
	}
	pr.Body = body
	pr.ContentLength = int64(len(pr.Body))

	return nil
}
