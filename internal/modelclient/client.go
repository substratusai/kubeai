package modelclient

import (
	"context"
	"fmt"
	"sync"

	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ModelClient struct {
	client                   client.Client
	namespace                string
	consecutiveScaleDownsMtx sync.RWMutex
	consecutiveScaleDowns    map[string]int
}

func NewModelClient(client client.Client, namespace string) *ModelClient {
	return &ModelClient{client: client, namespace: namespace, consecutiveScaleDowns: map[string]int{}}
}

// LookupModel checks if a model exists and matches the given label selectors.
func (c *ModelClient) LookupModel(ctx context.Context, model, adapter string, labelSelectors []string) (bool, error) {
	m := &kubeaiv1.Model{}
	if err := c.client.Get(ctx, types.NamespacedName{Name: model, Namespace: c.namespace}, m); err != nil {
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
			if a.Name == adapter {
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

func (s *ModelClient) ListAllModels(ctx context.Context) ([]kubeaiv1.Model, error) {
	models := &kubeaiv1.ModelList{}
	if err := s.client.List(ctx, models, client.InNamespace(s.namespace)); err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}

	return models.Items, nil
}
