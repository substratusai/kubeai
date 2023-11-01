package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const lingoDomain = "lingo.substratus.ai"

func NewDeploymentManager(mgr ctrl.Manager) (*DeploymentManager, error) {
	r := &DeploymentManager{}
	r.Client = mgr.GetClient()
	r.scalers = map[string]*scaler{}
	r.modelToDeployment = map[string]string{}
	if err := r.SetupWithManager(mgr); err != nil {
		return nil, err
	}
	return r, nil
}

type DeploymentManager struct {
	client.Client

	Namespace string

	ScaleDownPeriod time.Duration

	scalersMtx sync.Mutex
	scalers    map[string]*scaler

	modelToDeploymentMtx sync.RWMutex
	modelToDeployment    map[string]string
}

func (r *DeploymentManager) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Complete(r)
}

func (r *DeploymentManager) AtLeastOne(model string) {
	r.getScaler(model).AtLeastOne()
}

func (r *DeploymentManager) SetDesiredScale(model string, n int32) {
	r.getScaler(model).SetDesiredScale(n)
}

func (r *DeploymentManager) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var d appsv1.Deployment
	if err := r.Get(ctx, req.NamespacedName, &d); err != nil {
		return ctrl.Result{}, fmt.Errorf("get: %w", err)
	}

	if ann := d.GetAnnotations(); ann != nil {
		modelCSV, ok := ann[lingoDomain+"/models"]
		if !ok {
			return ctrl.Result{}, nil
		}
		models := strings.Split(modelCSV, ",")
		if len(models) == 0 {
			return ctrl.Result{}, nil
		}
		for _, model := range models {
			r.setModelMapping(strings.TrimSpace(model), d.Name)
		}
	}

	var scale autoscalingv1.Scale
	if err := r.SubResource("scale").Get(ctx, &d, &scale); err != nil {
		return ctrl.Result{}, fmt.Errorf("get scale: %w", err)
	}

	model := req.Name
	r.getScaler(model).UpdateState(
		scale.Spec.Replicas,
		getAnnotationInt32(d.GetAnnotations(), lingoDomain+"/min-replicas", 0),
		getAnnotationInt32(d.GetAnnotations(), lingoDomain+"/max-replicas", 3),
	)

	return ctrl.Result{}, nil
}

func (r *DeploymentManager) getScaler(model string) *scaler {
	r.scalersMtx.Lock()
	b, ok := r.scalers[model]
	if !ok {
		b = newScaler(r.ScaleDownPeriod, r.scaleFunc(context.TODO(), model))
		r.scalers[model] = b
	}
	r.scalersMtx.Unlock()
	return b
}

func (r *DeploymentManager) scaleFunc(ctx context.Context, model string) func(int32) error {
	return func(n int32) error {
		log.Printf("Scaling model %q: %v", model, n)

		req := types.NamespacedName{Namespace: r.Namespace, Name: model}
		var d appsv1.Deployment
		if err := r.Get(ctx, req, &d); err != nil {
			return fmt.Errorf("get: %w", err)
		}

		var scale autoscalingv1.Scale
		if err := r.SubResource("scale").Get(ctx, &d, &scale); err != nil {
			return fmt.Errorf("get scale: %w", err)
		}

		if scale.Spec.Replicas != n {
			scale.Spec.Replicas = n

			if err := r.SubResource("scale").Update(ctx, &d, client.WithSubResourceBody(&scale)); err != nil {
				return fmt.Errorf("update scale: %w", err)
			}
		}

		return nil
	}
}

func (r *DeploymentManager) setModelMapping(model, deployment string) {
	r.modelToDeploymentMtx.Lock()
	r.modelToDeployment[model] = deployment
	r.modelToDeploymentMtx.Unlock()
}

func (r *DeploymentManager) ResolveDeployment(model string) (string, bool) {
	r.modelToDeploymentMtx.RLock()
	deploy, ok := r.modelToDeployment[model]
	r.modelToDeploymentMtx.RUnlock()
	return deploy, ok
}

func getAnnotationInt32(ann map[string]string, key string, defaultValue int32) int32 {
	if ann == nil {
		return defaultValue
	}

	str, ok := ann[key]
	if !ok {
		return defaultValue
	}

	value, err := strconv.Atoi(str)
	if err != nil {
		log.Printf("parsing annotation as int: %v", err)
		return defaultValue
	}

	return int32(value)
}
