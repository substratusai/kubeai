package endpoints

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAwaitBestHost(t *testing.T) {
	const (
		myModel              = "my-model"
		myAdapter            = "my-adapter"
		myAddrWithoutAdapter = "10.0.0.1:8000"
		myAddrWithAdapter    = "10.0.0.2:8000"
	)

	manager := &Resolver{endpoints: make(map[string]*endpointGroup, 1)}
	manager.getEndpoints(myModel).
		setAddrs(map[string]endpointAttrs{
			myAddrWithoutAdapter: {},
			myAddrWithAdapter: {adapters: map[string]struct{}{
				myAdapter: {},
			}},
		})

	testCases := map[string]struct {
		model   string
		adapter string
		expAddr string
		expErr  error
	}{
		"model without adapter": {
			model:   myModel,
			expAddr: myAddrWithoutAdapter,
		},
		"model with adapter": {
			model:   myModel,
			adapter: myAdapter,
			expAddr: myAddrWithAdapter,
		},
		"unknown model blocks until timeout": {
			model:  "unknown-model",
			expErr: context.DeadlineExceeded,
		},
		// not covered: unknown port with multiple ports on entrypoint
	}

	for name, spec := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
			defer cancel()

			gotAddr, gotFunc, gotErr := manager.AwaitBestAddress(ctx, spec.model, spec.adapter)
			if spec.expErr != nil {
				require.ErrorIs(t, spec.expErr, gotErr)
				return
			}
			require.NoError(t, gotErr)
			gotFunc()
			assert.Equal(t, spec.expAddr, gotAddr)
		})
	}
}
