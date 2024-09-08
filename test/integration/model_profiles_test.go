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

// TestModelProfiles tests that resource profiles are applied as expected.
func TestModelProfiles(t *testing.T) {
	// Construct a Model object with MinReplicas set to 0.
	m := modelForTest(t)
	// Make sure there is a Pod created to run assertions against.
	m.Spec.MinReplicas = 1
	// Create the Model object in the Kubernetes cluster.
	require.NoError(t, testK8sClient.Create(testCtx, m))

	expectedResources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("3"),
			"memory": resource.MustParse("6Gi"),
		},
		Limits: corev1.ResourceList{
			"memory": resource.MustParse("12Gi"),
		},
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Retrieve the Model object from the Kubernetes cluster.
		assert.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(m), m))

		// Account for the 3x multiple set in the test Model.
		assert.Equal(t, &expectedResources, m.Spec.Resources)
		assert.Equal(t, sysCfg().ResourceProfiles[resourceProfileCPU].NodeSelector, m.Spec.NodeSelector)
		assert.Equal(t, testVLLMCPUImage, m.Spec.Image)
	}, 2*time.Second, time.Second/10, "Resource profile should be applied to the Model object")

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		podList := &corev1.PodList{}
		assert.NoError(t, testK8sClient.List(testCtx, podList, client.InNamespace(testNS), client.MatchingLabels{"model": m.Name}))
		assert.Len(t, podList.Items, 1)
		pod := &podList.Items[0]

		// The Pod should have a single container named "server".
		container := mustFindPodContainerByName(t, pod, "server")
		assert.Equal(t, expectedResources, container.Resources)
		assert.Equal(t, sysCfg().ResourceProfiles[resourceProfileCPU].NodeSelector, pod.Spec.NodeSelector)
	}, 2*time.Second, time.Second/10, "Resource profile should be applied to the model Pod object")
}
