package main

import (
	"log"
	"sync"
	"sync/atomic"
)

type fifoQueue struct {
	// queued is a channel of channels that are waiting to be processed.
	queued    chan chan struct{}
	completed chan struct{}
	waiting   atomic.Int64

	sizeMtx sync.RWMutex
	size    int
}

func newFifoQueue(size, queueCapacity int) *fifoQueue {
	q := &fifoQueue{}
	q.queued = make(chan chan struct{}, queueCapacity)
	q.completed = make(chan struct{}, queueCapacity)
	q.size = size
	return q
}

func (q *fifoQueue) start() {
	log.Println("Starting new fifo queue")
	var inProgress int = 0
	for {
		log.Println("inProgress", inProgress, "getSize()", q.getSize())
		if inProgress >= q.getSize() {
			<-q.completed
			inProgress--
			continue
		}
		inProgress++

		c := <-q.queued
		close(c)
	}
}

func (q *fifoQueue) getSize() int {
	q.sizeMtx.RLock()
	defer q.sizeMtx.RUnlock()
	return q.size
}

func (q *fifoQueue) resize(size int) {
	q.sizeMtx.Lock()
	q.size = size
	q.sizeMtx.Unlock()
}

func (q *fifoQueue) enqueue() func() {
	q.waiting.Add(1)
	c := make(chan struct{})
	q.queued <- c
	<-c
	return func() {
		q.completed <- struct{}{}
		q.waiting.Add(-1)
	}
}
