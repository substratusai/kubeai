package main

import (
	"sync"
	"sync/atomic"
)

func newEndpoints() *endpointGroup {
	e := &endpointGroup{}
	e.endpoints = make(map[string]endpoint)
	e.active = sync.NewCond(&e.mtx)
	return e
}

type endpoint struct {
	inFlight *atomic.Int64
}

type endpointGroup struct {
	endpoints map[string]endpoint
	active    *sync.Cond
	mtx       sync.Mutex
}

func (e *endpointGroup) getIP() string {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	for len(e.endpoints) == 0 {
		e.active.Wait()
	}

	var bestIP string
	var minInFlight int
	for ip := range e.endpoints {
		inFlight := int(e.endpoints[ip].inFlight.Load())
		if bestIP == "" || inFlight < minInFlight {
			bestIP = ip
			minInFlight = inFlight
		}
	}

	return bestIP
}

func (g *endpointGroup) lenIPs() int {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return len(g.endpoints)
}

func (g *endpointGroup) setIPs(ips map[string]struct{}) {
	g.mtx.Lock()
	defer g.mtx.Unlock()

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

	if len(g.endpoints) > 0 {
		g.active.Broadcast()
	}
}
