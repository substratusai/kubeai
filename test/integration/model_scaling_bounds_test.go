package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestModelScalingBounds(t *testing.T) {
	// Construct a Model object with MinReplicas set to 0.
	m := modelForTest(t)

	// Create the Model object in the Kubernetes cluster.
	require.NoError(t, testK8sClient.Create(testCtx, m))

	require.Never(t, func() bool {
		// Retrieve the Model object from the Kubernetes cluster.
		require.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(m), m))

		var replicas int32
		if m.Spec.Replicas != nil {
			replicas = *m.Spec.Replicas
		}
		// Return true if the Model object has been scaled to 0 replicas.
		return replicas != 0
	}, 2*time.Second, time.Second/10, "Model should not scale up yet")

	// Make sure no Pods are created for the Model yet.
	require.Never(t, func() bool {
		podList := &corev1.PodList{}
		require.NoError(t, testK8sClient.List(testCtx, podList, client.InNamespace(testNS), client.MatchingLabels{"model": m.Name}))
		return len(podList.Items) != 0
	}, 2*time.Second, time.Second/10, "No model Pods should be created yet")

	// Update the Model object to set MinReplicas to 1.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Retrieve the Model object from the Kubernetes cluster.
		assert.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(m), m))
		m.Spec.MinReplicas = 1
		assert.NoError(t, testK8sClient.Update(testCtx, m))
	}, 2*time.Second, time.Second/10, "Updating MinReplicas should succeed")

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Retrieve the Model object from the Kubernetes cluster.
		assert.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(m), m))

		assert.NotNil(t, m.Spec.Replicas)
		assert.Equal(t, int32(1), *m.Spec.Replicas)
	}, 2*time.Second, time.Second/10, "Model should scale up to MinReplicas after update")

	// Check that a Pod was created for the Model.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		podList := &corev1.PodList{}
		assert.NoError(t, testK8sClient.List(testCtx, podList, client.InNamespace(testNS), client.MatchingLabels{"model": m.Name}))
		assert.Len(t, podList.Items, 1)
	}, 2*time.Second, time.Second/10, "Pod should be created for the Model")

	// Update model replicas to be greater than max replicas.
	// TODO: Future: This should be a validation error enforced by webhook.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Retrieve the Model object from the Kubernetes cluster.
		assert.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(m), m))
		m.Spec.Replicas = ptr.To(m.Spec.MaxReplicas + 1)
		assert.NoError(t, testK8sClient.Update(testCtx, m))
	}, 2*time.Second, time.Second/10, "Updating replicas should succeed")

	// Model should scale down to MaxReplicas.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Retrieve the Model object from the Kubernetes cluster.
		assert.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(m), m))

		assert.NotNil(t, m.Spec.Replicas)
		assert.Equal(t, m.Spec.MaxReplicas, *m.Spec.Replicas)
	}, 2*time.Second, time.Second/10, "Model should scale down to MaxReplicas")

}
