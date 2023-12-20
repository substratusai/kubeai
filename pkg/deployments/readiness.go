package deployments

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type k8sClient interface {
	List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
}
type Readiness struct {
	ready       atomic.Bool
	k8sClient   k8sClient
	namespace   string
	deployments *Manager
	interval    time.Duration // await deployments loaded on startup
}

func NewReadiness(k8sClient k8sClient, namespace string, deployments *Manager, interval time.Duration) *Readiness {
	return &Readiness{k8sClient: k8sClient, namespace: namespace, deployments: deployments, interval: interval}
}

func (h *Readiness) StateLoaded(r *http.Request) error {
	if h.ready.Load() {
		return nil
	}
	return errors.New("not ready")
}

func (h *Readiness) WatchModelBackendsLoaded() {
	for range time.Tick(h.interval) {
		var sliceList appsv1.DeploymentList
		if err := h.k8sClient.List(context.TODO(), &sliceList, client.InNamespace(h.namespace)); err != nil {
			log.Printf("failed to list deployments: %v", err)
			continue
		}
		found := true
	outer:
		for _, v := range sliceList.Items {
			models := getModelsFromAnnotation(v.Annotations)
			if len(models) == 0 {
				continue
			}
			// ensure all models are known to us
			for _, m := range models {
				if _, ok := h.deployments.ResolveDeployment(m); !ok {
					found = false
					break outer
				}
			}
		}

		if found {
			h.ready.Store(true)
			return
		}
	}
}
