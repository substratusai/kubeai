package queue

import (
	"context"
	"log"
	"sync"
)

func NewManager(concurrencyPerReplica int) *Manager {
	m := &Manager{}
	m.queues = map[string]*Queue{}
	m.conccurencyPerReplica = concurrencyPerReplica
	return m
}

// Manager manages the a set of Queues (for Deployments).
type Manager struct {
	// The default conccurencyPerReplica of each queue for each deployment replica
	conccurencyPerReplica int

	mtx    sync.Mutex
	queues map[string]*Queue
}

// WaitCounts returns the number of pending or in-progress requests for each deployment name
func (m *Manager) WaitCounts() map[string]int {
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
func (m *Manager) EnqueueAndWait(ctx context.Context, deploymentName, id string) func() {
	return m.getQueue(deploymentName).EnqueueAndWait(ctx, id)
}

// UpdateQueueSizeForReplicas updates the queue size for the given deployment name.
func (m *Manager) UpdateQueueSizeForReplicas(deploymentName string, replicas int) {
	newSize := replicas * m.conccurencyPerReplica
	log.Printf("Updating queue size: deployment: %v, replicas: %v, newSize: %v", deploymentName, replicas, newSize)
	m.getQueue(deploymentName).SetConcurrency(newSize)
}

// getQueue returns the queue for the given model name.
// if the model does not have a queue, a new queue is created.
func (m *Manager) getQueue(deploymentName string) *Queue {
	m.mtx.Lock()
	q, ok := m.queues[deploymentName]
	if !ok {
		q = New(m.conccurencyPerReplica)
		go q.Start()
		m.queues[deploymentName] = q
	}
	m.mtx.Unlock()
	return q
}