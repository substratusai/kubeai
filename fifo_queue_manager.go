package main

import (
	"log"
	"sync"
)

func NewFIFOQueueManager(size int, totalCapacity int) *FIFOQueueManager {
	m := &FIFOQueueManager{}
	m.queues = map[string]*fifoQueue{}
	m.size = size
	m.totalCapacity = totalCapacity
	return m
}

type FIFOQueueManager struct {
	size          int
	totalCapacity int

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

func (m *FIFOQueueManager) UpdateQueueSize(model string, replicas int) {
	newSize := replicas * m.size
	log.Printf("Updating queue size: model: %v, replicas: %v, newSize: %v", model, replicas, newSize)
	m.getQueue(model).resize(newSize)
}

func (m *FIFOQueueManager) getQueue(model string) *fifoQueue {
	m.mtx.Lock()
	q, ok := m.queues[model]
	if !ok {
		q = newFifoQueue(m.size, m.totalCapacity)
		go q.start()
		m.queues[model] = q
	}
	m.mtx.Unlock()
	return q
}
