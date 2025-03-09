package v1_test

import (
	"encoding/json"
	"fmt"
	"testing"

	easyjson "github.com/mailru/easyjson"
	"github.com/stretchr/testify/require"
	v1 "github.com/substratusai/kubeai/api/openai/v1"
)

func TestCompletionRequestPrefix(t *testing.T) {
	cases := []struct {
		input            string
		n                int
		exp              string
		expErrorContains []string
	}{
		{`{}`, 0, "", []string{"missing", "prompt"}},
		{`{}`, 9, "", []string{"missing", "prompt"}},
		{`{"prompt": "abc"}`, 0, "", nil},
		{`{"prompt": "abc"}`, 9, "abc", nil},
		{`{"prompt": "abcefghijk"}`, 9, "abcefghij", nil},
		{`{"prompt": "世界"}`, 0, "", nil},
		{`{"prompt": "世界"}`, 1, "世", nil},
		{`{"prompt": "世界"}`, 2, "世界", nil},
		{`{"prompt": "世界"}`, 3, "世界", nil},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%q %d", c.input, c.n), func(t *testing.T) {
			var req v1.CompletionRequest
			require.NoError(t, easyjson.Unmarshal([]byte(c.input), &req))
			require.Equal(t, c.exp, req.Prefix(c.n))
		})
	}
}

func TestCompletionRequest_JSON(t *testing.T) {
	cases := []struct {
		name string
		json string
		req  *v1.CompletionRequest
	}{
		{
			name: "empty request",
			json: `{"model":"", "prompt": null}`,
			req:  &v1.CompletionRequest{},
		},
		{
			name: "extra field",
			json: `{"model":"", "prompt": null, "eexxttrraa":"val"}`,
			req:  &v1.CompletionRequest{},
		},
		{
			name: "string prompt",
			json: `{"model":"test-model","prompt":"test-prompt","extra":"val"}`,
			req: &v1.CompletionRequest{
				Model:  "test-model",
				Prompt: "test-prompt",
			},
		},
		{
			name: "array prompt",
			json: `{"model":"test-model","prompt":["test-prompt1","test-prompt2"],"extra":"val"}`,
			req: &v1.CompletionRequest{
				Model:  "test-model",
				Prompt: []interface{}{"test-prompt1", "test-prompt2"},
			},
		},
		{
			name: "all fields set",
			json: `{
				"model": "text-davinci-003",
				"prompt": "Write a story about a robot.",
				"max_tokens": 500,
				"temperature": 0.7,
				"top_p": 0.95,
				"n": 1,
				"stream": true,
				"stop": ["END", "STOP"],
				"presence_penalty": 0.5,
				"frequency_penalty": 0.8,
				"logit_bias": {"50256": -100},
				"seed": 42,
				"echo": true,
				"best_of": 3,
				"logprobs": 5,
				"suffix": "The End.",
				"user": "user123",
				"store": true,
				"metadata": {
					"user_id": "abc123",
					"session_id": "sess_456"
				}
			}`,
			req: &v1.CompletionRequest{
				Model:            "text-davinci-003",
				Prompt:           "Write a story about a robot.",
				MaxTokens:        500,
				Temperature:      v1.Ptr[float32](0.7),
				TopP:             v1.Ptr[float32](0.95),
				N:                v1.Ptr(1),
				Stream:           true,
				Stop:             []string{"END", "STOP"},
				PresencePenalty:  v1.Ptr[float32](0.5),
				FrequencyPenalty: v1.Ptr[float32](0.8),
				LogitBias:        map[string]int{"50256": -100},
				Seed:             func() *int { i := 42; return &i }(),
				Echo:             true,
				BestOf:           v1.Ptr(3),
				LogProbs:         v1.Ptr(5),
				Suffix:           "The End.",
				User:             "user123",
				Store:            true,
				Metadata: map[string]string{
					"user_id":    "abc123",
					"session_id": "sess_456",
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.True(t, json.Valid([]byte(c.json)), "test case should be valid json")

			var req v1.CompletionRequest
			err := easyjson.Unmarshal([]byte(c.json), &req)
			require.NoError(t, err, "unmarshal error")

			if c.req != nil {
				proxy := req.UnknownFieldsProxy
				req.UnknownFieldsProxy = easyjson.UnknownFieldsProxy{}
				// Assert on equality without the unknown fields.
				require.EqualValues(t, *c.req, req, "expected struct values")
				req.UnknownFieldsProxy = proxy
			}

			jsn, err := easyjson.Marshal(req)
			require.NoError(t, err, "marshal error")
			require.JSONEq(t, c.json, string(jsn), "expected round-trip JSON")
		})
	}
}

func TestCompletionResponse_JSON(t *testing.T) {
	cases := []struct {
		name string
		json string
		resp *v1.CompletionResponse
	}{
		{
			name: "empty response",
			json: `{"object":"","model":"","choices":null}`,
			resp: &v1.CompletionResponse{},
		},
		{
			name: "basic response",
			json: `{
				"id": "cmpl-123",
				"object": "text_completion",
				"created": 1677858242,
				"model": "text-davinci-003",
				"choices": [
					{
						"text": "This is a test response.",
						"index": 0,
						"finish_reason": "stop"
					}
				]
			}`,
			resp: &v1.CompletionResponse{
				ID:      "cmpl-123",
				Object:  "text_completion",
				Created: 1677858242,
				Model:   "text-davinci-003",
				Choices: []v1.CompletionChoice{
					{
						Text:         "This is a test response.",
						Index:        0,
						FinishReason: "stop",
					},
				},
			},
		},
		{
			name: "multiple choices",
			json: `{
				"id": "cmpl-456",
				"object": "text_completion",
				"created": 1689324671,
				"model": "text-davinci-003",
				"choices": [
					{
						"text": "First completion option.",
						"index": 0,
						"finish_reason": "stop"
					},
					{
						"text": "Second completion option.",
						"index": 1,
						"finish_reason": "length"
					}
				],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 15,
					"total_tokens": 25
				}
			}`,
			resp: &v1.CompletionResponse{
				ID:      "cmpl-456",
				Object:  "text_completion",
				Created: 1689324671,
				Model:   "text-davinci-003",
				Choices: []v1.CompletionChoice{
					{
						Text:         "First completion option.",
						Index:        0,
						FinishReason: "stop",
					},
					{
						Text:         "Second completion option.",
						Index:        1,
						FinishReason: "length",
					},
				},
				Usage: &v1.Usage{
					PromptTokens:     10,
					CompletionTokens: 15,
					TotalTokens:      25,
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.True(t, json.Valid([]byte(c.json)), "test case should be valid json")

			var resp v1.CompletionResponse
			err := easyjson.Unmarshal([]byte(c.json), &resp)
			require.NoError(t, err, "unmarshal error")

			if c.resp != nil {
				// Set aside the proxy to check equality without unknown fields
				proxy := resp.UnknownFieldsProxy
				resp.UnknownFieldsProxy = easyjson.UnknownFieldsProxy{}
				// Assert on equality without the unknown fields.
				require.EqualValues(t, *c.resp, resp, "expected struct values")
				resp.UnknownFieldsProxy = proxy
			}

			jsn, err := easyjson.Marshal(resp)
			require.NoError(t, err, "marshal error")
			require.JSONEq(t, c.json, string(jsn), "expected round-trip JSON")
		})
	}
}
