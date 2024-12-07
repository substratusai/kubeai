package apiutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"

	"github.com/google/uuid"
)

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
	r := &Request{
		ID: uuid.New().String(),
	}

	r.Selectors = headers.Values("X-Label-Selector")

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
		if err := r.readyMultiPartBody(body, mediaParams); err != nil {
			return nil, fmt.Errorf("reading multipart form data: %w", err)
		}

	// Assume "application/json":
	default:
		if err := r.readJSONBody(body); err != nil {
			return nil, fmt.Errorf("reading model from body: %w", err)
		}
	}

	return r, nil
}

func (r *Request) readyMultiPartBody(body io.Reader, mediaParams map[string]string) error {
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
	mr := multipart.NewReader(body, boundary)
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
			r.Model, r.Adapter = SplitModelAdapter(string(value))
			r.RequestedModel = string(value)
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
	r.Body = buf.Bytes()
	// Set a new content length based on the new body - which had the "model" field removed.
	r.ContentLength = int64(len(r.Body))

	return nil
}

func (r *Request) readJSONBody(body io.Reader) error {
	var payload map[string]interface{}
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
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

	r.RequestedModel = modelStr
	r.Model, r.Adapter = SplitModelAdapter(modelStr)

	if r.Adapter != "" {
		// vLLM expects the adapter to be in the model field.
		payload["model"] = r.Adapter
	}

	rewritten, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("remarshalling: %w", err)
	}
	r.Body = rewritten
	r.ContentLength = int64(len(r.Body))

	return nil
}
