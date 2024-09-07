package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/substratusai/kubeai/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestModelScaling(t *testing.T) {
	// Construct a Model object with MinReplicas set to 0.
	m := v1.Model{
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

	// Create the Model object in the Kubernetes cluster.
	require.NoError(t, testK8sClient.Create(testCtx, &m))

	require.Never(t, func() bool {
		// Retrieve the Model object from the Kubernetes cluster.
		require.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(&m), &m))

		var replicas int32
		if m.Spec.Replicas != nil {
			replicas = *m.Spec.Replicas
		}
		// Return true if the Model object has been scaled to 0 replicas.
		return replicas != 0
	}, 2*time.Second, time.Second/10, "Model should not scale up yet")

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Retrieve the Model object from the Kubernetes cluster.
		assert.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(&m), &m))

		// Account for the 3x multiple set in the test Model.
		assert.Equal(t, m.Spec.Resources, &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				"cpu":    resource.MustParse("3"),
				"memory": resource.MustParse("6Gi"),
			},
			Limits: corev1.ResourceList{
				"memory": resource.MustParse("12Gi"),
			},
		})
		assert.Equal(t, sysCfg.ResourceProfiles[resourceProfileCPU].NodeSelector, m.Spec.NodeSelector)
		assert.Equal(t, testVLLMCPUImage, m.Spec.Image)
	}, 2*time.Second, time.Second/10, "Resource profile should be applied to the Model object")
}
