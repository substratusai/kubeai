package deployments

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReadinessChecker(t *testing.T) {
	tests := map[string]struct {
		bootstrapped bool
		expectError  bool
	}{
		"not_bootstrapped": {
			expectError: true,
		},
		"bootstrapped": {
			bootstrapped: true,
			expectError:  false,
		},
	}
	for name, spec := range tests {
		t.Run(name, func(t *testing.T) {
			mgr := &Manager{
				Client: fake.NewClientBuilder().Build(),
			}
			if spec.bootstrapped {
				require.NoError(t, mgr.Bootstrap(context.TODO()))
			}
			// when
			gotErr := mgr.ReadinessChecker(nil)
			if spec.expectError {
				assert.Error(t, gotErr)
				return
			}
			assert.NoError(t, gotErr)
		})
	}
}
