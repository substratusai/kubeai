package modelresolver

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
)

func newEndpointGroup() *endpointGroup {
	e := &endpointGroup{}
	e.endpoints = make(map[string]endpoint)
	e.bcast = make(chan struct{})
	return e
}

type endpoint struct {
	inFlight *atomic.Int64
}

type endpointGroup struct {
	mtx       sync.RWMutex
	endpoints map[string]endpoint

	bmtx  sync.RWMutex
	bcast chan struct{} // closed when there's a broadcast
}

// getBestAddr returns the best "IP:Port". It blocks until there are available endpoints
// in the endpoint group. It selects the host with the minimum in-flight requests
// among all the available endpoints.
func (e *endpointGroup) getBestAddr(ctx context.Context) (string, func(), error) {
	e.mtx.RLock()
	// await endpoints exists
	for len(e.endpoints) == 0 {
		e.mtx.RUnlock()
		select {
		case <-e.awaitEndpoints():
		case <-ctx.Done():
			return "", func() {}, ctx.Err()
		}
		e.mtx.RLock()
	}
	var bestAddr string
	var minInFlight int
	for addr := range e.endpoints {
		inFlight := int(e.endpoints[addr].inFlight.Load())
		if bestAddr == "" || inFlight < minInFlight {
			bestAddr = addr
			minInFlight = inFlight
		}
	}
	ep := e.endpoints[bestAddr]
	ep.inFlight.Add(1)
	decFunc := func() {
		log.Printf("decrementing in-flight count for %s, new in-flight: %v", bestAddr, ep.inFlight.Add(-1))
	}
	e.mtx.RUnlock()
	return bestAddr, decFunc, nil
}

func (e *endpointGroup) awaitEndpoints() chan struct{} {
	e.bmtx.RLock()
	defer e.bmtx.RUnlock()
	return e.bcast
}

func (e *endpointGroup) getAllAddrs() []string {
	e.mtx.RLock()
	defer e.mtx.RUnlock()

	var hosts []string
	for ip := range e.endpoints {
		hosts = append(hosts, ip)
	}

	return hosts
}

func (g *endpointGroup) lenIPs() int {
	g.mtx.RLock()
	defer g.mtx.RUnlock()
	return len(g.endpoints)
}

func (g *endpointGroup) setAddrs(ips map[string]struct{}) {
	g.mtx.Lock()
	for ip := range ips {
		if _, ok := g.endpoints[ip]; !ok {
			g.endpoints[ip] = endpoint{inFlight: &atomic.Int64{}}
		}
	}
	for ip := range g.endpoints {
		if _, ok := ips[ip]; !ok {
			delete(g.endpoints, ip)
		}
	}
	g.mtx.Unlock()

	// notify waiting requests
	if len(ips) > 0 {
		g.broadcastEndpoints()
	}
}

func (g *endpointGroup) broadcastEndpoints() {
	g.bmtx.Lock()
	defer g.bmtx.Unlock()

	close(g.bcast)
	g.bcast = make(chan struct{})
}
