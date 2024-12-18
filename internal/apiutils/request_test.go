package apiutils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "github.com/substratusai/kubeai/api/v1"
)

func Test_getPrefixForCompletionRequest(t *testing.T) {
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
			var body map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(c.input), &body))
			out, err := getPrefixForCompletionRequest(body, c.n)
			if c.expErrorContains != nil {
				for _, ec := range c.expErrorContains {
					require.ErrorContains(t, err, ec)
				}
				return
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, c.exp, out)
		})
	}
}

func Test_getPrefixForChatCompletionRequest(t *testing.T) {
	cases := []struct {
		input            string
		n                int
		exp              string
		expErrorContains []string
	}{
		{`{}`, 0, "", []string{"missing", "messages"}},
		{`{}`, 0, "", []string{"missing", "messages"}},
		{`{"messages": []}`, 0, "", []string{"empty"}},
		{`{"messages": []}`, 9, "", []string{"empty"}},
		{`{"messages": [{"role": "user", "content": "abc"}]}`, 0, "", nil},
		{`{"messages": [{"role": "user", "content": "abc"}]}`, 9, "abc", nil},
		{`{"messages": [{"role": "user", "content": "abcefghijk"}]}`, 9, "abcefghij", nil},
		{`{"messages": [{"role": "user", "content": "世界"}]}`, 0, "", nil},
		{`{"messages": [{"role": "user", "content": "世界"}]}`, 1, "世", nil},
		{`{"messages": [{"role": "user", "content": "世界"}]}`, 2, "世界", nil},
		{`{"messages": [{"role": "user", "content": "世界"}]}`, 3, "世界", nil},
		{`{"messages": [{"role": "user", "content": "abc"}, {"role": "user", "content": "xyz"}]}`, 0, "", nil},
		{`{"messages": [{"role": "user", "content": "abc"}, {"role": "user", "content": "xyz"}]}`, 9, "abc", nil},
		{`{"messages": [{"role": "system", "content": "abc"}, {"role": "user", "content": "xyz"}]}`, 0, "", nil},
		{`{"messages": [{"role": "system", "content": "abc"}, {"role": "user", "content": "xyz"}]}`, 9, "xyz", nil},
		{`{"messages": [{"role": "system", "content": "abc"}]}`, 9, "", []string{"no", "user", "found"}},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%q %d", c.input, c.n), func(t *testing.T) {
			var body map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(c.input), &body))
			out, err := getPrefixForChatCompletionRequest(body, c.n)
			if c.expErrorContains != nil {
				for _, ec := range c.expErrorContains {
					require.ErrorContains(t, err, ec)
				}
				return
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, c.exp, out)
		})
	}
}

func Test_firstNChars(t *testing.T) {
	cases := []struct {
		input string
		n     int
		exp   string
	}{
		{"", 0, ""},
		{"", 1, ""},
		{"abc", 0, ""},
		{"abc", 1, "a"},
		{"abc", 2, "ab"},
		{"abc", 3, "abc"},
		{"abc", 4, "abc"},
		{"世界", 1, "世"},
		{"世界", 2, "世界"},
		{"世界", 3, "世界"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%q %d", c.input, c.n), func(t *testing.T) {
			require.Equal(t, c.exp, firstNChars(c.input, c.n))
		})
	}
}

func TestParseRequest(t *testing.T) {
	cases := []struct {
		name             string
		body             string
		path             string
		headers          http.Header
		expModel         string
		expAdapter       string
		expPrefix        string
		expErrorContains []string
	}{
		{
			name:             "empty",
			body:             `{}`,
			expErrorContains: []string{"bad request"},
		},
		{
			name:     "model only",
			body:     `{"model": "test-model"}`,
			expModel: "test-model",
		},
		{
			name:       "model and adapter",
			body:       `{"model": "test-model_test-adapter"}`,
			expModel:   "test-model",
			expAdapter: "test-adapter",
		},
		{
			name:             "openai chat completion missing messages",
			body:             `{"model": "test-model"}`,
			path:             "/v1/chat/completions",
			expModel:         "test-model",
			expErrorContains: []string{"missing", "messages"},
		},
		{
			name:             "openai chat completion missing user message",
			body:             `{"model": "test-model", "messages": [{"role": "system", "content": "test"}]}`,
			path:             "/v1/chat/completions",
			expModel:         "test-model",
			expErrorContains: []string{"no", "user", "found"},
		},
		{
			name:      "openai chat completion",
			body:      `{"model": "test-model", "messages": [{"role": "user", "content": "test-prefix"}]}`,
			path:      "/v1/chat/completions",
			expModel:  "test-model",
			expPrefix: "test-prefi", // "test-prefix" (max 10) --> "test-prefi"
		},
		{
			name:      "openai legacy completion",
			body:      `{"model": "test-model", "prompt": "test-prefix"}`,
			path:      "/v1/completions",
			expModel:  "test-model",
			expPrefix: "test-prefi", // "test-prefix" (max 10) --> "test-prefi"

		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()

			mockClient := &mockModelClient{prefixCharLen: 10}

			req, err := ParseRequest(ctx, mockClient, bytes.NewReader([]byte(c.body)), c.path, c.headers)
			if c.expErrorContains != nil {
				for _, ec := range c.expErrorContains {
					require.ErrorContains(t, err, ec)
				}
				return
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, c.expModel, req.Model)
			require.Equal(t, c.expAdapter, req.Adapter)
			require.Equal(t, c.expPrefix, req.Prefix)
		})
	}

}

type mockModelClient struct {
	prefixCharLen int
}

func (m *mockModelClient) LookupModel(ctx context.Context, model, adapter string, selectors []string) (*v1.Model, error) {
	return &v1.Model{
		Spec: v1.ModelSpec{
			LoadBalancing: v1.LoadBalancing{
				Strategy: v1.PrefixHashStrategy,
				PrefixHash: v1.PrefixHash{
					// "test-prefix" --> "test-prefi"
					PrefixCharLength: m.prefixCharLen,
				},
			},
		},
	}, nil
}
