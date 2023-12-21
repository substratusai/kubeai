package endpoints

import (
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/util/rand"
)

func TestConcurrentAccess(t *testing.T) {
	const myService = "myService"
	const myPort = "myPort"

	testCases := map[string]struct {
		readerCount int
		writerCount int
	}{
		"lot of reader": {readerCount: 10_000, writerCount: 1},
		"lot of writer": {readerCount: 1, writerCount: 10_000},
		"lot of both":   {readerCount: 10_000, writerCount: 10_000},
	}

	for name, spec := range testCases {
		t.Run(name, func(t *testing.T) {
			endpoint := newEndpointGroup()
			endpoint.setIPs(
				map[string]struct{}{myService: {}},
				map[string]int32{myPort: 1},
			)

			var startWg, doneWg sync.WaitGroup
			startTogether := func(n int, f func()) {
				startWg.Add(n)
				doneWg.Add(n)
				for i := 0; i < n; i++ {
					go func() {
						startWg.Done()
						startWg.Wait()
						f()
						doneWg.Done()
					}()
				}
			}
			startTogether(spec.readerCount, func() { endpoint.getBestHost(nil, myPort) })
			startTogether(spec.writerCount, func() {
				endpoint.setIPs(
					map[string]struct{}{rand.String(1): {}},
					map[string]int32{rand.String(1): 1},
				)
			})
			doneWg.Wait()
		})
	}
}
