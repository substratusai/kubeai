package endpoints

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAwaitBestHost(t *testing.T) {
	const myModel = "myModel"

	manager := &Resolver{endpoints: make(map[string]*endpointGroup, 1)}
	manager.getEndpoints(myModel).
		setAddrs(map[string]struct{}{myModel: {}})

	testCases := map[string]struct {
		model   string
		timeout time.Duration
		expErr  error
	}{
		"all good": {
			model:   myModel,
			timeout: time.Millisecond,
		},
		"unknown service - blocks until timeout": {
			model:   "unknownService",
			timeout: time.Millisecond,
			expErr:  context.DeadlineExceeded,
		},
		// not covered: unknown port with multiple ports on entrypoint
	}

	for name, spec := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), spec.timeout)
			defer cancel()

			gotHost, gotFunc, gotErr := manager.AwaitBestAddress(ctx, spec.model)
			if spec.expErr != nil {
				require.ErrorIs(t, spec.expErr, gotErr)
				return
			}
			require.NoError(t, gotErr)
			gotFunc()
			assert.Equal(t, myModel, gotHost)
		})
	}
}
