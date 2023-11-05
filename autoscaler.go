package main

import (
	"log"
	"math"
	"sync"
	"time"
)

func NewAutoscaler() *Autoscaler {
	return &Autoscaler{movingAvgQueueSize: map[string]*movingAvg{}}
}

// Autoscaler is responsible for making continuous adjustments to
// the scale of the backend. It is not responsible for scale-from-zero.
// Each deployment has its own autoscaler.
type Autoscaler struct {
	Interval     time.Duration
	AverageCount int

	Scaler *DeploymentManager
	FIFO   *FIFOQueueManager

	movingAvgQueueSizeMtx sync.Mutex
	movingAvgQueueSize    map[string]*movingAvg
}

func (a *Autoscaler) Start() {
	for range time.Tick(a.Interval) {
		log.Println("Calculating scales for all")
		for deploymentName, waitCount := range a.FIFO.WaitCounts() {
			avg := a.getMovingAvgQueueSize(deploymentName)
			avg.Next(float64(waitCount))
			flt := avg.Calculate()
			queueSize := a.FIFO.getQueue(deploymentName).getSize()
			ceil := math.Ceil(flt)
			scale := ceil / float64(queueSize)
			log.Printf("Average for deployment: %s: %v (ceil: %v), current wait count: %v", deploymentName, flt, ceil, waitCount)
			a.Scaler.SetDesiredScale(deploymentName, int32(scale))
		}
	}
}

func (r *Autoscaler) getMovingAvgQueueSize(deploymentName string) *movingAvg {
	r.movingAvgQueueSizeMtx.Lock()
	a, ok := r.movingAvgQueueSize[deploymentName]
	if !ok {
		a = newSimpleMovingAvg(make([]float64, r.AverageCount))
		r.movingAvgQueueSize[deploymentName] = a
	}
	r.movingAvgQueueSizeMtx.Unlock()
	return a
}
