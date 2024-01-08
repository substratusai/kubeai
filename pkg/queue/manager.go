package queue

import (
	"context"
	"errors"
	"log"
	"sync"
)

func NewManager(concurrencyPerReplica int32) *Manager {
	return &Manager{
		queues:                            make(map[string]*Queue),
		defaultConcurrencyPerReplica:      concurrencyPerReplica,
		deploymentToConcurrencyPerReplica: make(map[string]uint32),
	}
}

// Manager manages the set of Queues (for Deployments).
type Manager struct {
	// defaultConcurrencyPerReplica of each queue for each deployment replica.
	defaultConcurrencyPerReplica int32

	mtx    sync.RWMutex
	queues map[string]*Queue

	deploymentToConcurrencyPerReplicaMtx sync.RWMutex
	deploymentToConcurrencyPerReplica    map[string]uint32
}

// TotalCounts returns the number of pending or in-progress requests for each deployment name.
func (m *Manager) TotalCounts() map[string]int64 {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	sizes := make(map[string]int64, len(m.queues))
	for name, q := range m.queues {
		sizes[name] = q.TotalCount()
	}
	return sizes
}

// InProgressCount returns the number of in-progress requests for each deployment name.
func (m *Manager) InProgressCount() map[string]int64 {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	sizes := make(map[string]int64, len(m.queues))
	for name, q := range m.queues {
		sizes[name] = q.InProgressCount()
	}
	return sizes
}

// EnqueueAndWait adds a request to the queue for the given deployment name
// and wait for it to be dequeued.
func (m *Manager) EnqueueAndWait(ctx context.Context, deploymentName, id string) func() {
	return m.getQueue(deploymentName).EnqueueAndWait(ctx, id)
}

// UpdateQueueSizeForReplicas updates the queue size for the given deployment name.
func (m *Manager) UpdateQueueSizeForReplicas(deploymentName string, replicas int) {
	// max is needed to prevent the queue size from being set to 0
	perReplica := int(m.GetCurrencyPerReplica(deploymentName))
	newSize := max(replicas*perReplica, perReplica)
	log.Printf("Updating queue size: deployment: %v, replicas: %v, newSize: %v", deploymentName, replicas, newSize)
	m.getQueue(deploymentName).SetConcurrency(newSize)
}

// getQueue returns the queue for the given model name.
// if the model does not have a queue, a new queue is created.
func (m *Manager) getQueue(deploymentName string) *Queue {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	q, ok := m.queues[deploymentName]
	if !ok {
		q = New(int(m.GetCurrencyPerReplica(deploymentName)))
		m.queues[deploymentName] = q
		go q.Start()
	}
	return q
}

func (r *Manager) GetCurrencyPerReplica(deployment string) uint32 {
	r.deploymentToConcurrencyPerReplicaMtx.RLock()
	defer r.deploymentToConcurrencyPerReplicaMtx.RUnlock()
	if n, exists := r.deploymentToConcurrencyPerReplica[deployment]; exists {
		return n
	}
	return uint32(r.defaultConcurrencyPerReplica)
}

func (r *Manager) SetCurrencyPerReplica(deployment string, value uint32) error {
	if value == 0 {
		return errors.New("empty value")
	}
	r.deploymentToConcurrencyPerReplicaMtx.Lock()
	defer r.deploymentToConcurrencyPerReplicaMtx.Unlock()
	r.deploymentToConcurrencyPerReplica[deployment] = value
	return nil
}
