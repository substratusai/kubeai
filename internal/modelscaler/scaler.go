package modelscaler

import (
	"context"
	"fmt"
	"log"
	"sync"

	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO: Change to ModelClient

type ModelScaler struct {
	client                        client.Client
	namespace                     string
	consecutiveScaleDownsMtx      sync.RWMutex
	consecutiveScaleDowns         map[string]int
	requiredConsecutiveScaleDowns int
}

func NewModelScaler(client client.Client, namespace string, requiredConsecutiveScaleDowns int) *ModelScaler {
	return &ModelScaler{client: client, namespace: namespace, consecutiveScaleDowns: map[string]int{}, requiredConsecutiveScaleDowns: requiredConsecutiveScaleDowns}
}

func (s *ModelScaler) ModelExists(ctx context.Context, model string) (bool, error) {
	if err := s.client.Get(ctx, types.NamespacedName{Name: model, Namespace: s.namespace}, &kubeaiv1.Model{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *ModelScaler) ListAllModels(ctx context.Context) ([]string, error) {
	models := &kubeaiv1.ModelList{}
	if err := s.client.List(ctx, models, client.InNamespace(s.namespace)); err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}

	var names []string
	for _, model := range models.Items {
		names = append(names, model.Name)
	}
	return names, nil
}

func (s *ModelScaler) ScaleAtLeastOneReplica(ctx context.Context, model string) error {
	obj := &kubeaiv1.Model{}
	if err := s.client.Get(ctx, types.NamespacedName{Namespace: s.namespace, Name: model}, obj); err != nil {
		return fmt.Errorf("get scale: %w", err)
	}

	if obj.Spec.Replicas != nil && *obj.Spec.Replicas == 0 && obj.Spec.MaxReplicas > 0 {
		scale := &autoscalingv1.Scale{
			Spec: autoscalingv1.ScaleSpec{Replicas: 1},
		}
		if err := s.client.SubResource("scale").Update(ctx, obj, client.WithSubResourceBody(scale)); err != nil {
			return fmt.Errorf("update scale: %w", err)
		}
	}

	return nil
}

func (s *ModelScaler) Scale(ctx context.Context, model string, replicas int32) error {
	obj := &kubeaiv1.Model{}
	if err := s.client.Get(ctx, types.NamespacedName{Namespace: s.namespace, Name: model}, obj); err != nil {
		return fmt.Errorf("get scale: %w", err)
	}

	replicas = enforceReplicaBounds(replicas, obj)

	var existingReplicas int32 = 0
	if obj.Spec.Replicas != nil {
		existingReplicas = *obj.Spec.Replicas
	}

	if existingReplicas > replicas {
		// Scale down
		s.consecutiveScaleDownsMtx.RLock()
		consec := s.consecutiveScaleDowns[model]
		s.consecutiveScaleDownsMtx.RUnlock()
		if consec < s.requiredConsecutiveScaleDowns {
			log.Printf("model %s has %d consecutive scale downs (< %d), not scaling down yet", model, consec, s.requiredConsecutiveScaleDowns)
			s.consecutiveScaleDownsMtx.Lock()
			s.consecutiveScaleDowns[model]++
			s.consecutiveScaleDownsMtx.Unlock()
			return nil
		}
	} else {
		// Scale up or constant scale.
		s.consecutiveScaleDownsMtx.Lock()
		s.consecutiveScaleDowns[model] = 0
		s.consecutiveScaleDownsMtx.Unlock()
	}

	if existingReplicas != replicas {
		log.Printf("scaling model %s from %d to %d replicas", model, existingReplicas, replicas)
		scale := &autoscalingv1.Scale{
			Spec: autoscalingv1.ScaleSpec{Replicas: replicas},
		}
		if err := s.client.SubResource("scale").Update(ctx, obj, client.WithSubResourceBody(scale)); err != nil {
			return fmt.Errorf("update scale: %w", err)
		}
	}

	return nil
}

func enforceReplicaBounds(replicas int32, model *kubeaiv1.Model) int32 {
	if replicas > model.Spec.MaxReplicas {
		return model.Spec.MaxReplicas
	}
	if replicas < model.Spec.MinReplicas {
		return model.Spec.MinReplicas
	}
	return replicas
}
