package endpoints

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/util/rand"
)

func TestConcurrentAccess(t *testing.T) {
	const myModel = "myModel"

	testCases := map[string]struct {
		readerCount int
		writerCount int
	}{
		"lot of reader": {readerCount: 1_000, writerCount: 1},
		"lot of writer": {readerCount: 1, writerCount: 1_000},
		"lot of both":   {readerCount: 1_000, writerCount: 1_000},
	}
	for name, spec := range testCases {
		randomReadFn := []func(g *endpointGroup){
			func(g *endpointGroup) { g.getBestAddr(nil) },
			func(g *endpointGroup) { g.getAllAddrs() },
			func(g *endpointGroup) { g.lenIPs() },
		}
		t.Run(name, func(t *testing.T) {
			// setup endpoint with one service so that requests are not waiting
			endpoint := newEndpointGroup()
			endpoint.setAddrs(
				map[string]struct{}{myModel: {}},
			)

			var startWg, doneWg sync.WaitGroup
			startWg.Add(spec.readerCount + spec.writerCount)
			doneWg.Add(spec.readerCount + spec.writerCount)
			startTogether := func(n int, f func()) {
				for i := 0; i < n; i++ {
					go func() {
						startWg.Done()
						startWg.Wait()
						f()
						doneWg.Done()
					}()
				}
			}
			// when
			startTogether(spec.readerCount, func() { randomReadFn[rand.Intn(len(randomReadFn)-1)](endpoint) })
			startTogether(spec.writerCount, func() {
				endpoint.setAddrs(
					map[string]struct{}{rand.String(1): {}},
				)
			})
			doneWg.Wait()
		})
	}
}

func TestBlockAndWaitForEndpoints(t *testing.T) {
	var completed atomic.Int32
	var startWg, doneWg sync.WaitGroup
	startTogether := func(n int, f func()) {
		startWg.Add(n)
		doneWg.Add(n)
		for i := 0; i < n; i++ {
			go func() {
				startWg.Done()
				startWg.Wait()
				f()
				completed.Add(1)
				doneWg.Done()
			}()
		}
	}
	endpoint := newEndpointGroup()
	ctx := context.TODO()
	startTogether(100, func() {
		endpoint.getBestAddr(ctx)
	})
	startWg.Wait()

	// when broadcast triggered
	endpoint.setAddrs(
		map[string]struct{}{rand.String(4): {}},
	)
	// then
	doneWg.Wait()
	assert.Equal(t, int32(100), completed.Load())
}

func TestAbortOnCtxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var startWg, doneWg sync.WaitGroup
	startWg.Add(1)
	doneWg.Add(1)
	go func(t *testing.T) {
		startWg.Wait()
		endpoint := newEndpointGroup()
		_, f, err := endpoint.getBestAddr(ctx)
		defer f()
		require.Error(t, err)
		doneWg.Done()
	}(t)
	startWg.Done()
	cancel()

	doneWg.Wait()
}
