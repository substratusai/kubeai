package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewScalerManager(mgr ctrl.Manager) (*ScalerManager, error) {
	r := &ScalerManager{}
	r.Client = mgr.GetClient()
	r.scalers = map[string]*scaler{}
	if err := r.SetupWithManager(mgr); err != nil {
		return nil, err
	}
	return r, nil
}

type ScalerManager struct {
	client.Client

	Namespace string

	ScaleDownPeriod time.Duration

	scalersMtx sync.Mutex
	scalers    map[string]*scaler
}

func (r *ScalerManager) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Complete(r)
}

func (r *ScalerManager) AtLeastOne(model string) {
	r.getScaler(model).AtLeastOne()
}

func (r *ScalerManager) SetDesiredScale(model string, n int32) {
	r.getScaler(model).SetDesiredScale(n)
}

func (r *ScalerManager) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var d appsv1.Deployment
	if err := r.Get(ctx, req.NamespacedName, &d); err != nil {
		return ctrl.Result{}, fmt.Errorf("get: %w", err)
	}

	//labels := d.GetLabels()
	//if labels == nil {
	//	return ctrl.Result{}, nil
	//}
	//if labels["lingo"] == "enabled" {
	//	return ctrl.Result{}, nil
	//}

	var scale autoscalingv1.Scale
	if err := r.SubResource("scale").Get(ctx, &d, &scale); err != nil {
		return ctrl.Result{}, fmt.Errorf("get scale: %w", err)
	}

	// TODO: Use a label.
	model := req.Name
	r.getScaler(model).SetCurrentScale(scale.Spec.Replicas)

	return ctrl.Result{}, nil
}

func (r *ScalerManager) getScaler(model string) *scaler {
	r.scalersMtx.Lock()
	b, ok := r.scalers[model]
	if !ok {
		b = newScaler(r.ScaleDownPeriod, r.scaleFunc(context.TODO(), model))
		r.scalers[model] = b
	}
	r.scalersMtx.Unlock()
	return b
}

func (r *ScalerManager) scaleFunc(ctx context.Context, model string) func(int32) error {
	return func(n int32) error {
		log.Printf("Scaling model %q: %v", model, n)

		// TODO: Use model label.
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
