package autoscaler

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

	"github.com/substratusai/lingo/pkg/deployments"
	"github.com/substratusai/lingo/pkg/endpoints"
	"github.com/substratusai/lingo/pkg/leader"
	"github.com/substratusai/lingo/pkg/movingaverage"
	"github.com/substratusai/lingo/pkg/queue"
	"github.com/substratusai/lingo/pkg/stats"
)

func New(mgr ctrl.Manager) (*Autoscaler, error) {
	a := &Autoscaler{}
	a.Client = mgr.GetClient()
	a.HTTPClient = &http.Client{}
	a.movingAvgQueueSize = map[string]*movingaverage.Simple{}
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

	LeaderElection *leader.Election

	Deployments *deployments.Manager
	Queues      *queue.Manager
	Endpoints   *endpoints.Manager

	ConcurrencyPerReplica int

	HTTPClient *http.Client

	movingAvgQueueSizeMtx sync.Mutex
	movingAvgQueueSize    map[string]*movingaverage.Simple
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

func (r *Autoscaler) Start() {
	for range time.Tick(r.Interval) {
		if !r.LeaderElection.IsLeader.Load() {
			log.Println("Not leader, doing nothing")
			continue
		}

		log.Println("Calculating scales for all")

		// TODO: Remove hardcoded Service lookup by name "lingo".
		otherLingoEndpoints := r.Endpoints.GetAllHosts("lingo", "stats")

		stats, errs := aggregateStats(stats.Stats{
			ActiveRequests: r.Queues.TotalCounts(),
		}, r.HTTPClient, otherLingoEndpoints)
		if len(errs) != 0 {
			for _, err := range errs {
				log.Printf("Failed to aggregate stats: %v", err)
			}
			continue
		}

		for deploymentName, waitCount := range stats.ActiveRequests {
			log.Println("Is leader, autoscaling")
			avg := r.getMovingAvgQueueSize(deploymentName)
			avg.Next(float64(waitCount))
			flt := avg.Calculate()
			normalized := flt / float64(r.ConcurrencyPerReplica)
			ceil := math.Ceil(normalized)
			log.Printf("Average for deployment: %s: %v (ceil: %v), current wait count: %v", deploymentName, flt, ceil, waitCount)
			r.Deployments.SetDesiredScale(deploymentName, int32(ceil))
		}
	}
}

func (r *Autoscaler) getMovingAvgQueueSize(deploymentName string) *movingaverage.Simple {
	r.movingAvgQueueSizeMtx.Lock()
	a, ok := r.movingAvgQueueSize[deploymentName]
	if !ok {
		a = movingaverage.NewSimple(make([]float64, r.AverageCount))
		r.movingAvgQueueSize[deploymentName] = a
	}
	r.movingAvgQueueSizeMtx.Unlock()
	return a
}

func aggregateStats(s stats.Stats, httpc *http.Client, endpoints []string) (stats.Stats, []error) {
	var errs []error

	for k, v := range s.ActiveRequests {
		log.Printf("Leader active requests for: %v: %v", k, v)
	}

	for _, endpoint := range endpoints {
		fetched, err := getStats(httpc, "http://"+endpoint)
		if err != nil {
			errs = append(errs, fmt.Errorf("getting stats: %v: %w", endpoint, err))
			continue
		}
		for k, v := range fetched.ActiveRequests {
			log.Printf("Aggregating active requests for endpoint: %v: %v: %v", endpoint, k, v)
			s.ActiveRequests[k] = fetched.ActiveRequests[k] + v
		}
	}

	return s, errs
}

func getStats(httpc *http.Client, endpoint string) (stats.Stats, error) {
	var stats stats.Stats

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
