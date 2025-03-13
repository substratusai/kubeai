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
				name:   "path",
				path:   "b://to/model",
			},
		},
		"valid-google-storage": {
			input: "gs://bucket-name/path/to/model",
			want: modelURL{
				scheme: "gs",
				ref:    "bucket-name/path/to/model",
				name:   "bucket-name",
				path:   "path/to/model",
			},
		},
		"valid-ollama": {
			input: "ollama://gemma2:2b",
			want: modelURL{
				scheme: "ollama",
				ref:    "gemma2:2b",
				name:   "gemma2:2b",
				path:   "",
			},
		},
		"valid-huggingface": {
			input: "hf://test-user/model-name",
			want: modelURL{
				scheme: "hf",
				ref:    "test-user/model-name",
				name:   "test-user",
				path:   "model-name",
			},
		},
		"valid-pvc": {
			input: "pvc://my-vpc/path/to/model",
			want: modelURL{
				scheme: "pvc",
				ref:    "my-vpc/path/to/model",
				name:   "my-vpc",
				path:   "path/to/model",
			},
		},
		"valid-pvc-no-path": {
			input: "pvc://my-vpc",
			want: modelURL{
				scheme: "pvc",
				ref:    "my-vpc",
				name:   "my-vpc",
				path:   "",
			},
		},
		"valid-pvc-with-slash-empty": {
			input: "pvc://my-vpc/",
			want: modelURL{
				scheme: "pvc",
				ref:    "my-vpc/",
				name:   "my-vpc",
				path:   "",
			},
		},
		"valid-pvc-with-double-slash": {
			input: "pvc://my-vpc//",
			want: modelURL{
				scheme: "pvc",
				ref:    "my-vpc//",
				name:   "my-vpc",
				path:   "/",
			},
		},
		"valid-pvc-with-modelname": {
			input: "pvc://my-vpc?model=qwen2:0.5b",
			want: modelURL{
				scheme:     "pvc",
				ref:        "my-vpc",
				name:       "my-vpc",
				path:       "",
				modelParam: "qwen2:0.5b",
			},
		},
		"valid-pvc-withpath-and-modelname": {
			input: "pvc://my-vpc/path/to/model?model=qwen2:0.5b",
			want: modelURL{
				scheme:     "pvc",
				ref:        "my-vpc/path/to/model",
				name:       "my-vpc",
				path:       "path/to/model",
				modelParam: "qwen2:0.5b",
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
