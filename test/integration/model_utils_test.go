package integration

import (
	"strings"
	"testing"

	v1 "github.com/substratusai/kubeai/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func modelForTest(t *testing.T) *v1.Model {
	return &v1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.ToLower(t.Name()),
			Namespace: testNS,
		},
		Spec: v1.ModelSpec{
			Owner:           "test",
			URL:             "hf://test-org/test-model",
			Features:        []v1.ModelFeature{v1.ModelFeatureTextGeneration},
			Engine:          v1.VLLMEngine,
			ResourceProfile: resourceProfileCPU + ":3",
			MinReplicas:     0,
			MaxReplicas:     3,
			Args:            []string{"--test-arg"},
			Env:             map[string]string{"TEST_ENV": "test"},
		},
	}
}
