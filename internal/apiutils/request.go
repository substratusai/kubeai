package apiutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"

	"context"

	"github.com/google/uuid"
	v1 "github.com/substratusai/kubeai/api/v1"
)

var (
	ErrBadRequest    = fmt.Errorf("bad request")
	ErrModelNotFound = fmt.Errorf("model not found")
)

type Request struct {
	Body        []byte
	bodyPayload map[string]interface{}

	Selectors []string

	ID string

	// RequestedModel is the model name requested by the client.
	// This might contain the adapter name as well.
	RequestedModel string

	Model   string
	Adapter string

	LoadBalancing v1.LoadBalancing

	Prefix string

	ContentLength int64
}

type ModelClient interface {
	LookupModel(ctx context.Context, model, adapter string, selectors []string) (*v1.Model, error)
}

func ParseRequest(ctx context.Context, client ModelClient, body io.Reader, path string, headers http.Header) (*Request, error) {
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
			return nil, fmt.Errorf("%w: parse media type: %w", ErrBadRequest, err)
		}
	}

	switch mediaType {
	// Multipart form data is used for endpoints that accept file uploads:
	case "multipart/form-data":
		if err := r.readyMultiPartBody(body, mediaParams); err != nil {
			return nil, fmt.Errorf("%w: reading multipart form data: %w", ErrBadRequest, err)
		}

	// Assume "application/json":
	default:
		if err := r.readJSONBody(body); err != nil {
			return nil, fmt.Errorf("%w: reading model from body: %w", ErrBadRequest, err)
		}
	}

	if err := r.lookupModel(ctx, client, path); err != nil {
		return nil, err
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

func (r *Request) lookupModel(ctx context.Context, client ModelClient, path string) error {
	model, err := client.LookupModel(ctx, r.Model, r.Adapter, r.Selectors)
	if err != nil {
		return fmt.Errorf("lookup model: %w", err)
	}
	if model == nil {
		return fmt.Errorf("%w: %q", ErrModelNotFound, r.RequestedModel)
	}

	if r.bodyPayload == nil {
		return nil
	}
	defer func() {
		r.bodyPayload = nil
	}()

	r.LoadBalancing = model.Spec.LoadBalancing
	if r.LoadBalancing.Strategy == v1.PrefixHashStrategy {
		switch path {
		case "/v1/completions":
			r.Prefix = getPrefixForCompletionRequest(r.bodyPayload, r.LoadBalancing.PrefixHash.PrefixCharLength)
		case "/v1/chat/completions":
			r.Prefix = getPrefixForChatCompletionRequest(r.bodyPayload, r.LoadBalancing.PrefixHash.PrefixCharLength)
		}
	}

	return nil
}

func getPrefixForCompletionRequest(body map[string]interface{}, n int) string {
	// Example request body:
	// {
	//   "model": "gpt-3.5-turbo-instruct",
	//   "prompt": "Say this is a test",
	//   "max_tokens": 7,
	//   "temperature": 0
	// }
	promptInf, ok := body["prompt"]
	if !ok {
		return ""
	}
	prompt, ok := promptInf.(string)
	if !ok {
		return ""
	}
	return firstNChars(prompt, n)
}

func getPrefixForChatCompletionRequest(body map[string]interface{}, n int) string {
	// Example request body:
	// {
	//   "model": "gpt-4o",
	//   "messages": [
	//     {
	//       "role": "system",
	//       "content": "You are a helpful assistant."
	//     },
	//     {
	//       "role": "user",
	//       "content": "Hello!"
	//     }
	//   ]
	// }
	messagesInf, ok := body["messages"]
	if !ok {
		return ""
	}
	messages, ok := messagesInf.([]interface{})
	if !ok {
		return ""
	}
	if len(messages) == 0 {
		return ""
	}

	// Find the first user request and return the first n characters.
	for _, msgInf := range messages {
		msg, ok := msgInf.(map[string]interface{})
		if !ok {
			return ""
		}
		if msg["role"] == "user" {
			textInf, ok := msg["content"]
			if !ok {
				return ""
			}
			text, ok := textInf.(string)
			if !ok {
				return ""
			}
			return firstNChars(text, n)
		}
	}

	return ""
}

// firstNChars returns the first n characters of a string.
// This function is needed because Go's string indexing is based on bytes, not runes.
func firstNChars(s string, n int) string {
	runes := []rune(s)
	return string(runes[:min(n, len(runes))])
}
