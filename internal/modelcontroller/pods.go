package modelcontroller

import (
	"context"
	"fmt"
	"math"

	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
	"github.com/substratusai/kubeai/internal/k8sutils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *ModelReconciler) calculatePodPlan(allPods *corev1.PodList, model *kubeaiv1.Model, modelConfig ModelConfig) *podPlan {
	var scaleDesired int32
	// Replicas could be nil if autoscaling is disabled.
	if model.Spec.Replicas != nil {
		scaleDesired = *model.Spec.Replicas
	}
	scaleActual := int32(len(allPods.Items))
	scaleDiff := scaleActual - scaleDesired
	scaleDiffAbs := int32(math.Abs(float64(scaleDiff)))

	// TODO: Take into account Pods that are in a deletion state.

	var podForModel func(*kubeaiv1.Model, ModelConfig, int32) *corev1.Pod
	switch model.Spec.Engine {
	case kubeaiv1.OLlamaEngine:
		podForModel = r.oLlamaPodForModel
	case kubeaiv1.FasterWhisperEngine:
		podForModel = r.fasterWhisperPodForModel
	default:
		podForModel = r.vLLMPodForModel
	}

	var (
		toCreate []*corev1.Pod
		toDelete []*corev1.Pod
	)

	switch {
	case scaleDiff == 0:
		// At correct scale.
	case scaleDiff < 0:
		for i := int32(0); i < scaleDiffAbs; i++ {
			toCreate = append(toCreate, podForModel(model, modelConfig, scaleActual+i))
		}
	case scaleDiff > 0:
		toDeleteCount := scaleDiffAbs
		for _, pod := range allPods.Items {
			if toDeleteCount == 0 {
				break
			}
			toDelete = append(toDelete, &pod)
			toDeleteCount--
		}
	}

	return &podPlan{
		model:    model,
		toCreate: toCreate,
		toDelete: toDelete,
	}
}

type podPlan struct {
	model    *kubeaiv1.Model
	toCreate []*corev1.Pod
	toDelete []*corev1.Pod
}

func (pp *podPlan) execute(ctx context.Context, client client.Client, scheme *runtime.Scheme) error {
	log := log.FromContext(ctx)

	for _, pod := range pp.toCreate {
		if err := ctrl.SetControllerReference(pp.model, pod, scheme); err != nil {
			return fmt.Errorf("setting controller reference: %w", err)
		}
		log.Info("Creating Pod", "podName", pod.Name)
		if err := client.Create(ctx, pod, k8sutils.DefaultCreateOptions()); err != nil {
			return fmt.Errorf("creating pod: %w", err)
		}
	}

	for _, pod := range pp.toDelete {
		log.Info("Deleting Pod", "podName", pod.Name)
		if err := client.Delete(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: pod.Namespace,
				Name:      pod.Name,
			},
		}); err != nil {
			return fmt.Errorf("deleting pod: %w", err)
		}
	}

	return nil
}
