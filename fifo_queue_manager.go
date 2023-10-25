package main

import (
	"sync"
)

func NewFIFOQueueManager(size int) *FIFOQueueManager {
	m := &FIFOQueueManager{}
	m.queues = map[string]*fifoQueue{}
	m.size = size
	return m
}

type FIFOQueueManager struct {
	size int

	mtx    sync.Mutex
	queues map[string]*fifoQueue
}

func (m *FIFOQueueManager) WaitCounts() map[string]int64 {
	m.mtx.Lock()
	sizes := make(map[string]int64, len(m.queues))
	for name, q := range m.queues {
		sizes[name] = q.waiting.Load()
	}
	m.mtx.Unlock()
	return sizes
}

func (m *FIFOQueueManager) Enqueue(model string) func() {
	return m.getQueue(model).enqueue()
}

func (m *FIFOQueueManager) getQueue(model string) *fifoQueue {
	m.mtx.Lock()
	q, ok := m.queues[model]
	if !ok {
		q = newFifoQueue(m.size)
		go q.start()
		m.queues[model] = q
	}
	m.mtx.Unlock()
	return q
}
