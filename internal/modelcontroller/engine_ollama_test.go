// TODO: write tests
package modelcontroller

import (
	"testing"

	"github.com/stretchr/testify/require"
	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
)

func Test_ollamaStartupProbeScript(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		model       kubeaiv1.Model
		modelURL    modelURL
		featuresMap map[kubeaiv1.ModelFeature]struct{}
		want        string
	}{
		"basic-model-no-pvc": {
			model: kubeaiv1.Model{
				Spec: kubeaiv1.ModelSpec{
					Features: []kubeaiv1.ModelFeature{kubeaiv1.ModelFeatureTextGeneration},
				},
			},
			modelURL: modelURL{
				scheme: "ollama",
				ref:    "def",
				name:   "abc",
			},
			want: "/bin/ollama pull def && /bin/ollama cp def abc && /bin/ollama run abc hi",
		},
		"basic-model-with-pvc": {
			model: kubeaiv1.Model{
				Spec: kubeaiv1.ModelSpec{
					Features: []kubeaiv1.ModelFeature{kubeaiv1.ModelFeatureTextGeneration},
				},
			},
			modelURL: modelURL{
				scheme: "pvc",
				ref:    "def",
				name:   "abc",
			},
			want: "/bin/ollama cp def abc && /bin/ollama run abc hi",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := ollamaStartupProbeScript(&c.model, c.modelURL)
			require.Equal(t, c.want, got)
		})
	}
}
