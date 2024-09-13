package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/substratusai/kubeai/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestModelDefaultProfiles tests that default profiles are applied as expected.
func TestModelDefaultProfiles(t *testing.T) {
	// Construct a Model object with MinReplicas set to 0.
	m := modelForTest(t)
	m.Spec.AutoscalingProfile = ""
	require.NoError(t, testK8sClient.Create(testCtx, m))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(m), m))
		assert.Equal(t, "default", m.Spec.AutoscalingProfile)
		if assert.NotNil(t, m.Spec.Autoscaling) {
			assert.Equal(t, sysCfg().Autoscaling.Profiles["default"], *m.Spec.Autoscaling)
		}
	}, 5*time.Second, time.Second/10, "Default autoscaling profile should be applied to the Model object")
}

// TestModelProfiles tests that profiles are applied as expected.
func TestModelProfiles(t *testing.T) {
	// Construct a Model object with MinReplicas set to 0.
	m := modelForTest(t)
	// Make sure there is a Pod created to run assertions against.
	m.Spec.AutoscalingProfile = "online"
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
		assert.Equal(t, sysCfg().ResourceProfiles[resourceProfileCPU].Tolerations, m.Spec.Tolerations)
		assert.Equal(t, sysCfg().ResourceProfiles[resourceProfileCPU].Affinity, m.Spec.Affinity)
		assert.Equal(t, sysCfg().ResourceProfiles[resourceProfileCPU].NodeSelector, m.Spec.NodeSelector)
		assert.Equal(t, testVLLMCPUImage, m.Spec.Image)
		if assert.NotNil(t, m.Spec.Autoscaling) {
			assert.Equal(t, sysCfg().Autoscaling.Profiles["online"], *m.Spec.Autoscaling)
		}
	}, 2*time.Second, time.Second/10, "Resource profile should be applied to the Model object")

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		podList := &corev1.PodList{}
		assert.NoError(t, testK8sClient.List(testCtx, podList, client.InNamespace(testNS), client.MatchingLabels{"model": m.Name}))
		assert.Len(t, podList.Items, 1)
		pod := &podList.Items[0]

		// The Pod should have a single container named "server".
		container := mustFindPodContainerByName(t, pod, "server")
		assert.Equal(t, expectedResources, container.Resources)
		assert.Contains(t, pod.Spec.Tolerations, sysCfg().ResourceProfiles[resourceProfileCPU].Tolerations[0])
		assert.Equal(t, sysCfg().ResourceProfiles[resourceProfileCPU].Affinity, pod.Spec.Affinity)
		assert.Equal(t, sysCfg().ResourceProfiles[resourceProfileCPU].NodeSelector, pod.Spec.NodeSelector)
	}, 2*time.Second, time.Second/10, "Resource profile should be applied to the model Pod object")
}

// TestModelProfileWithUserSetValues tests that user-set values are not overridden by profiles.
func TestModelProfileWithUserSetValues(t *testing.T) {
	// Construct a Model object with MinReplicas set to 0.
	m := modelForTest(t)
	// Make sure there is a Pod created to run assertions against.
	m.Spec.AutoscalingProfile = "online"

	userSetResources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("1"),
			"memory": resource.MustParse("2Gi"),
		},
		Limits: corev1.ResourceList{
			"memory": resource.MustParse("3Gi"),
		},
	}
	m.Spec.Resources = &userSetResources

	userSetNodeSelector := map[string]string{
		"foo": "bar",
	}
	m.Spec.NodeSelector = userSetNodeSelector

	userSetTolerations := []corev1.Toleration{
		{
			Key:      "foo",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}
	m.Spec.Tolerations = userSetTolerations

	userSetAffinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "foo-user",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"bar-user"},
							},
						},
					},
				},
			},
		},
	}
	m.Spec.Affinity = userSetAffinity

	const userSetImage = "users-repo.com/users-image:latest"
	m.Spec.Image = userSetImage

	m = m.DeepCopyObject().(*v1.Model)

	// Create the Model object in the Kubernetes cluster.
	require.NoError(t, testK8sClient.Create(testCtx, m))

	// Ensure that user-set values are not overridden.
	for i := 0; i < 30; i++ {
		// Retrieve the Model object from the Kubernetes cluster.
		require.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(m), m))

		require.Equal(t, &userSetResources, m.Spec.Resources)
		require.Equal(t, userSetNodeSelector, m.Spec.NodeSelector)
		require.Equal(t, userSetImage, m.Spec.Image)
		require.Equal(t, userSetTolerations, m.Spec.Tolerations)
		require.Equal(t, userSetAffinity, m.Spec.Affinity)

		time.Sleep(time.Second / 10)
	}
}
