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

// TestModelPodUpdateRollout tests that an update to a Model object triggers the recreation
// of the corresponding Pods.
func TestModelPodUpdateRollout(t *testing.T) {
	// Create a Model object.
	m := modelForTest(t)
	m.Spec.MinReplicas = 3
	m.Spec.MaxReplicas = ptr.To[int32](3)

	require.NoError(t, testK8sClient.Create(testCtx, m))

	// Expect 3 Pods to be created.
	requireModelPods(t, m, 3, "3 Pods should be created", 2*time.Second)

	// Update the Model object.
	const newArg = "--my-new-arg-added-in-testcase"
	updateModel(t, m, func() { m.Spec.Args = []string{newArg} }, "Adding a new arg to the Model")

	// Expect 3 Pods to be created with the new arg.
	require.Eventually(t, func() bool {
		podList := &corev1.PodList{}
		require.NoError(t, testK8sClient.List(testCtx, podList, client.InNamespace(testNS), client.MatchingLabels{"model": m.Name}))
		for _, pod := range podList.Items {
			if !assert.Contains(t, pod.Spec.Containers[0].Args, newArg, "Pod should have the new arg") {
				return false
			}
		}
		require.Equal(t, 3, len(podList.Items), "Exactly 3 Pods should exist")
		return true
	}, time.Second, 100*time.Millisecond, "Exactly 3 Pods should exist (with the new arg)")
}
