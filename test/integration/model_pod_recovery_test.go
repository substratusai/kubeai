package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestModelPodRecovery tests that if a Pod is deleted, it is recreated
// with the same name.
func TestModelPodRecovery(t *testing.T) {
	initTest(t, baseSysCfg(t))

	// Create a Model object.
	m := modelForTest(t)
	m.Spec.MinReplicas = 3
	m.Spec.MaxReplicas = ptr.To[int32](3)

	require.NoError(t, testK8sClient.Create(testCtx, m))

	// Expect 3 Pods to be created.
	requireModelPods(t, m, 3, "3 Pods should be created", 5*time.Second)

	podList := &corev1.PodList{}
	require.NoError(t, testK8sClient.List(testCtx, podList, client.InNamespace(testNS), client.MatchingLabels{"model": m.Name}))

	// Delete pod[1].
	pod1 := &corev1.Pod{}
	require.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(&podList.Items[1]), pod1))
	require.NoError(t, testK8sClient.Delete(testCtx, pod1))

	// NOTE: Wait time needs to be long enough to take into account
	// the rate limiting the Model Reconciler has for Pod updates.
	requireModelPods(t, m, 3, "3 Pods should be exist again", 5*time.Second)
}
