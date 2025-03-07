package v1

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

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
