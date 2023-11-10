package main

import (
	"log"
	"sync"
)

func NewFIFOQueueManager(size int, totalCapacity int) *FIFOQueueManager {
	m := &FIFOQueueManager{}
	m.queues = map[string]*fifoQueue{}
	m.concurrencyPerReplica = size
	m.totalCapacity = totalCapacity
	return m
}

// FIFOQueueManager manages the queues for each deployment
type FIFOQueueManager struct {
	// The default concurrencyPerReplica of each queue for each deployment replica
	concurrencyPerReplica int

	// The default total capacity of the queue for deployment
	totalCapacity int

	mtx    sync.Mutex
	queues map[string]*fifoQueue
}

// WaitCounts returns the number of pending or in-progress requests for each deployment name
func (m *FIFOQueueManager) WaitCounts() map[string]int64 {
	m.mtx.Lock()
	sizes := make(map[string]int64, len(m.queues))
	for name, q := range m.queues {
		sizes[name] = q.waiting.Load()
	}
	m.mtx.Unlock()
	return sizes
}

// Enqueue adds a request to the queue for the given deployment name.
func (m *FIFOQueueManager) Enqueue(deploymentName string) func() {
	return m.getQueue(deploymentName).enqueue()
}

// UpdateQueueSize updates the queue size for the given model name
func (m *FIFOQueueManager) UpdateQueueSize(deploymentName string, replicas int) {
	newSize := replicas * m.concurrencyPerReplica
	log.Printf("Updating queue size: deployment: %v, replicas: %v, newSize: %v", deploymentName, replicas, newSize)
	m.getQueue(deploymentName).resize(newSize)
}

// getQueue returns the queue for the given model name.
// if the model does not have a queue, a new queue is created.
func (m *FIFOQueueManager) getQueue(deploymentName string) *fifoQueue {
	m.mtx.Lock()
	q, ok := m.queues[deploymentName]
	if !ok {
		q = newFifoQueue(m.concurrencyPerReplica, m.totalCapacity)
		go q.start()
		m.queues[deploymentName] = q
	}
	m.mtx.Unlock()
	return q
}
