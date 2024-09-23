package modelautoscaler

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/substratusai/kubeai/internal/config"
	"github.com/substratusai/kubeai/internal/leader"
	"github.com/substratusai/kubeai/internal/modelresolver"
	"github.com/substratusai/kubeai/internal/modelscaler"
	"github.com/substratusai/kubeai/internal/movingaverage"
)

func New(
	leaderElection *leader.Election,
	scaler *modelscaler.ModelScaler,
	resolver *modelresolver.Manager,
	cfg config.ModelAutoscaling,
) *Autoscaler {
	a := &Autoscaler{
		leaderElection:   leaderElection,
		scaler:           scaler,
		resolver:         resolver,
		movingAvgByModel: map[string]*movingaverage.Simple{},
		cfg:              cfg,
	}
	return a
}

// Autoscaler is responsible for making continuous adjustments to
// the scale of the backend. It is not responsible for scale-from-zero.
// Each deployment has its own autoscaler.
type Autoscaler struct {
	leaderElection *leader.Election

	scaler   *modelscaler.ModelScaler
	resolver *modelresolver.Manager

	cfg config.ModelAutoscaling

	movingAvgByModelMtx sync.Mutex
	movingAvgByModel    map[string]*movingaverage.Simple
}

func (a *Autoscaler) Start(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.Interval.Duration)
	defer ticker.Stop()
	for range ticker.C {
		if ctx.Err() != nil {
			return
		}
		if !a.leaderElection.IsLeader.Load() {
			log.Println("Not leader, doing nothing")
			continue
		}

		log.Println("Calculating scales for all")

		// TODO: Remove hardcoded Service lookup by name "lingo".

		models, err := a.scaler.ListAllModels(ctx)
		if err != nil {
			log.Printf("Failed to list models: %v", err)
			continue
		}

		for _, m := range models {
			if m.Spec.AutoscalingDisabled {
				log.Printf("Model %q has autoscaling disabled, skipping", m.Name)
				continue
			}

			endpointAddrs := a.resolver.GetAllAddresses(m.Name)
			if len(endpointAddrs) == 0 {
				log.Printf("No ready endpoints found for model: %s, skipping", m.Name)
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
			avg := a.getMovingAvgQueueSize(m.Name)
			avg.Next(agg.CurrentRequests())
			flt := avg.Calculate()
			normalized := flt / float64(*m.Spec.TargetRequests)
			ceil := math.Ceil(normalized)
			log.Printf("Average for model %q: %v/%v (normalized ceil: %v), current requests: %v, history: %v", m.Name, flt, *m.Spec.TargetRequests, ceil, agg.CurrentRequests(), avg.History())
			a.scaler.Scale(ctx, &m, int32(ceil), a.cfg.RequiredConsecutiveScaleDowns(*m.Spec.ScaleDownDelaySeconds))
		}
	}
}

func (r *Autoscaler) getMovingAvgQueueSize(deploymentName string) *movingaverage.Simple {
	r.movingAvgByModelMtx.Lock()
	a, ok := r.movingAvgByModel[deploymentName]
	if !ok {
		a = movingaverage.NewSimple(make([]float64, r.cfg.AverageWindowCount()))
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
