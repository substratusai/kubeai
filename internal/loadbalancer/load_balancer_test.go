package loadbalancer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/substratusai/kubeai/api/v1"
	"github.com/substratusai/kubeai/internal/apiutils"
	"github.com/substratusai/kubeai/internal/metrics/metricstest"
)

func TestAwaitBestHostBehavior(t *testing.T) {
	const (
		myModel              = "my-model"
		myAdapter            = "my-adapter"
		myPodWithoutAdapter  = "pod1"
		myPodWithAdapter     = "pod2"
		myAddrWithoutAdapter = "10.0.0.1:8000"
		myAddrWithAdapter    = "10.0.0.2:8000"
	)

	testCases := map[string]struct {
		model      string
		adapter    string
		endpoints  map[string]endpoint
		strategies []v1.LoadBalancingStrategy
		expAddr    string
		expErr     error
	}{
		"model only": {
			model: myModel,
			strategies: []v1.LoadBalancingStrategy{
				v1.LeastLoadStrategy,
				v1.PrefixHashStrategy,
			},
			expAddr: myAddrWithoutAdapter,
			endpoints: map[string]endpoint{
				myPodWithoutAdapter: {address: myAddrWithoutAdapter},
			},
		},
		"model and adapter": {
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
			strategies: []v1.LoadBalancingStrategy{
				v1.LeastLoadStrategy,
				v1.PrefixHashStrategy,
			},
			expAddr: myAddrWithAdapter,
		},
		"no matching model blocks until timeout": {
			model: "unknown-model",
			endpoints: map[string]endpoint{
				myPodWithoutAdapter: {address: myAddrWithoutAdapter},
			},
			strategies: []v1.LoadBalancingStrategy{
				v1.LeastLoadStrategy,
				v1.PrefixHashStrategy,
			},
			expErr: context.DeadlineExceeded,
		},
		"no matching adapter blocks until timeout": {
			model:   myModel,
			adapter: "unknown-adapter",
			endpoints: map[string]endpoint{
				myPodWithoutAdapter: {address: myAddrWithoutAdapter},
			},
			strategies: []v1.LoadBalancingStrategy{
				v1.LeastLoadStrategy,
				v1.PrefixHashStrategy,
			},
			expErr: context.DeadlineExceeded,
		},
		// not covered: unknown port with multiple ports on entrypoint
	}

	for name, spec := range testCases {
		for _, strategy := range spec.strategies {
			t.Run(name+" with "+string(strategy)+" strategy", func(t *testing.T) {
				metricstest.Init(t)

				manager := &LoadBalancer{
					groups: map[string]*group{},
				}

				manager.getEndpoints(myModel).reconcileEndpoints(spec.endpoints)

				ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
				defer cancel()

				gotAddr, gotFunc, gotErr := manager.AwaitBestAddress(ctx, &apiutils.Request{
					Model:   spec.model,
					Adapter: spec.adapter,
					LoadBalancing: v1.LoadBalancing{
						Strategy: strategy,
						PrefixHash: v1.PrefixHash{
							MeanLoadPercentage: 125,
							Replication:        1,
						},
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
}

func TestLoadBalancingStrategies(t *testing.T) {
	const (
		modelA = "model-a"
		modelB = "model-b"

		adapterA1 = "adapter-a-1"
		adapterA2 = "adapter-a-2"

		podA1Name = "pod-a-1"
		podA1Addr = "10.0.0.1:8000"

		podA2Name = "pod-a-2"
		podA2Addr = "10.0.0.2:8000"

		podB1Name = "pod-b-1"
		podB1Addr = "10.0.0.3:8000"

		podB2Name = "pod-b-2"
		podB2Addr = "10.0.0.4:8000"
	)

	var (
		podA1Hash = chwblEndpointReplicaHashInput(podA1Name, 0)
		podA2Hash = chwblEndpointReplicaHashInput(podA2Name, 0)
	)

	type testStep struct {
		name string

		requestCount int
		model        string
		adapter      string
		prefix       string

		expectedAddrCounts map[string]int
		completeForAddrs   map[string]int
	}
	cases := []struct {
		name string
		// map[<model-name>]map[<endpoint-name>]<endpoint>
		modelEndpoints map[string]map[string]endpoint
		// map[<model-name>]map[<endpoint-name>]<in-flight-count>
		initialInFlight map[string]map[string]int64
		loadBalancing   v1.LoadBalancing
		steps           []testStep
	}{
		{
			name: "least load strategy",
			modelEndpoints: map[string]map[string]endpoint{
				modelA: {
					podA1Name: {address: podA1Addr, adapters: map[string]struct{}{adapterA1: {}}},
					podA2Name: {address: podA2Addr, adapters: map[string]struct{}{adapterA2: {}}},
				},
				modelB: {
					podB1Name: {address: podB1Addr},
					podB2Name: {address: podB2Addr},
				},
			},
			loadBalancing: v1.LoadBalancing{
				Strategy: v1.LeastLoadStrategy,
			},
			steps: []testStep{
				{
					name:         "first 2 requests to model-a",
					model:        modelA,
					requestCount: 2,
					expectedAddrCounts: map[string]int{
						podA1Addr: 1,
						podA2Addr: 1,
					},
				},
				{
					name:         "a lot more requests to model-a",
					model:        modelA,
					requestCount: 100,
					expectedAddrCounts: map[string]int{
						podA1Addr: 50,
						podA2Addr: 50,
					},
				},
				{
					name:         "requests to model-a adapter-a-1",
					model:        modelA,
					adapter:      adapterA1,
					requestCount: 50,
					expectedAddrCounts: map[string]int{
						podA1Addr: 50,
					},
				},
				{
					name:         "requests to model-a without adapter should be distributed to the other pod",
					model:        modelA,
					requestCount: 52,
					expectedAddrCounts: map[string]int{
						podA1Addr: 1,
						podA2Addr: 51,
					},
				},
				{
					name:         "back to even balance",
					model:        modelA,
					requestCount: 2,
					expectedAddrCounts: map[string]int{
						podA1Addr: 1,
						podA2Addr: 1,
					},
				},
				{
					name: "complete some request for pod-a-2",
					completeForAddrs: map[string]int{
						podA2Addr: 10,
					},
				},
				{
					name:         "requests to model-a should now be distributed to the other pod",
					model:        modelA,
					requestCount: 12,
					expectedAddrCounts: map[string]int{
						podA1Addr: 1,
						podA2Addr: 11,
					},
				},
				{
					name:         "first requests to model-b",
					model:        modelB,
					requestCount: 2,
					expectedAddrCounts: map[string]int{
						podB1Addr: 1,
						podB2Addr: 1,
					},
				},
			},
		},
		{
			name: "prefix hash strategy",
			modelEndpoints: map[string]map[string]endpoint{
				modelA: {
					podA1Name: {address: podA1Addr},
					podA2Name: {address: podA2Addr},
				},
				modelB: {
					podB1Name: {address: podB1Addr},
				},
			},
			initialInFlight: map[string]map[string]int64{
				modelA: {
					podA1Name: 10,
					podA2Name: 10,
				},
			},
			loadBalancing: v1.LoadBalancing{
				Strategy: v1.PrefixHashStrategy,
				PrefixHash: v1.PrefixHash{
					MeanLoadPercentage: 150,
					Replication:        1,
				},
			},
			steps: []testStep{
				{
					name:         "first request to model-a, preferring pod-a-1, each pod has 10 in-flight requests",
					model:        modelA,
					prefix:       podA1Hash,
					requestCount: 1,
					expectedAddrCounts: map[string]int{
						podA1Addr: 1,
					},
				},
				{
					// load0	load1	1.5*(avg+1)		(load0)+1 <= (thres)
					// 10		10		15.75			TRUE
					// 11		10		16.5			TRUE
					// 12		10		17.25			TRUE
					// 13		10		18				TRUE
					// 14		10		18.75			TRUE
					// 15		10		19.5			TRUE
					// 16		10		20.25			TRUE
					// 17		10		21				TRUE
					// 18		10		21.75			TRUE
					// 19		10		22.5			TRUE
					// 20		10		23.25			TRUE
					// 21		10		24				TRUE
					// 22		10		24.75			TRUE
					// 23		10		25.5			TRUE
					// 24		10		26.25			TRUE
					// 25		10		27				TRUE
					// 26		10		27.75			TRUE
					// 27		10		28.5			TRUE
					// 28		10		29.25			TRUE
					// 29		10		30				TRUE
					// 30		10		30.75			FALSE
					name:  "20 more requests preferring pod-a-1",
					model: modelA,
					// By making sure that the prefix matches the input used to hash the endpoint (pod-a-1),
					// we can ensure that the algorithm will prefer pod-a-1.
					prefix:       podA1Hash,
					requestCount: 20,
					// See the table above for the expected distribution.
					expectedAddrCounts: map[string]int{
						podA1Addr: 19,
						podA2Addr: 1,
					},
				},
				{
					// 30	10	30.75	FALSE
					// 30	11	31.5	TRUE  <-- 1 request (starting here)
					// 31	11	32.25	TRUE  <-- 2 requests
					// 32	11	33		TRUE  <-- 3 requests
					// 33	11	33.75	FALSE <-- 4 requests
					name:         "4 more requests preferring pod-a-1",
					model:        modelA,
					prefix:       podA1Hash,
					requestCount: 4,
					// See the table above for the expected distribution.
					expectedAddrCounts: map[string]int{
						podA1Addr: 3,
						podA2Addr: 1,
					},
				},
				{
					name:         "with pod-a-1 near max load, requests preferring pod-a-2 should be distributed to pod-a-2",
					model:        modelA,
					prefix:       podA2Hash,
					requestCount: 20,
					expectedAddrCounts: map[string]int{
						podA2Addr: 20,
					},
				},
				{
					name:  "requests to model-b should be distributed to pod-b-1, as it is the only endpoint",
					model: modelB,
					// Use a hash that doesn't match any of the endpoints in model-b
					// but does for model-a (to test hash-ring separation by model).
					prefix:       podA2Hash,
					requestCount: 100_000,
					expectedAddrCounts: map[string]int{
						podB1Addr: 100_000,
					},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			manager := &LoadBalancer{
				groups: map[string]*group{},
			}

			for model, endpoints := range c.modelEndpoints {
				manager.getEndpoints(model).reconcileEndpoints(endpoints)
			}

			for modelName, inFlight := range c.initialInFlight {
				for endpointName, count := range inFlight {
					g := manager.getEndpoints(modelName)
					ep := g.endpoints[endpointName]
					g.addInFlight(ep.inFlight, count)
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			doneFuncs := map[string][]func(){}
			for _, step := range c.steps {
				counts := map[string]int{}
				for i := 0; i < step.requestCount; i++ {
					// fmt.Println("request: ", step.name, "i: ", i)
					addr, done, err := manager.AwaitBestAddress(ctx, &apiutils.Request{
						Model:         step.model,
						Adapter:       step.adapter,
						Prefix:        step.prefix,
						LoadBalancing: c.loadBalancing,
					})
					require.NoError(t, err, "request: "+step.name)
					doneFuncs[addr] = append(doneFuncs[addr], done)
					counts[addr]++
				}
				if step.expectedAddrCounts != nil {
					require.Equalf(t, step.expectedAddrCounts, counts, "request: %s", step.name)
				}

				for addr, count := range step.completeForAddrs {
					for i := 0; i < count; i++ {
						doneFuncs[addr][i]()
						// remove the done function from the list
						doneFuncs[addr] = doneFuncs[addr][1:]
					}
				}
			}

			for _, dones := range doneFuncs {
				for _, done := range dones {
					done()
				}
			}
		})
	}
}
