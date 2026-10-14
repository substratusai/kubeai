package apiutils

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"io"
	"mime/multipart"
	"strings"

	openaiv1 "github.com/kubeai-project/kubeai/api/openai/v1"
)

const SystemOnePath = "/v1/systemone"

// MaxSystemOneBodyBytes bounds buffering, including multipart image uploads.
const MaxSystemOneBodyBytes int64 = 32 << 20

var ErrRequestTooLarge = fmt.Errorf("request body too large")

// readSystemOneBody inspects only the routing envelope. Unlike transcription
// uploads, System One embeds its model in a JSON part named "request". Retain
// the entire original body so image bytes, option order and extensions survive
// forwarding and retries unchanged.
func (r *Request) readSystemOneBody(body io.Reader, mediaType string, params map[string]string) error {
	if mediaType != "application/json" && mediaType != "multipart/form-data" {
		return fmt.Errorf("%w: systemone requires application/json or multipart/form-data", ErrBadRequest)
	}
	raw, err := io.ReadAll(io.LimitReader(body, MaxSystemOneBodyBytes+1))
	if err != nil {
		return fmt.Errorf("%w: reading systemone body: %v", ErrBadRequest, err)
	}
	if int64(len(raw)) > MaxSystemOneBodyBytes {
		return ErrRequestTooLarge
	}
	envelope := raw
	if mediaType == "multipart/form-data" {
		if params["boundary"] == "" {
			return fmt.Errorf("%w: missing multipart boundary", ErrBadRequest)
		}
		envelope = nil
		reader := multipart.NewReader(bytes.NewReader(raw), params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("%w: invalid systemone multipart body: %v", ErrBadRequest, err)
			}
			if part.FormName() != "request" {
				continue
			}
			if envelope != nil || part.FileName() != "" {
				return fmt.Errorf("%w: expected exactly one JSON form field named 'request'", ErrBadRequest)
			}
			envelope, err = io.ReadAll(part)
			if err != nil {
				return fmt.Errorf("%w: reading request field: %v", ErrBadRequest, err)
			}
		}
		if envelope == nil {
			return fmt.Errorf("%w: missing JSON form field 'request'", ErrBadRequest)
		}
	}
	var request openaiv1.SystemOneRequest
	if err := json.Unmarshal(envelope, &request); err != nil {
		return fmt.Errorf("%w: invalid systemone JSON: %v", ErrBadRequest, err)
	}
	if strings.TrimSpace(request.Model) == "" {
		return fmt.Errorf("%w: missing 'model' field", ErrBadRequest)
	}
	// This endpoint preserves the envelope and does not rewrite adapter names.
	if strings.ContainsAny(request.Model, "_:") {
		return fmt.Errorf("%w: systemone does not support model adapters", ErrBadRequest)
	}
	r.modelRequest = &request
	r.RequestedModel, r.Model = request.Model, request.Model
	r.Body, r.ContentLength = raw, int64(len(raw))
	return nil
}
