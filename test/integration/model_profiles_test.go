package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestModelProfiles tests that profiles are applied as expected.
func TestModelProfiles(t *testing.T) {
	sysCfg := baseSysCfg(t)
	initTest(t, sysCfg)

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

	var pod *corev1.Pod
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		podList := &corev1.PodList{}
		if !assert.NoError(t, testK8sClient.List(testCtx, podList, client.InNamespace(testNS), client.MatchingLabels{"model": m.Name})) {
			return
		}
		if !assert.Len(t, podList.Items, 1) {
			return
		}
		pod = &podList.Items[0]

		// The Pod should have a single container named "server".
		container := mustFindPodContainerByName(t, pod, "server")
		assert.Equal(t, expectedResources, container.Resources)
		assert.Equal(t, ptr.To(cpuRuntimeClassName), pod.Spec.RuntimeClassName)
		assert.Contains(t, pod.Spec.Tolerations, sysCfg.ResourceProfiles[resourceProfileCPU].Tolerations[0])
		assert.Equal(t, sysCfg.ResourceProfiles[resourceProfileCPU].Affinity, pod.Spec.Affinity)
		assert.Equal(t, sysCfg.ResourceProfiles[resourceProfileCPU].NodeSelector, pod.Spec.NodeSelector)
	}, 5*time.Second, time.Second/10, "Resource profile should be applied to the model Pod object")

	const userImage = "my-repo.com/my-repo/my-image:latest"
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		if !assert.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(m), m)) {
			return
		}
		m.Spec.Image = userImage
		require.NoError(t, testK8sClient.Update(testCtx, m))
	}, 2*time.Second, time.Second/10, "Update model with user specified image")

	require.NoError(t, testK8sClient.Delete(testCtx, pod))
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		podList := &corev1.PodList{}
		if !assert.NoError(t, testK8sClient.List(testCtx, podList, client.InNamespace(testNS), client.MatchingLabels{"model": m.Name})) {
			return
		}
		if !assert.Len(t, podList.Items, 1) {
			return
		}
		pod = &podList.Items[0]

		// The Pod should have a single container named "server".
		container := mustFindPodContainerByName(t, pod, "server")
		assert.Equal(t, userImage, container.Image)
	}, 15*time.Second, time.Second/10, "User specified image should be respected")
}
