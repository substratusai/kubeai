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

// TestModelScalingBounds tests whether the Model controller respects the
// MinReplicas and MaxReplicas set in the Model spec.
// NOTE: This does not test scale-from-zero or autoscaling code paths!
func TestModelScalingBounds(t *testing.T) {
	initTest(t, baseSysCfg())

	// Construct a Model object with MinReplicas set to 0.
	m := modelForTest(t)
	m.Spec.TargetRequests = ptr.To[int32](1)
	m.Spec.ScaleDownDelaySeconds = ptr.To[int64](999999)
	m.Spec.MinReplicas = 0
	m.Spec.MaxReplicas = ptr.To[int32](2)

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
	updateModel(t, m, func() { m.Spec.MinReplicas = 1 }, "MinReplicas=1")
	requireModelReplicas(t, m, 1, "Replicas should be scaled up to MinReplicas after update", 5*time.Second)

	// Check that a Pod was created for the Model.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		podList := &corev1.PodList{}
		assert.NoError(t, testK8sClient.List(testCtx, podList, client.InNamespace(testNS), client.MatchingLabels{"model": m.Name}))
		assert.Len(t, podList.Items, 1)
	}, 2*time.Second, time.Second/10, "Pod should be created for the Model")

	// Update model replicas to be greater than max replicas.
	// TODO: Future: This should be a validation error enforced by webhook.
	updateModel(t, m, func() { m.Spec.Replicas = ptr.To(*m.Spec.MaxReplicas + 1) }, "Replicas > MaxReplicas")

	// Model should scale down to MaxReplicas.
	requireModelReplicas(t, m, *m.Spec.MaxReplicas, "Replicas should be scaled down to MaxReplicas", 5*time.Second)
}
