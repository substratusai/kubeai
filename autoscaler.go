package main

import (
	"log"
	"time"
)

// Autoscaler is responsible for making continuous adjustments to
// the scale of the backend. It is not responsible for scale-from-zero.
type Autoscaler struct {
	Interval time.Duration
	Scaler   *ScalerManager
	FIFO     *FIFOQueueManager
}

func (a *Autoscaler) Start() {
	for range time.Tick(a.Interval) {
		log.Println("Calculating scales for all")
		for model, waitCount := range a.FIFO.WaitCounts() {
			a.Scaler.SetDesiredScale(model, int32(waitCount))
		}
	}
}
