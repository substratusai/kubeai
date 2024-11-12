package apiutils_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/substratusai/kubeai/internal/apiutils"
)

func TestSplitModelAdapter(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		input                string
		expModel, expAdapter string
	}{
		"empty input": {
			input:      "",
			expModel:   "",
			expAdapter: "",
		},
		"model only": {
			input:    "my-model",
			expModel: "my-model",
		},
		"model and adapter": {
			input:      "my-model/my-adapter",
			expModel:   "my-model",
			expAdapter: "my-adapter",
		},
		"too many slashes": {
			input:      "my-model/my-adapter/extra",
			expModel:   "my-model",
			expAdapter: "my-adapter/extra",
		},
		"trailing slash": {
			input:      "my-model/",
			expModel:   "my-model",
			expAdapter: "",
		},
	}

	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			model, adapter := apiutils.SplitModelAdapter(spec.input)
			require.Equal(t, spec.expModel, model, "model")
			require.Equal(t, spec.expAdapter, adapter, "adapter")
		})
	}

}
