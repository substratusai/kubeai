package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestModelDefaults tests that defaults are applied as expected.
func TestModelDefaults(t *testing.T) {
	// Construct a Model object with MinReplicas set to 0.
	m := modelForTest(t)
	require.NoError(t, testK8sClient.Create(testCtx, m))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(m), m))
		if assert.NotNil(t, m.Spec.TargetRequests) {
			assert.Equal(t, int32(100), *m.Spec.TargetRequests)
		}
		if assert.NotNil(t, m.Spec.ScaleDownDelaySeconds) {
			assert.Equal(t, int64(30), *m.Spec.ScaleDownDelaySeconds)
		}
	}, 5*time.Second, time.Second/10, "Default autoscaling profile should be applied to the Model object")
}
