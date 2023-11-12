package queue

import (
	"context"
	"log"
	"sync"
)

func NewFIFOManager(concurrencyPerReplica int) *FIFOManager {
	m := &FIFOManager{}
	m.queues = map[string]*FIFO{}
	m.conccurencyPerReplica = concurrencyPerReplica
	return m
}

// FIFOManager manages the queues for each deployment.
type FIFOManager struct {
	// The default conccurencyPerReplica of each queue for each deployment replica
	conccurencyPerReplica int

	mtx    sync.Mutex
	queues map[string]*FIFO
}

// WaitCounts returns the number of pending or in-progress requests for each deployment name
func (m *FIFOManager) WaitCounts() map[string]int {
	m.mtx.Lock()
	sizes := make(map[string]int, len(m.queues))
	for name, q := range m.queues {
		sizes[name] = q.Size()
	}
	m.mtx.Unlock()
	return sizes
}

// EnqueueAndWait adds a request to the queue for the given deployment name
// and wait for it to be dequeued.
func (m *FIFOManager) EnqueueAndWait(ctx context.Context, deploymentName, id string) func() {
	return m.getQueue(deploymentName).EnqueueAndWait(ctx, id)
}

// UpdateQueueSizeForReplicas updates the queue size for the given deployment name.
func (m *FIFOManager) UpdateQueueSizeForReplicas(deploymentName string, replicas int) {
	newSize := replicas * m.conccurencyPerReplica
	log.Printf("Updating queue size: deployment: %v, replicas: %v, newSize: %v", deploymentName, replicas, newSize)
	m.getQueue(deploymentName).SetConcurrency(newSize)
}

// getQueue returns the queue for the given model name.
// if the model does not have a queue, a new queue is created.
func (m *FIFOManager) getQueue(deploymentName string) *FIFO {
	m.mtx.Lock()
	q, ok := m.queues[deploymentName]
	if !ok {
		q = NewFIFO(m.conccurencyPerReplica)
		go q.Start()
		m.queues[deploymentName] = q
	}
	m.mtx.Unlock()
	return q
}
