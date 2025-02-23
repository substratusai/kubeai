// TODO: write tests
package modelcontroller

import (
	"testing"

	"github.com/stretchr/testify/require"
	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
)

func Test_startupProbeScriptConstructer(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		modelName   string
		modelRef    string
		modelScheme string
		featuresMap map[kubeaiv1.ModelFeature]struct{}
		want        string
	}{
		"basic-model-no-pvc": {
			modelName:   "abc",
			modelRef:    "def",
			modelScheme: "hf",
			featuresMap: map[kubeaiv1.ModelFeature]struct{}{
				kubeaiv1.ModelFeatureTextGeneration: {},
			},
			want: "/bin/ollama pull def && /bin/ollama cp def abc && /bin/ollama run abc hi",
		},
		"basic-model-with-pvc": {
			modelName:   "abc",
			modelRef:    "def",
			modelScheme: "pvc",
			featuresMap: map[kubeaiv1.ModelFeature]struct{}{
				kubeaiv1.ModelFeatureTextGeneration: {},
			},
			want: "/bin/ollama cp def abc && /bin/ollama run abc hi",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := startupProbeScriptConstructer(c.modelName, c.modelRef, c.modelScheme, c.featuresMap)
			require.Equal(t, c.want, got)
		})
	}
}
