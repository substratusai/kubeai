package queue

import (
	"container/list"
	"context"
	"fmt"
	"sync"
	"time"
)

func NewFIFO(initialConcurrency int) *FIFOQueue {
	return &FIFOQueue{
		concurrency: initialConcurrency,
		list:        list.New(),
		enqueued:    make(chan struct{}),
		completed:   make(chan struct{}),
	}
}

type FIFOQueue struct {
	// concurrency is the max number of in progress items.
	concurrency    int
	concurrencyMtx sync.RWMutex

	list    *list.List
	listMtx sync.Mutex

	// enqueued signals when a new item is enqueued.
	enqueued chan struct{}

	// completed signals when a item that has been dequeued has completed.
	completed chan struct{}
}

type item struct {
	id string
	e  *list.Element

	// dequeued is a channel that signals this item was removed from the queue.
	// It is used to block EnqueueAndWait().
	dequeued chan struct{}

	inProgress bool

	closed bool
}

func (q *FIFOQueue) dequeue(itm *item, inProgress bool) {
	q.listMtx.Lock()
	itm.inProgress = inProgress
	q.list.Remove(itm.e)
	if !itm.closed {
		close(itm.dequeued)
		itm.closed = true
	}
	q.listMtx.Unlock()
}

// EnqueueAndWait adds an item to the queue and waits for it to be dequeued.
// It returns a function that should be called after all work has completed.
// The id parameter is only used for tracking/debugging purposes.
func (q *FIFOQueue) EnqueueAndWait(ctx context.Context, id string) func() {
	itm := &item{
		id:       id,
		dequeued: make(chan struct{}),
	}
	q.listMtx.Lock()
	itm.e = q.list.PushBack(itm)
	q.listMtx.Unlock()

	// Send a notification on the enqueued channel only if the central loop
	// is waiting for an item to be enqueued (only happens when queue is empty).
	select {
	case q.enqueued <- struct{}{}:
	default:
	}

	// Wait to be dequeued.
	select {
	case <-itm.dequeued:
	case <-ctx.Done():
		q.dequeue(itm, false)
	}

	return q.completeFunc(itm)
}

func (q *FIFOQueue) completeFunc(itm *item) func() {
	return func() {
		q.listMtx.Lock()
		if !itm.closed {
			close(itm.dequeued)
			itm.closed = true
		}
		inProgress := itm.inProgress
		q.listMtx.Unlock()
		if inProgress {
			// Make sure we only send a message on the completed channel if the
			// item was counted as inProgress.
			q.completed <- struct{}{}
		}
	}
}

func (q *FIFOQueue) Start() {
	var inProgress int
	for {
		if inProgress >= q.GetConcurrency() {
			<-q.completed
			inProgress--
			continue
		}

		q.listMtx.Lock()
		e := q.list.Front()
		q.listMtx.Unlock()

		if e == nil {
			// Queue is empty, wait until something is enqueued.
			<-q.enqueued
			continue
		}

		inProgress++

		itm := e.Value.(*item)
		q.dequeue(itm, true)
		fmt.Println("Dequeued: ", itm.id)

		time.Sleep(time.Second / 100)
	}
}

func (q *FIFOQueue) GetConcurrency() int {
	q.concurrencyMtx.RLock()
	defer q.concurrencyMtx.RUnlock()
	return q.concurrency
}

func (q *FIFOQueue) SetConcurrency(n int) {
	q.concurrencyMtx.Lock()
	q.concurrency = n
	q.concurrencyMtx.Unlock()
}

func (q *FIFOQueue) Size() int {
	q.listMtx.Lock()
	defer q.listMtx.Unlock()
	return q.list.Len()
}
