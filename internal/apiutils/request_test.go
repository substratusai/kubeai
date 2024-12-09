package apiutils

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_getPrefixForCompletionRequest(t *testing.T) {
	cases := []struct {
		input string
		n     int
		exp   string
	}{
		{`{}`, 0, ""},
		{`{}`, 9, ""},
		{`{"prompt": "abc"}`, 0, ""},
		{`{"prompt": "abc"}`, 9, "abc"},
		{`{"prompt": "abcefghijk"}`, 9, "abcefghij"},
		{`{"prompt": "世界"}`, 0, ""},
		{`{"prompt": "世界"}`, 1, "世"},
		{`{"prompt": "世界"}`, 2, "世界"},
		{`{"prompt": "世界"}`, 3, "世界"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%q %d", c.input, c.n), func(t *testing.T) {
			var body map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(c.input), &body))
			require.Equal(t, c.exp, getPrefixForCompletionRequest(body, c.n))
		})
	}
}

func Test_getPrefixForChatCompletionRequest(t *testing.T) {
	cases := []struct {
		input string
		n     int
		exp   string
	}{
		{`{}`, 0, ""},
		{`{}`, 9, ""},
		{`{"messages": []}`, 0, ""},
		{`{"messages": []}`, 9, ""},
		{`{"messages": [{"role": "user", "content": "abc"}]}`, 0, ""},
		{`{"messages": [{"role": "user", "content": "abc"}]}`, 9, "abc"},
		{`{"messages": [{"role": "user", "content": "abcefghijk"}]}`, 9, "abcefghij"},
		{`{"messages": [{"role": "user", "content": "世界"}]}`, 0, ""},
		{`{"messages": [{"role": "user", "content": "世界"}]}`, 1, "世"},
		{`{"messages": [{"role": "user", "content": "世界"}]}`, 2, "世界"},
		{`{"messages": [{"role": "user", "content": "世界"}]}`, 3, "世界"},
		{`{"messages": [{"role": "user", "content": "abc"}, {"role": "user", "content": "xyz"}]}`, 0, ""},
		{`{"messages": [{"role": "user", "content": "abc"}, {"role": "user", "content": "xyz"}]}`, 9, "abc"},
		{`{"messages": [{"role": "system", "content": "abc"}, {"role": "user", "content": "xyz"}]}`, 0, ""},
		{`{"messages": [{"role": "system", "content": "abc"}, {"role": "user", "content": "xyz"}]}`, 9, "xyz"},
		{`{"messages": [{"role": "system", "content": "abc"}]}`, 9, ""},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%q %d", c.input, c.n), func(t *testing.T) {
			var body map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(c.input), &body))
			require.Equal(t, c.exp, getPrefixForChatCompletionRequest(body, c.n))
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
