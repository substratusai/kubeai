package deployments

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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const lingoDomain = "lingo.substratus.ai"

func NewManager(mgr ctrl.Manager) (*Manager, error) {
	r := &Manager{}
	r.Client = mgr.GetClient()
	r.scalers = map[string]*scaler{}
	r.modelToDeployment = map[string]string{}
	if err := r.SetupWithManager(mgr); err != nil {
		return nil, err
	}
	return r, nil
}

type Manager struct {
	client.Client

	Namespace string

	ScaleDownPeriod time.Duration

	scalersMtx sync.Mutex

	// scalers maps deployment names to scalers
	scalers map[string]*scaler

	modelToDeploymentMtx sync.RWMutex

	// modelToDeployment maps model names to deployment names. A single deployment
	// can serve multiple models.
	modelToDeployment map[string]string
}

func (r *Manager) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Complete(r)
}

func (r *Manager) AtLeastOne(deploymentName string) {
	r.getScaler(deploymentName).AtLeastOne()
}

func (r *Manager) SetDesiredScale(deploymentName string, n int32) {
	r.getScaler(deploymentName).SetDesiredScale(n)
}

func (r *Manager) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var d appsv1.Deployment
	switch err := r.Get(ctx, req.NamespacedName, &d); {
	case apierrors.IsNotFound(err):
		r.removeDeployment(req)
		return ctrl.Result{}, nil
	case err != nil:
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

	deploymentName := req.Name
	r.getScaler(deploymentName).UpdateState(
		scale.Spec.Replicas,
		getAnnotationInt32(d.GetAnnotations(), lingoDomain+"/min-replicas", 0),
		getAnnotationInt32(d.GetAnnotations(), lingoDomain+"/max-replicas", 3),
	)

	return ctrl.Result{}, nil
}

func (r *Manager) removeDeployment(req ctrl.Request) {
	r.scalersMtx.Lock()
	delete(r.scalers, req.Name)
	r.scalersMtx.Unlock()

	r.modelToDeploymentMtx.Lock()
	for model, deployment := range r.modelToDeployment {
		if deployment == req.Name {
			delete(r.modelToDeployment, model)
		}
	}
	r.modelToDeploymentMtx.Unlock()
}

func (r *Manager) getScaler(deploymentName string) *scaler {
	r.scalersMtx.Lock()
	b, ok := r.scalers[deploymentName]
	if !ok {
		b = newScaler(r.ScaleDownPeriod, r.scaleFunc(context.TODO(), deploymentName))
		r.scalers[deploymentName] = b
	}
	r.scalersMtx.Unlock()
	return b
}

func (r *Manager) scaleFunc(ctx context.Context, deploymentName string) func(int32, bool) error {
	return func(n int32, atLeastOne bool) error {
		if atLeastOne {
			log.Printf("Scaling model %q: at-least-one", deploymentName)
		} else {
			log.Printf("Scaling model %q: %v", deploymentName, n)
		}

		req := types.NamespacedName{Namespace: r.Namespace, Name: deploymentName}
		var d appsv1.Deployment
		if err := r.Get(ctx, req, &d); err != nil {
			return fmt.Errorf("get: %w", err)
		}

		var scale autoscalingv1.Scale
		if err := r.SubResource("scale").Get(ctx, &d, &scale); err != nil {
			return fmt.Errorf("get scale: %w", err)
		}

		if atLeastOne {
			if scale.Spec.Replicas == 0 {
				scale.Spec.Replicas = 1
				if err := r.SubResource("scale").Update(ctx, &d, client.WithSubResourceBody(&scale)); err != nil {
					return fmt.Errorf("update scale (from zero): %w", err)
				}
			}
			return nil
		}

		if scale.Spec.Replicas != n {
			scale.Spec.Replicas = n
			if err := r.SubResource("scale").Update(ctx, &d, client.WithSubResourceBody(&scale)); err != nil {
				return fmt.Errorf("update scale (desired): %w", err)
			}
		}

		return nil
	}
}

func (r *Manager) setModelMapping(modelName, deploymentName string) {
	r.modelToDeploymentMtx.Lock()
	r.modelToDeployment[modelName] = deploymentName
	r.modelToDeploymentMtx.Unlock()
}

func (r *Manager) ResolveDeployment(model string) (string, bool) {
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
