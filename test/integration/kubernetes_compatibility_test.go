package integration

import (
	"context"
	"testing"
	"time"

	kubeaiv1 "github.com/kubeai-project/kubeai/api/k8s/v1"
	"github.com/kubeai-project/kubeai/internal/k8sutils"
	"github.com/stretchr/testify/require"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Exercise the API contracts used by reconciliation and autoscaling against a
// real API server. Fake clients cannot validate the generated CRD or its CEL.
func TestKubernetesModelLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(testCtx, 30*time.Second)
	defer cancel()
	m := modelForTest(t)
	m.Spec.Replicas = ptr.To[int32](1)
	m.Spec.MaxReplicas = ptr.To[int32](3)
	require.NoError(t, k8sutils.ServerSideApply(ctx, testK8sClient, m, "kubeai-compatibility-test"))

	stored := &kubeaiv1.Model{}
	key := client.ObjectKeyFromObject(m)
	require.NoError(t, testK8sClient.Get(ctx, key, stored))
	require.Equal(t, m.Spec.URL, stored.Spec.URL)
	require.NotEmpty(t, stored.ManagedFields, "server-side apply must record field ownership")

	scale := &autoscalingv1.Scale{}
	require.NoError(t, testK8sClient.SubResource("scale").Get(ctx, stored, scale))
	require.EqualValues(t, 1, scale.Spec.Replicas)
	scale.Spec.Replicas = 2
	require.NoError(t, testK8sClient.SubResource("scale").Update(ctx, stored, client.WithSubResourceBody(scale)))
	require.NoError(t, testK8sClient.Get(ctx, key, stored))
	require.EqualValues(t, 2, *stored.Spec.Replicas)

	stored.Status.Replicas = kubeaiv1.ModelStatusReplicas{All: 2, Ready: 1}
	require.NoError(t, testK8sClient.Status().Update(ctx, stored))
	require.NoError(t, testK8sClient.SubResource("scale").Get(ctx, stored, scale))
	require.EqualValues(t, 2, scale.Status.Replicas)

	// An invalid update must still be rejected by the generated CEL rules.
	stored.Spec.MinReplicas = 4
	require.True(t, apierrors.IsInvalid(testK8sClient.Update(ctx, stored)))
	require.NoError(t, testK8sClient.Get(ctx, key, stored))
	require.EqualValues(t, 2, *stored.Spec.Replicas)
	require.NotEqualValues(t, 4, stored.Spec.MinReplicas)
}
