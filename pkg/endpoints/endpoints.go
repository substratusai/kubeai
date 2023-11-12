package endpoints

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func newEndpointGroup() *endpointGroup {
	e := &endpointGroup{}
	e.endpoints = make(map[string]endpoint)
	e.active = sync.NewCond(&e.mtx)
	return e
}

type endpoint struct {
	port     int32
	inFlight *atomic.Int64
}

type endpointGroup struct {
	endpoints map[string]endpoint
	active    *sync.Cond
	mtx       sync.Mutex
}

func (e *endpointGroup) getHost() string {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	for len(e.endpoints) == 0 {
		e.active.Wait()
	}

	var bestIP string
	var port int32
	var minInFlight int
	for ip := range e.endpoints {
		inFlight := int(e.endpoints[ip].inFlight.Load())
		if bestIP == "" || inFlight < minInFlight {
			bestIP = ip
			port = e.endpoints[ip].port
			minInFlight = inFlight
		}
	}

	return fmt.Sprintf("%s:%v", bestIP, port)
}

func (g *endpointGroup) lenIPs() int {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return len(g.endpoints)
}

func (g *endpointGroup) setIPs(ips map[string]struct{}, port int32) {
	g.mtx.Lock()
	defer g.mtx.Unlock()

	for ip := range ips {
		if _, ok := g.endpoints[ip]; !ok {
			g.endpoints[ip] = endpoint{inFlight: &atomic.Int64{}, port: port}
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
