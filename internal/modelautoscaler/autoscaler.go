package modelautoscaler

import (
	"context"
	"errors"
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
	metricsPort int,
) *Autoscaler {
	a := &Autoscaler{
		leaderElection:   leaderElection,
		scaler:           scaler,
		resolver:         resolver,
		movingAvgByModel: map[string]*movingaverage.Simple{},
		cfg:              cfg,
		metricsPort:      metricsPort,
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

	metricsPort int

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

		agg := newMetricsAggregation()
		if err := aggregateAllMetrics(agg, a.resolver.GetSelfIPs(), a.metricsPort, "/metrics"); err != nil {
			log.Printf("Failed to aggregate metrics: %v", err)
			continue
		}

		for _, m := range models {
			if m.Spec.AutoscalingDisabled {
				log.Printf("Model %q has autoscaling disabled, skipping", m.Name)
				continue
			}

			fmt.Println("AGG", agg)

			activeRequests, ok := agg.activeRequestsByModel[m.Name]
			if !ok {
				log.Printf("No metrics found for model %q, skipping", m.Name)
				continue
			}

			modelEndpoints := a.resolver.GetAllAddresses(m.Name)
			if len(modelEndpoints) == 0 {
				log.Printf("No endpoints found for model %q, skipping", m.Name)
				continue
			}

			log.Println("Is leader, autoscaling")
			avg := a.getMovingAvg(m.Name)
			avg.Next(float64(activeRequests))
			avgActiveRequests := avg.Calculate()
			normalized := avgActiveRequests / float64(*m.Spec.TargetRequests)
			ceil := math.Ceil(normalized)
			log.Printf("Calculated target replicas for model %q: ceil(%v/%v) = %v, current requests: %v, history: %v",
				m.Name, avgActiveRequests, *m.Spec.TargetRequests, ceil, activeRequests, avg.History())
			a.scaler.Scale(ctx, &m, int32(ceil), a.cfg.RequiredConsecutiveScaleDowns(*m.Spec.ScaleDownDelaySeconds))
		}
	}
}

func (r *Autoscaler) getMovingAvg(deploymentName string) *movingaverage.Simple {
	r.movingAvgByModelMtx.Lock()
	a, ok := r.movingAvgByModel[deploymentName]
	if !ok {
		a = movingaverage.NewSimple(make([]float64, r.cfg.AverageWindowCount()))
		r.movingAvgByModel[deploymentName] = a
	}
	r.movingAvgByModelMtx.Unlock()
	return a
}

func aggregateAllMetrics(agg *metricsAggregation, endpointIPs []string, port int, path string) (err error) {
	for _, ip := range endpointIPs {
		if e := scrapeAndAggregateMetrics(agg, fmt.Sprintf("http://%s:%d%s", ip, port, path)); e != nil {
			err = errors.Join(err, e)
		}
	}

	return err
}
