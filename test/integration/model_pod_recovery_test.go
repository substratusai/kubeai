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

// TestModelPodRecovery tests that if a Pod is deleted, it is recreated
// with the same name.
func TestModelPodRecovery(t *testing.T) {
	// Create a Model object.
	m := modelForTest(t)
	m.Spec.MinReplicas = 3
	m.Spec.MaxReplicas = ptr.To[int32](3)

	require.NoError(t, testK8sClient.Create(testCtx, m))

	// Expect 3 Pods to be created.
	requireModelPods(t, m, 3, "3 Pods should be created", 2*time.Second)

	// Delete pod-1.
	pod1 := &corev1.Pod{}
	require.NoError(t, testK8sClient.Get(testCtx, client.ObjectKey{Namespace: testNS, Name: "model-" + m.Name + "-1"}, pod1))
	require.NoError(t, testK8sClient.Delete(testCtx, pod1))
	origUID := pod1.UID

	require.Eventually(t, func() bool {
		newPod1 := &corev1.Pod{}
		if !assert.NoError(t, testK8sClient.Get(testCtx, client.ObjectKey{Namespace: testNS, Name: "model-" + m.Name + "-1"}, newPod1)) {
			return false
		}
		return newPod1.UID != origUID
	}, time.Second, 100*time.Millisecond, "Pod-1 should be recreated")

	requireModelPods(t, m, 3, "3 Pods should be exist again", 2*time.Second)
}
