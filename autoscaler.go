package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewAutoscaler(mgr ctrl.Manager) (*Autoscaler, error) {
	a := &Autoscaler{}
	a.Client = mgr.GetClient()
	a.movingAvgQueueSize = map[string]*movingAvg{}
	if err := a.SetupWithManager(mgr); err != nil {
		return nil, err
	}
	return a, nil
}

// Autoscaler is responsible for making continuous adjustments to
// the scale of the backend. It is not responsible for scale-from-zero.
// Each deployment has its own autoscaler.
type Autoscaler struct {
	client.Client

	Interval     time.Duration
	AverageCount int

	LeaderElection *LeaderElection

	Scaler *DeploymentManager
	FIFO   *FIFOQueueManager

	HTTPClient *http.Client

	movingAvgQueueSizeMtx sync.Mutex
	movingAvgQueueSize    map[string]*movingAvg
}

func (r *Autoscaler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Complete(r)
}

func (r *Autoscaler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cm corev1.ConfigMap
	if err := r.Get(ctx, req.NamespacedName, &cm); err != nil {
		return ctrl.Result{}, fmt.Errorf("get: %w", err)
	}

	return ctrl.Result{}, nil
}

func (a *Autoscaler) Start() {
	for range time.Tick(a.Interval) {
		if !a.LeaderElection.IsLeader.Load() {
			log.Println("Not leader, doing nothing")
			continue
		}

		log.Println("Calculating scales for all")

		concurrencyPerReplica := a.FIFO.concurrencyPerReplica

		statsEndpoints := []string{}
		stats, errs := aggregateStats(a.FIFO, a.HTTPClient, statsEndpoints)
		if len(errs) != 0 {
			for _, err := range errs {
				log.Printf("Failed to aggregate stats: %v", err)
			}
			continue
		}

		for deploymentName, waitCount := range stats.WaitCounts {
			log.Println("Is leader, autoscaling")
			avg := a.getMovingAvgQueueSize(deploymentName)
			avg.Next(float64(waitCount))
			flt := avg.Calculate()
			// TODO fix this to use configurable concurrency setting that's supplied
			// by the user.
			// Note this uses the default queue size, not the current queue size.
			// the current queue size increases and decreases based on replica count
			normalized := flt / float64(concurrencyPerReplica)
			ceil := math.Ceil(normalized)
			log.Printf("Average for deployment: %s: %v (ceil: %v), current wait count: %v", deploymentName, flt, ceil, waitCount)
			a.Scaler.SetDesiredScale(deploymentName, int32(ceil))
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

func aggregateStats(thisQueue *FIFOQueueManager, httpc *http.Client, endpoints []string) (Stats, []error) {
	stats := Stats{
		WaitCounts: thisQueue.WaitCounts(),
	}

	var errs []error
	for _, endpoint := range endpoints {
		s, err := getStats(httpc, endpoint)
		if err != nil {
			errs = append(errs, fmt.Errorf("getting stats: %v: %v", endpoint, err))
			continue
		}
		for k, v := range s.WaitCounts {
			stats.WaitCounts[k] = stats.WaitCounts[k] + v
		}
	}

	return stats, errs
}

func getStats(httpc *http.Client, endpoint string) (Stats, error) {
	var stats Stats

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return stats, fmt.Errorf("new request: %w", err)
	}

	resp, err := httpc.Do(req)
	if err != nil {
		return stats, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return stats, fmt.Errorf("decoding response body: %w", err)
	}

	return stats, nil
}
