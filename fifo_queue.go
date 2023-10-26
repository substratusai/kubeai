package main

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
)

type fifoQueue struct {
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
	var inProgress int
	for {
		fmt.Println("1")
		fmt.Println("inProgress", inProgress, "getSize()", q.getSize())
		if inProgress >= q.getSize() {
			fmt.Println("2")
			<-q.completed
			fmt.Println("3")
			inProgress--
			fmt.Println("4")
			continue
		}
		fmt.Println("5")
		inProgress++

		c := <-q.queued
		fmt.Println("6")
		close(c)
		fmt.Println("7")
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
