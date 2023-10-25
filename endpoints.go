package main

import (
	"sync"
)

func newEndpoints() *endpoints {
	e := &endpoints{}
	e.active = sync.NewCond(&e.mtx)
	return e
}

type endpoints struct {
	ips    []string
	active *sync.Cond
	mtx    sync.Mutex
}

func (e *endpoints) get() []string {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	for len(e.ips) == 0 {
		e.active.Wait()
	}

	// Return copy of IPs slice.
	ips := make([]string, len(e.ips))
	copy(ips, e.ips)
	return ips
}

func (e *endpoints) set(ips []string) {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	e.ips = ips

	if len(e.ips) > 0 {
		e.active.Broadcast()
	}
}
