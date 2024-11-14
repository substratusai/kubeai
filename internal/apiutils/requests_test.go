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
			input:      "my-model.my-adapter",
			expModel:   "my-model",
			expAdapter: "my-adapter",
		},
		"too many dots": {
			input:      "my-model.my-adapter.extra",
			expModel:   "my-model",
			expAdapter: "my-adapter.extra",
		},
		"trailing dor": {
			input:      "my-model.",
			expModel:   "my-model",
			expAdapter: "",
		},
	}

	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			model, adapter := apiutils.SplitModelAdapter(spec.input)
			require.Equal(t, spec.expModel, model, "model")
			require.Equal(t, spec.expAdapter, adapter, "adapter")
		})
	}
}

func TestMergeModelAdapter(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		model, adapter, exp string
	}{
		"model only": {
			model: "my-model",
			exp:   "my-model",
		},
		"model and adapter": {
			model:   "my-model",
			adapter: "my-adapter",
			exp:     "my-model.my-adapter",
		},
	}

	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			merged := apiutils.MergeModelAdapter(spec.model, spec.adapter)
			require.Equal(t, spec.exp, merged)
		})
	}
}
