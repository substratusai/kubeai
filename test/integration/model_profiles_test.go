package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestModelProfiles(t *testing.T) {
	// Construct a Model object with MinReplicas set to 0.
	m := modelForTest(t)

	// Create the Model object in the Kubernetes cluster.
	require.NoError(t, testK8sClient.Create(testCtx, m))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Retrieve the Model object from the Kubernetes cluster.
		assert.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(m), m))

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
		assert.Equal(t, sysCfg().ResourceProfiles[resourceProfileCPU].NodeSelector, m.Spec.NodeSelector)
		assert.Equal(t, testVLLMCPUImage, m.Spec.Image)
	}, 2*time.Second, time.Second/10, "Resource profile should be applied to the Model object")
}
