package modelcontroller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseModelURL(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name    string
		input   string
		want    modelURL
		wantErr bool
	}{
		"empty": {
			input:   "",
			wantErr: true,
		},
		"invalid-scheme": {
			input:   "iNv@lid://path/to/model",
			wantErr: true,
		},
		"double-scheme-edge-case": {
			input: "a://path/b://to/model",
			want: modelURL{
				scheme: "a",
				ref:    "path/b://to/model",
			},
		},
		"valid-google-storage": {
			input: "gs://bucket-name/path/to/model",
			want: modelURL{
				scheme: "gs",
				ref:    "bucket-name/path/to/model",
			},
		},
		"valid-ollama": {
			input: "ollama://gemma2:2b",
			want: modelURL{
				scheme: "ollama",
				ref:    "gemma2:2b",
			},
		},
		"valid-huggingface": {
			input: "hf://test-user/model-name",
			want: modelURL{
				scheme: "hf",
				ref:    "test-user/model-name",
			},
		},
		"valid-pvc": {
			input: "pvc://my-vpc/path/to/model",
			want: modelURL{
				scheme: "pvc",
				ref:    "my-vpc/path/to/model",
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := parseModelURL(c.input)
			if c.wantErr {
				require.Error(t, err)
				return
			} else {
				require.NoError(t, err)
			}
			c.want.original = c.input
			require.Equal(t, c.want, got)
		})
	}
}

func Test_parsePVCNamePath(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		ref     string
		pvcName string
		path    string
	}{
		"fully-specified": {
			ref:     "my-pvc/llama",
			pvcName: "my-pvc",
			path:    "llama",
		},
		"root": {
			ref:     "my-pvc/",
			pvcName: "my-pvc",
			path:    "",
		},
		"root-with-additional-slash": {
			ref:     "my-pvc//",
			pvcName: "my-pvc",
			path:    "",
		},
		"no-slash-in-middle": {
			ref:     "my-pvc",
			pvcName: "my-pvc",
			path:    "",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			pvcName, path := parsePVCNamePath(modelURL{ref: c.ref})
			require.Equal(t, c.pvcName, pvcName)
			require.Equal(t, c.path, path)
		})
	}

}
