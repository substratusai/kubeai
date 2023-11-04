package main

import (
	"log"
	"sync"
)

type ModelName string

func NewFIFOQueueManager(size int, totalCapacity int) *FIFOQueueManager {
	m := &FIFOQueueManager{}
	m.queues = map[ModelName]*fifoQueue{}
	m.size = size
	m.totalCapacity = totalCapacity
	return m
}

// FIFOQueueManager manages the queues for each model
type FIFOQueueManager struct {
	// The size of each queue for each model replica
	size          int
	totalCapacity int

	mtx    sync.Mutex
	queues map[ModelName]*fifoQueue
}

// WaitCounts returns the number of pending or in-progress requests for each model
func (m *FIFOQueueManager) WaitCounts() map[ModelName]int64 {
	m.mtx.Lock()
	sizes := make(map[ModelName]int64, len(m.queues))
	for name, q := range m.queues {
		sizes[ModelName(name)] = q.waiting.Load()
	}
	m.mtx.Unlock()
	return sizes
}

// Enqueue adds a request to the queue for the given model name.
func (m *FIFOQueueManager) Enqueue(model string) func() {
	return m.getQueue(model).enqueue()
}

// UpdateQueueSize updates the queue size for the given model name
func (m *FIFOQueueManager) UpdateQueueSize(model string, replicas int) {
	newSize := replicas * m.size
	log.Printf("Updating queue size: model: %v, replicas: %v, newSize: %v", model, replicas, newSize)
	m.getQueue(model).resize(newSize)
}

// getQueue returns the queue for the given model name.
// if the model does not have a queue, a new queue is created.
func (m *FIFOQueueManager) getQueue(model string) *fifoQueue {
	m.mtx.Lock()
	q, ok := m.queues[ModelName(model)]
	if !ok {
		q = newFifoQueue(m.size, m.totalCapacity)
		go q.start()
		m.queues[ModelName(model)] = q
	}
	m.mtx.Unlock()
	return q
}
