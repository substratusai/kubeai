package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestModelPriorityClassName tests that priorityClassName is set properly on model pods.
func TestModelPriorityClassName(t *testing.T) {
	initTest(t, baseSysCfg(t))
	priorityClass := &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-priority-class",
		},
	}
	require.NoError(t, testK8sClient.Create(testCtx, priorityClass))

	// Create a Model object with priorityClassName set
	m := modelForTest(t)
	const expectedPriorityClassName = "test-priority-class"
	m.Spec.PriorityClassName = expectedPriorityClassName
	m.Spec.MinReplicas = 1

	// Create the Model in the Kubernetes cluster
	require.NoError(t, testK8sClient.Create(testCtx, m))

	// Check that the pod has the correct priorityClassName
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		podList := &corev1.PodList{}
		if !assert.NoError(t, testK8sClient.List(testCtx, podList, client.InNamespace(testNS), client.MatchingLabels{"model": m.Name})) {
			return
		}
		if !assert.Len(t, podList.Items, 1) {
			return
		}
		pod := &podList.Items[0]

		// Verify the priorityClassName is set on the pod
		assert.Equal(t, expectedPriorityClassName, pod.Spec.PriorityClassName)
	}, 5*time.Second, time.Second/10, "Pod should have the correct priorityClassName")
}
