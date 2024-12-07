package endpoints

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
)

func newEndpointGroup() *group {
	g := &group{
		endpoints: make(map[string]endpoint),
		chwbl: newCHWBL(chwblConfig{
			LoadFactor:  1.25,
			Replication: 100,
		}),
	}
	return g
}

type group struct {
	mtx sync.RWMutex

	endpoints map[string]endpoint

	totalInFlight *atomic.Int64

	chwbl *chwbl

	bmtx  sync.RWMutex
	bcast chan struct{} // closed when there's a broadcast
}

type endpoint struct {
	address string

	inFlight *atomic.Int64

	adapters map[string]struct{}
}

type LoadBalancingStrategy int

const (
	LeastLoaded LoadBalancingStrategy = iota
	CHWBL                             // Consistent Hashing with Bounded Load
)

// getBestAddr returns the best "IP:Port". It blocks until there are available endpoints
// in the endpoint group.
func (g *group) getBestAddr(ctx context.Context, strategy LoadBalancingStrategy, adapter string, awaitChangeEndpoints bool) (string, func(), error) {
	g.mtx.RLock()
	// await endpoints exists
	for awaitChangeEndpoints || len(g.endpoints) == 0 {
		g.mtx.RUnlock()
		select {
		case <-g.awaitEndpoints():
		case <-ctx.Done():
			return "", func() {}, ctx.Err()
		}
		g.mtx.RLock()
	}

	var ep endpoint
	var found bool
	switch strategy {
	case CHWBL:
		// TODO: prefix
		ep, found = g.chwbl.getAddr(adapter + prefix)
	case LeastLoaded:
		ep, found = g.getAddrLeastLoad(adapter)
	default:
		return "", func() {}, fmt.Errorf("unknown load balancing strategy: %v", strategy)
	}

	if !found {
		g.mtx.RUnlock()
		return g.getBestAddr(ctx, strategy, adapter, true)
	}

	g.addInFlight(ep.inFlight, 1)
	decFunc := func() {
		log.Printf("decrementing in-flight count for %s, new in-flight: %v", ep.address, g.addInFlight(ep.inFlight, -1))
	}
	g.mtx.RUnlock()
	return ep.address, decFunc, nil
}

func (g *group) awaitEndpoints() chan struct{} {
	g.bmtx.RLock()
	defer g.bmtx.RUnlock()
	return g.bcast
}

func (g *group) getAllAddrs() []string {
	g.mtx.RLock()
	defer g.mtx.RUnlock()

	var hosts []string
	for ip := range g.endpoints {
		hosts = append(hosts, ip)
	}

	return hosts
}

func (g *group) lenIPs() int {
	g.mtx.RLock()
	defer g.mtx.RUnlock()
	return len(g.endpoints)
}

func (g *group) reconcileEndpoints(observed map[string]endpoint) {
	g.mtx.Lock()
	for name, observedEp := range observed {
		if currentEp, ok := g.endpoints[name]; ok {
			currentEp.adapters = observedEp.adapters
		} else {
			g.endpoints[name] = endpoint{
				inFlight: &atomic.Int64{},
				address:  observedEp.address,
				adapters: observedEp.adapters,
			}
		}
	}
	for name := range g.endpoints {
		if _, ok := observed[name]; !ok {
			delete(g.endpoints, name)
		}
	}
	g.mtx.Unlock()

	// notify waiting requests
	if len(observed) > 0 {
		g.broadcastEndpoints()
	}
}

func (g *group) broadcastEndpoints() {
	g.bmtx.Lock()
	defer g.bmtx.Unlock()

	close(g.bcast)
	g.bcast = make(chan struct{})
}

func (g *group) addInFlight(endpointInFlight *atomic.Int64, add int64) int64 {
	g.totalInFlight.Add(add)
	return endpointInFlight.Add(add)
}
