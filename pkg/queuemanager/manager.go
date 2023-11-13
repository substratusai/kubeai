package queuemanager

import (
	"context"
	"log"
	"sync"

	"github.com/substratusai/lingo/pkg/queue"
)

func NewFIFOQueueManager(concurrencyPerReplica int) *FIFOQueueManager {
	m := &FIFOQueueManager{}
	m.queues = map[string]*queue.FIFOQueue{}
	m.conccurencyPerReplica = concurrencyPerReplica
	return m
}

// FIFOQueueManager manages the queues for each deployment.
type FIFOQueueManager struct {
	// The default conccurencyPerReplica of each queue for each deployment replica
	conccurencyPerReplica int

	mtx    sync.Mutex
	queues map[string]*queue.FIFOQueue
}

// TotalCounts returns the number of pending or in-progress requests for each deployment name
func (m *FIFOQueueManager) TotalCounts() map[string]int64 {
	m.mtx.Lock()
	sizes := make(map[string]int64, len(m.queues))
	for name, q := range m.queues {
		sizes[name] = q.TotalCount()
	}
	m.mtx.Unlock()
	return sizes
}

// EnqueueAndWait adds a request to the queue for the given deployment name
// and wait for it to be dequeued.
func (m *FIFOQueueManager) EnqueueAndWait(ctx context.Context, deploymentName, id string) func() {
	return m.getQueue(deploymentName).EnqueueAndWait(ctx, id)
}

// UpdateQueueSizeForReplicas updates the queue size for the given deployment name.
func (m *FIFOQueueManager) UpdateQueueSizeForReplicas(deploymentName string, replicas int) {
	// max is needed to prevent the queue size from being set to 0
	newSize := max(replicas*m.conccurencyPerReplica, m.conccurencyPerReplica)
	log.Printf("Updating queue size: deployment: %v, replicas: %v, newSize: %v", deploymentName, replicas, newSize)
	m.getQueue(deploymentName).SetConcurrency(newSize)
}

// getQueue returns the queue for the given model name.
// if the model does not have a queue, a new queue is created.
func (m *FIFOQueueManager) getQueue(deploymentName string) *queue.FIFOQueue {
	m.mtx.Lock()
	q, ok := m.queues[deploymentName]
	if !ok {
		q = queue.NewFIFO(m.conccurencyPerReplica)
		go q.Start()
		m.queues[deploymentName] = q
	}
	m.mtx.Unlock()
	return q
}
