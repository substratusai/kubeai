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
			ceil := math.Ceil(flt)
			log.Printf("Average for model: %s: %v (ceil: %v), current wait count: %v", deploymentName, flt, ceil, waitCount)
			a.Scaler.SetDesiredScale(deploymentName, int32(ceil))
		}
	}
}

func (r *Autoscaler) getMovingAvgQueueSize(model string) *movingAvg {
	r.movingAvgQueueSizeMtx.Lock()
	a, ok := r.movingAvgQueueSize[model]
	if !ok {
		a = newSimpleMovingAvg(make([]float64, r.AverageCount))
		r.movingAvgQueueSize[model] = a
	}
	r.movingAvgQueueSizeMtx.Unlock()
	return a
}
