package v1_test

import (
	"encoding/json"
	"testing"

	easyjson "github.com/mailru/easyjson"
	"github.com/stretchr/testify/require"
	v1 "github.com/substratusai/kubeai/api/openai/v1"
)

func TestEmbeddingRequest_JSON(t *testing.T) {
	cases := []struct {
		name string
		json string
		req  *v1.EmbeddingRequest
	}{
		{
			name: "empty request",
			json: `{"model":"","input":null}`,
			req:  &v1.EmbeddingRequest{},
		},
		{
			name: "minimal",
			json: `{"model":"test","input": "hey"}`,
			req:  &v1.EmbeddingRequest{Model: "test", Input: "hey"},
		},
		{
			name: "extra field",
			json: `{"model":"","input":null,"eexxttrraa":"val"}`,
			req:  &v1.EmbeddingRequest{},
		},
		{
			name: "string input",
			json: `{"model":"text-embedding-ada-002","input":"This is a test string","extra":"val"}`,
			req: &v1.EmbeddingRequest{
				Model: "text-embedding-ada-002",
				Input: "This is a test string",
			},
		},
		{
			name: "array input",
			json: `{"model":"text-embedding-ada-002","input":["This is a test string","And another string"],"extra":"val"}`,
			req: &v1.EmbeddingRequest{
				Model: "text-embedding-ada-002",
				Input: []interface{}{"This is a test string", "And another string"},
			},
		},
		{
			name: "with encoding format float",
			json: `{"model":"text-embedding-ada-002","input":"Test","encoding_format":"float","extra":"val"}`,
			req: &v1.EmbeddingRequest{
				Model:          "text-embedding-ada-002",
				Input:          "Test",
				EncodingFormat: v1.EmbeddingEncodingFormatFloat,
			},
		},
		{
			name: "with encoding format base64",
			json: `{"model":"text-embedding-ada-002","input":"Test","encoding_format":"base64","extra":"val"}`,
			req: &v1.EmbeddingRequest{
				Model:          "text-embedding-ada-002",
				Input:          "Test",
				EncodingFormat: v1.EmbeddingEncodingFormatBase64,
			},
		},
		{
			name: "all fields set",
			json: `{
			"model": "text-embedding-3-large",
			"input": "The food was delicious and the waiter...",
			"encoding_format": "float",
			"user": "user123",
			"dimensions": 256
		}`,
			req: &v1.EmbeddingRequest{
				Model:          "text-embedding-3-large",
				Input:          "The food was delicious and the waiter...",
				EncodingFormat: v1.EmbeddingEncodingFormatFloat,
				User:           "user123",
				Dimensions:     256,
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.True(t, json.Valid([]byte(c.json)), "test case should be valid json")

			var req v1.EmbeddingRequest
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

func TestEmbeddingResponse_JSON(t *testing.T) {
	cases := []struct {
		name string
		json string
		resp *v1.EmbeddingResponse
	}{
		{
			name: "empty response",
			json: `{"object":"","data":null,"model":""}`,
			resp: &v1.EmbeddingResponse{},
		},
		{
			name: "single embedding",
			json: `{
			"object": "list",
			"data": [
				{
					"object": "embedding",
					"embedding": [0.1, 0.2, 0.3],
					"index": 0
				}
			],
			"model": "text-embedding-ada-002",
			"usage": {
				"prompt_tokens": 10,
				"total_tokens": 20,
				"completion_tokens": 10
			}
		}`,
			resp: &v1.EmbeddingResponse{
				Object: "list",
				Data: []v1.Embedding{
					{
						Object:    "embedding",
						Embedding: []float32{0.1, 0.2, 0.3},
						Index:     0,
					},
				},
				Model: "text-embedding-ada-002",
				Usage: &v1.Usage{
					PromptTokens:     10,
					TotalTokens:      20,
					CompletionTokens: 10,
				},
			},
		},
		{
			name: "multiple embeddings",
			json: `{
			"object": "list",
			"data": [
				{
					"object": "embedding",
					"embedding": [0.1, 0.2, 0.3],
					"index": 0
				},
				{
					"object": "embedding",
					"embedding": [0.4, 0.5, 0.6],
					"index": 1
				}
			],
			"model": "text-embedding-ada-002"
		}`,
			resp: &v1.EmbeddingResponse{
				Object: "list",
				Data: []v1.Embedding{
					{
						Object:    "embedding",
						Embedding: []float32{0.1, 0.2, 0.3},
						Index:     0,
					},
					{
						Object:    "embedding",
						Embedding: []float32{0.4, 0.5, 0.6},
						Index:     1,
					},
				},
				Model: "text-embedding-ada-002",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.True(t, json.Valid([]byte(c.json)), "test case should be valid json")

			var resp v1.EmbeddingResponse
			err := easyjson.Unmarshal([]byte(c.json), &resp)
			require.NoError(t, err, "unmarshal error")

			if c.resp != nil {
				// Set aside the proxy to check equality without unknown fields
				proxy := resp.UnknownFieldsProxy
				resp.UnknownFieldsProxy = easyjson.UnknownFieldsProxy{}
				require.EqualValues(t, *c.resp, resp, "expected struct values")
				// Restore the proxy
				resp.UnknownFieldsProxy = proxy
			}

			jsn, err := easyjson.Marshal(resp)
			require.NoError(t, err, "marshal error")
			require.JSONEq(t, c.json, string(jsn), "expected round-trip JSON")
		})
	}
}
