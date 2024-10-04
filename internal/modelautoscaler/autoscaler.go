package modelautoscaler

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/substratusai/kubeai/internal/config"
	"github.com/substratusai/kubeai/internal/endpoints"
	"github.com/substratusai/kubeai/internal/leader"
	"github.com/substratusai/kubeai/internal/modelscaler"
	"github.com/substratusai/kubeai/internal/movingaverage"
)

func New(
	leaderElection *leader.Election,
	scaler *modelscaler.ModelScaler,
	resolver *endpoints.Resolver,
	cfg config.ModelAutoscaling,
	metricsPort int,
	fixedSelfMetricAddrs []string,
) *Autoscaler {
	a := &Autoscaler{
		leaderElection:       leaderElection,
		scaler:               scaler,
		resolver:             resolver,
		movingAvgByModel:     map[string]*movingaverage.Simple{},
		cfg:                  cfg,
		metricsPort:          metricsPort,
		fixedSelfMetricAddrs: fixedSelfMetricAddrs,
	}
	return a
}

// Autoscaler is responsible for making continuous adjustments to
// the scale of the backend. It is not responsible for scale-from-zero.
// Each deployment has its own autoscaler.
type Autoscaler struct {
	leaderElection *leader.Election

	scaler   *modelscaler.ModelScaler
	resolver *endpoints.Resolver

	cfg config.ModelAutoscaling

	metricsPort int

	movingAvgByModelMtx sync.Mutex
	movingAvgByModel    map[string]*movingaverage.Simple

	fixedSelfMetricAddrs []string
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

		log.Println("Is leader, autoscaling")

		// TODO: Remove hardcoded Service lookup by name "lingo".

		models, err := a.scaler.ListAllModels(ctx)
		if err != nil {
			log.Printf("Failed to list models: %v", err)
			continue
		}

		agg := newMetricsAggregation()
		var selfAddrs []string
		if len(a.fixedSelfMetricAddrs) > 0 {
			selfAddrs = a.fixedSelfMetricAddrs
		} else {
			selfIPs := a.resolver.GetSelfIPs()
			for _, ip := range selfIPs {
				selfAddrs = append(selfAddrs, fmt.Sprintf("%s:%d", ip, a.metricsPort))
			}
		}
		if len(selfAddrs) == 0 {
			log.Println("Unable to resolve KubeAI addresses, skipping")
			continue
		}
		if err := aggregateAllMetrics(agg, selfAddrs, "/metrics"); err != nil {
			log.Printf("Failed to aggregate metrics: %v", err)
			continue
		}

		for _, m := range models {
			if m.Spec.AutoscalingDisabled {
				log.Printf("Model %q has autoscaling disabled, skipping", m.Name)
				continue
			}

			activeRequests, ok := agg.activeRequestsByModel[m.Name]
			if !ok {
				log.Printf("No metrics found for model %q, skipping", m.Name)
				continue
			}
			var activeRequestSum int64
			for _, req := range activeRequests {
				activeRequestSum += req
			}

			avg := a.getMovingAvgActiveReqPerModel(m.Name)
			avg.Next(float64(activeRequestSum))
			avgActiveRequests := avg.Calculate()
			normalized := avgActiveRequests / float64(*m.Spec.TargetRequests)
			ceil := math.Ceil(normalized)
			log.Printf("Calculated target replicas for model %q: ceil(%v/%v) = %v, current requests: %v, history: %v",
				m.Name, avgActiveRequests, *m.Spec.TargetRequests, ceil, activeRequests, avg.History())
			a.scaler.Scale(ctx, &m, int32(ceil), a.cfg.RequiredConsecutiveScaleDowns(*m.Spec.ScaleDownDelaySeconds))
		}
	}
}

func (r *Autoscaler) getMovingAvgActiveReqPerModel(model string) *movingaverage.Simple {
	r.movingAvgByModelMtx.Lock()
	a, ok := r.movingAvgByModel[model]
	if !ok {
		a = movingaverage.NewSimple(make([]float64, r.cfg.AverageWindowCount()))
		r.movingAvgByModel[model] = a
	}
	r.movingAvgByModelMtx.Unlock()
	return a
}
