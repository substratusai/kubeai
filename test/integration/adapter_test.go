package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "github.com/substratusai/kubeai/api/v1"
)

func TestAdapters(t *testing.T) {
	sysCfg := baseSysCfg(t)
	initTest(t, sysCfg)
	m := modelForTest(t)
	m.Spec.Adapters = []v1.Adapter{
		{ID: "adapter1", URL: "hf://test-repo/test-adapter"},
		{ID: "adapter2", URL: "hf://test-repo/test-adapter"},
	}
	require.NoError(t, testK8sClient.Create(testCtx, m))

	requireOpenAIModelList(t, []string{modelLabelSelectorForTest(t)}, 3, []string{
		m.Name,
		m.Name + "/adapter1",
		m.Name + "/adapter2",
	}, "Model list should contain the model and its adapters")
}
