package v1

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
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
			var body CompletionRequest
			require.NoError(t, json.Unmarshal([]byte(c.input), &body))
			require.Equal(t, c.exp, body.Prefix(c.n))
		})
	}
}

func TestChatCompletionRequestPrefix(t *testing.T) {
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
			var body ChatCompletionRequest
			require.NoError(t, json.Unmarshal([]byte(c.input), &body))
			require.Equal(t, c.exp, body.Prefix(c.n))
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
