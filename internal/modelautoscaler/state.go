package modelautoscaler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newTotalModelState() totalModelState {
	return totalModelState{
		Models:              make(map[string]modelState),
		LastCalculationTime: time.Now(),
	}
}

type totalModelState struct {
	Models              map[string]modelState `json:"models"`
	LastCalculationTime time.Time             `json:"lastCalculationTime"`
}

type modelState struct {
	AverageActiveRequests float64 `json:"averageActiveRequests"`
}

func (a *Autoscaler) loadLastTotalModelState(ctx context.Context) (totalModelState, error) {
	cm := &corev1.ConfigMap{}
	if err := a.k8sClient.Get(ctx, a.stateConfigMapRef, cm); err != nil {
		return totalModelState{}, fmt.Errorf("get ConfigMap %q: %w", a.stateConfigMapRef, err)
	}
	const key = "models"
	jsonState, ok := cm.Data[key]
	if !ok {
		log.Printf("Autoscaler state ConfigMap %q has no key %q, state not loaded", key, a.stateConfigMapRef)
		return totalModelState{}, nil
	}
	tms := totalModelState{}
	if err := json.Unmarshal([]byte(jsonState), &tms); err != nil {
		return totalModelState{}, fmt.Errorf("unmarshalling state: %w", err)
	}
	return tms, nil
}

func (a *Autoscaler) saveTotalModelState(ctx context.Context, state totalModelState) error {
	jsonState, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}
	patch := fmt.Sprintf(`{"data":{"models":%q}}`, string(jsonState))
	if err := a.k8sClient.Patch(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: a.stateConfigMapRef.Namespace,
			Name:      a.stateConfigMapRef.Name,
		},
	}, client.RawPatch(types.StrategicMergePatchType, []byte(patch))); err != nil {
		return fmt.Errorf("patching ConfigMap %q: %w", a.stateConfigMapRef, err)
	}
	return nil
}
