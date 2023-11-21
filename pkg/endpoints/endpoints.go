package endpoints

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func newEndpointGroup() *endpointGroup {
	e := &endpointGroup{}
	e.ports = make(map[string]int32)
	e.endpoints = make(map[string]endpoint)
	e.active = sync.NewCond(&e.mtx)
	return e
}

type endpoint struct {
	inFlight *atomic.Int64
}

type endpointGroup struct {
	ports     map[string]int32
	endpoints map[string]endpoint
	active    *sync.Cond
	mtx       sync.Mutex
}

func (e *endpointGroup) getHost(portName string) string {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	for len(e.endpoints) == 0 {
		e.active.Wait()
	}

	var bestIP string
	port := e.ports[portName]
	var minInFlight int
	for ip := range e.endpoints {
		inFlight := int(e.endpoints[ip].inFlight.Load())
		if bestIP == "" || inFlight < minInFlight {
			bestIP = ip
			minInFlight = inFlight
		}
	}

	return fmt.Sprintf("%s:%v", bestIP, port)
}

func (e *endpointGroup) getAllHosts(portName string) []string {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	var hosts []string
	port := e.ports[portName]
	for ip := range e.endpoints {
		hosts = append(hosts, fmt.Sprintf("%s:%v", ip, port))
	}

	return hosts
}

func (g *endpointGroup) lenIPs() int {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return len(g.endpoints)
}

func (g *endpointGroup) setIPs(ips map[string]struct{}, ports map[string]int32) {
	g.mtx.Lock()
	defer g.mtx.Unlock()

	g.ports = ports
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
