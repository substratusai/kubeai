package modelautoscaler

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/substratusai/kubeai/internal/leader"
	"github.com/substratusai/kubeai/internal/modelresolver"
	"github.com/substratusai/kubeai/internal/modelscaler"
	"github.com/substratusai/kubeai/internal/movingaverage"
)

func New(
	interval time.Duration,
	averageCount int,
	leaderElection *leader.Election,
	scaler *modelscaler.ModelScaler,
	resolver *modelresolver.Manager,
	targetRequests float64,
) *Autoscaler {
	a := &Autoscaler{
		interval:         interval,
		averageCount:     averageCount,
		leaderElection:   leaderElection,
		scaler:           scaler,
		resolver:         resolver,
		targetRequests:   targetRequests,
		movingAvgByModel: map[string]*movingaverage.Simple{},
	}
	return a
}

// Autoscaler is responsible for making continuous adjustments to
// the scale of the backend. It is not responsible for scale-from-zero.
// Each deployment has its own autoscaler.
type Autoscaler struct {
	interval     time.Duration
	averageCount int

	leaderElection *leader.Election

	scaler   *modelscaler.ModelScaler
	resolver *modelresolver.Manager

	targetRequests float64

	movingAvgByModelMtx sync.Mutex
	movingAvgByModel    map[string]*movingaverage.Simple
}

func (a *Autoscaler) Start() {
	for range time.Tick(a.interval) {
		if !a.leaderElection.IsLeader.Load() {
			log.Println("Not leader, doing nothing")
			continue
		}

		log.Println("Calculating scales for all")

		// TODO: Remove hardcoded Service lookup by name "lingo".

		models, err := a.scaler.ListAllModels(context.Background())
		if err != nil {
			log.Printf("Failed to list models: %v", err)
			continue
		}

		for _, m := range models {
			endpointAddrs := a.resolver.GetAllAddresses(m)
			if len(endpointAddrs) == 0 {
				log.Printf("No ready endpoints found for model: %s, skipping", m)
				continue
			}

			agg := &vLLMMetrics{}
			errs := aggregateMetrics(agg, endpointAddrs)
			if len(errs) != 0 {
				for _, err := range errs {
					log.Printf("Failed to aggregate stats: %v", err)
				}
				continue
			}

			log.Println("Is leader, autoscaling")
			avg := a.getMovingAvgQueueSize(m)
			avg.Next(agg.CurrentRequests())
			flt := avg.Calculate()
			normalized := flt / a.targetRequests
			ceil := math.Ceil(normalized)
			log.Printf("Average for model %q: %v/%v (normalized ceil: %v), current requests: %v, history: %v", m, flt, a.targetRequests, ceil, agg.CurrentRequests(), avg.History())
			a.scaler.Scale(context.Background(), m, int32(ceil))
		}
	}
}

func (r *Autoscaler) getMovingAvgQueueSize(deploymentName string) *movingaverage.Simple {
	r.movingAvgByModelMtx.Lock()
	a, ok := r.movingAvgByModel[deploymentName]
	if !ok {
		a = movingaverage.NewSimple(make([]float64, r.averageCount))
		r.movingAvgByModel[deploymentName] = a
	}
	r.movingAvgByModelMtx.Unlock()
	return a
}

func aggregateMetrics(agg metricsAggregator, endpointAddrs []string) []error {
	var errs []error

	for _, addr := range endpointAddrs {
		if err := scrapeSumMetrics(agg, fmt.Sprintf("http://%s/metrics", addr)); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}
