package main

import "sync/atomic"

type fifoQueue struct {
	queued     chan chan struct{}
	inProgress chan struct{}
	waiting    atomic.Int64
}

func newFifoQueue(size int) *fifoQueue {
	q := &fifoQueue{}
	q.queued = make(chan chan struct{})
	q.inProgress = make(chan struct{}, size)
	return q
}

func (q *fifoQueue) start() {
	for {
		c := <-q.queued
		q.inProgress <- struct{}{}
		close(c)
	}
}

func (q *fifoQueue) enqueue() func() {
	q.waiting.Add(1)
	c := make(chan struct{})
	q.queued <- c
	<-c
	return func() {
		<-q.inProgress
		q.waiting.Add(-1)
	}
}
