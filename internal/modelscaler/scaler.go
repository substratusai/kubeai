package modelscaler

import (
	"context"
	"fmt"
	"log"
	"sync"

	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ModelScaler struct {
	client                   client.Client
	namespace                string
	consecutiveScaleDownsMtx sync.RWMutex
	consecutiveScaleDowns    map[string]int
}

func NewModelScaler(client client.Client, namespace string) *ModelScaler {
	return &ModelScaler{client: client, namespace: namespace, consecutiveScaleDowns: map[string]int{}}
}

// LookupModel checks if a model exists and matches the given label selectors.
func (s *ModelScaler) LookupModel(ctx context.Context, model, adapter string, labelSelectors []string) (bool, error) {
	m := &kubeaiv1.Model{}
	if err := s.client.Get(ctx, types.NamespacedName{Name: model, Namespace: s.namespace}, m); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	modelLabels := m.GetLabels()
	if modelLabels == nil {
		modelLabels = map[string]string{}
	}
	for _, sel := range labelSelectors {
		parsedSel, err := labels.Parse(sel)
		if err != nil {
			return false, fmt.Errorf("parse label selector: %w", err)
		}
		if !parsedSel.Matches(labels.Set(modelLabels)) {
			return false, nil
		}
	}

	if adapter != "" {
		adapterFound := false
		for _, a := range m.Spec.Adapters {
			if a.ID == adapter {
				adapterFound = true
				break
			}
		}
		if !adapterFound {
			return false, nil
		}
	}

	return true, nil
}

func (s *ModelScaler) ListAllModels(ctx context.Context) ([]kubeaiv1.Model, error) {
	models := &kubeaiv1.ModelList{}
	if err := s.client.List(ctx, models, client.InNamespace(s.namespace)); err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}

	return models.Items, nil
}

func (s *ModelScaler) ScaleAtLeastOneReplica(ctx context.Context, model string) error {
	obj := &kubeaiv1.Model{}
	if err := s.client.Get(ctx, types.NamespacedName{Namespace: s.namespace, Name: model}, obj); err != nil {
		return fmt.Errorf("get scale: %w", err)
	}

	if obj.Spec.AutoscalingDisabled {
		return nil
	}

	replicas := int32(0)
	if obj.Spec.Replicas != nil {
		replicas = *obj.Spec.Replicas
	}

	if replicas == 0 && !obj.Spec.AutoscalingDisabled {
		scale := &autoscalingv1.Scale{
			Spec: autoscalingv1.ScaleSpec{Replicas: 1},
		}
		if err := s.client.SubResource("scale").Update(ctx, obj, client.WithSubResourceBody(scale)); err != nil {
			return fmt.Errorf("update scale: %w", err)
		}
	}

	return nil
}

// Scale scales the model to the desired number of replicas, enforcing the min and max replica bounds.
// Model should have .Spec defined before calling Scale().
func (s *ModelScaler) Scale(ctx context.Context, model *kubeaiv1.Model, replicas int32, requiredConsecutiveScaleDowns int) error {
	//obj := &kubeaiv1.Model{}
	//if err := s.client.Get(ctx, types.NamespacedName{Namespace: s.namespace, Name: model}, obj); err != nil {
	//	return fmt.Errorf("get scale: %w", err)
	//}

	replicas = enforceReplicaBounds(replicas, model)

	var existingReplicas int32 = 0
	if model.Spec.Replicas != nil {
		existingReplicas = *model.Spec.Replicas
	}

	if existingReplicas > replicas {
		// Scale down
		s.consecutiveScaleDownsMtx.RLock()
		consec := s.consecutiveScaleDowns[model.Name]
		s.consecutiveScaleDownsMtx.RUnlock()
		if consec < requiredConsecutiveScaleDowns {
			log.Printf("model %s has %d consecutive scale downs (< %d), not scaling down yet", model.Name, consec, requiredConsecutiveScaleDowns)
			s.consecutiveScaleDownsMtx.Lock()
			s.consecutiveScaleDowns[model.Name]++
			s.consecutiveScaleDownsMtx.Unlock()
			return nil
		}
	} else {
		// Scale up or constant scale.
		s.consecutiveScaleDownsMtx.Lock()
		s.consecutiveScaleDowns[model.Name] = 0
		s.consecutiveScaleDownsMtx.Unlock()
	}

	if existingReplicas != replicas {
		log.Printf("scaling model %s from %d to %d replicas", model.Name, existingReplicas, replicas)
		scale := &autoscalingv1.Scale{
			Spec: autoscalingv1.ScaleSpec{Replicas: replicas},
		}
		if err := s.client.SubResource("scale").Update(ctx, model, client.WithSubResourceBody(scale)); err != nil {
			return fmt.Errorf("update scale: %w", err)
		}
	}

	return nil
}

func enforceReplicaBounds(replicas int32, model *kubeaiv1.Model) int32 {
	max := model.Spec.MaxReplicas
	min := model.Spec.MinReplicas
	if max != nil {
		if replicas > *max {
			return *max
		}
	}
	if replicas < min {
		return min
	}
	return replicas
}
