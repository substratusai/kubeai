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
	"github.com/substratusai/kubeai/internal/loadbalancer"
	"github.com/substratusai/kubeai/internal/modelclient"
	"github.com/substratusai/kubeai/internal/movingaverage"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(
	ctx context.Context,
	k8sClient client.Client,
	leaderElection *leader.Election,
	modelClient *modelclient.ModelClient,
	resolver *loadbalancer.LoadBalancer,
	cfg config.ModelAutoscaling,
	metricsPort int,
	stateConfigMapRef types.NamespacedName,
	fixedSelfMetricAddrs []string,
) (*Autoscaler, error) {
	a := &Autoscaler{
		k8sClient:            k8sClient,
		leaderElection:       leaderElection,
		modelClient:          modelClient,
		resolver:             resolver,
		movingAvgByModel:     map[string]*movingaverage.Simple{},
		cfg:                  cfg,
		metricsPort:          metricsPort,
		stateConfigMapRef:    stateConfigMapRef,
		fixedSelfMetricAddrs: fixedSelfMetricAddrs,
	}

	// Load preloaded moving averages from the last known state.
	//
	// NOTE: There is an edge case where this might load an empty state:
	// 1. KubeAI running with active requests.
	// 2. KubeAI saves request states.
	// 3. KubeAI is restarted.
	// 4. Leader election occurs and KubeAI saves request states before any requests come in (empty state).
	// 5. KubeAI restarted again (before requests come in).
	// 6. New KubeAI instance starts and loads the empty state.
	// 7. Leader election occurs and KubeAI scales down Model replicas.
	//
	lastModelState, err := a.loadLastTotalModelState(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading last state of models: %w", err)
	}
	log.Printf("Loaded last state of models: %d total, last calculated on %s", len(lastModelState.Models), lastModelState.LastCalculationTime)
	for m, s := range lastModelState.Models {
		// Preload moving averages with the last known state.
		// If the last known state was 5.5, the preloaded moving average
		// would look like [5.5, 5.5, 5.5, ...].
		preloaded := newPrefilledFloat64Slice(a.cfg.AverageWindowCount(), s.AverageActiveRequests)
		a.movingAvgByModel[m] = movingaverage.NewSimple(preloaded)
		log.Printf("Preloaded moving average for model %q with %v", m, preloaded)
	}

	return a, nil
}

// Autoscaler is responsible for making continuous adjustments to
// the scale of the backend. It is not responsible for scale-from-zero.
// Each deployment has its own autoscaler.
type Autoscaler struct {
	k8sClient client.Client

	stateConfigMapRef types.NamespacedName

	leaderElection *leader.Election

	modelClient *modelclient.ModelClient
	resolver    *loadbalancer.LoadBalancer

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

		models, err := a.modelClient.ListAllModels(ctx)
		if err != nil {
			log.Printf("Failed to list models: %v", err)
			continue
		}

		nextModelState := newTotalModelState()

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

		log.Printf("Aggregating metrics from KubeAI addresses %v", selfAddrs)
		agg := newMetricsAggregation()
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
			log.Printf("Calculated target replicas for model %q: ceil(%v/%v) = %v, current requests: sum(%v) = %v, history: %v",
				m.Name, avgActiveRequests, *m.Spec.TargetRequests, ceil, activeRequests, activeRequestSum, avg.History())
			a.modelClient.Scale(ctx, &m, int32(ceil), a.cfg.RequiredConsecutiveScaleDowns(*m.Spec.ScaleDownDelaySeconds))

			nextModelState.Models[m.Name] = modelState{
				AverageActiveRequests: avgActiveRequests,
			}
		}

		if err := a.saveTotalModelState(ctx, nextModelState); err != nil {
			log.Printf("Failed to save model state: %v", err)
		}
	}
}

func (a *Autoscaler) getMovingAvgActiveReqPerModel(model string) *movingaverage.Simple {
	a.movingAvgByModelMtx.Lock()
	avg, ok := a.movingAvgByModel[model]
	if !ok {
		avg = movingaverage.NewSimple(make([]float64, a.cfg.AverageWindowCount()))
		a.movingAvgByModel[model] = avg
	}
	a.movingAvgByModelMtx.Unlock()
	return avg
}

func newPrefilledFloat64Slice(length int, value float64) []float64 {
	s := make([]float64, length)
	for i := range s {
		s[i] = value
	}
	return s
}
