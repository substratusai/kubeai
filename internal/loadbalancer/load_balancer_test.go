package loadbalancer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/substratusai/kubeai/api/v1"
	"github.com/substratusai/kubeai/internal/apiutils"
)

func TestAwaitBestHost(t *testing.T) {
	const (
		myModel              = "my-model"
		myAdapter            = "my-adapter"
		myPodWithoutAdapter  = "pod1"
		myPodWithAdapter     = "pod2"
		myAddrWithoutAdapter = "10.0.0.1:8000"
		myAddrWithAdapter    = "10.0.0.2:8000"
	)

	testCases := map[string]struct {
		model     string
		adapter   string
		endpoints map[string]endpoint
		expAddr   string
		expErr    error
	}{
		"model without adapter": {
			model:   myModel,
			expAddr: myAddrWithoutAdapter,
			endpoints: map[string]endpoint{
				myPodWithoutAdapter: {address: myAddrWithoutAdapter},
			},
		},
		"model with adapter": {
			model:   myModel,
			adapter: myAdapter,
			endpoints: map[string]endpoint{
				myPodWithoutAdapter: {
					address: myAddrWithoutAdapter,
				},
				myPodWithAdapter: {
					address: myAddrWithAdapter,
					adapters: map[string]struct{}{
						myAdapter: {},
					}},
			},
			expAddr: myAddrWithAdapter,
		},
		"unknown model blocks until timeout": {
			model: "unknown-model",
			endpoints: map[string]endpoint{
				myPodWithoutAdapter: {address: myAddrWithoutAdapter},
			},
			expErr: context.DeadlineExceeded,
		},
		// not covered: unknown port with multiple ports on entrypoint
	}

	for name, spec := range testCases {
		t.Run(name, func(t *testing.T) {
			manager := &LoadBalancer{
				groups: make(map[string]*group, 1),
			}

			manager.getEndpoints(myModel).reconcileEndpoints(spec.endpoints)

			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
			defer cancel()

			gotAddr, gotFunc, gotErr := manager.AwaitBestAddress(ctx, &apiutils.Request{
				Model:   spec.model,
				Adapter: spec.adapter,
				LoadBalancing: v1.LoadBalancing{
					Strategy: v1.LeastLoadStrategy,
				},
			})
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
