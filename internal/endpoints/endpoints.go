package endpoints

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

type endpointGroup struct {
	mtx       sync.RWMutex
	endpoints map[string]endpoint

	bmtx  sync.RWMutex
	bcast chan struct{} // closed when there's a broadcast
}

func newEndpoint(attrs endpointAttrs) endpoint {
	return endpoint{
		inFlight:      &atomic.Int64{},
		endpointAttrs: attrs,
	}
}

type endpoint struct {
	inFlight *atomic.Int64
	endpointAttrs
}

// getBestAddr returns the best "IP:Port". It blocks until there are available endpoints
// in the endpoint group. It selects the host with the minimum in-flight requests
// among all the available endpoints.
func (e *endpointGroup) getBestAddr(ctx context.Context, adapter string, awaitChangeEndpoints bool) (string, func(), error) {
	e.mtx.RLock()
	// await endpoints exists
	for awaitChangeEndpoints || len(e.endpoints) == 0 {
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
	for addr, ep := range e.endpoints {
		if adapter != "" {
			// Skip endpoints that don't have the requested adapter.
			if _, ok := ep.adapters[adapter]; !ok {
				continue
			}
		}
		inFlight := int(e.endpoints[addr].inFlight.Load())
		if bestAddr == "" || inFlight < minInFlight {
			bestAddr = addr
			minInFlight = inFlight
		}
	}

	if bestAddr == "" {
		e.mtx.RUnlock()
		return e.getBestAddr(ctx, adapter, true)
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

type endpointAttrs struct {
	adapters map[string]struct{}
}

func (g *endpointGroup) setAddrs(addrs map[string]endpointAttrs) {
	g.mtx.Lock()
	for addr, attrs := range addrs {
		if _, ok := g.endpoints[addr]; !ok {
			g.endpoints[addr] = newEndpoint(attrs)
		}
	}
	for addr := range g.endpoints {
		if _, ok := addrs[addr]; !ok {
			delete(g.endpoints, addr)
		}
	}
	g.mtx.Unlock()

	// notify waiting requests
	if len(addrs) > 0 {
		g.broadcastEndpoints()
	}
}

func (g *endpointGroup) broadcastEndpoints() {
	g.bmtx.Lock()
	defer g.bmtx.Unlock()

	close(g.bcast)
	g.bcast = make(chan struct{})
}
