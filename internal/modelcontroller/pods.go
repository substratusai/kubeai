package modelcontroller

import (
	"context"
	"fmt"

	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
	"github.com/substratusai/kubeai/internal/k8sutils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *ModelReconciler) calculatePodPlan(allPods *corev1.PodList, model *kubeaiv1.Model, modelConfig ModelConfig) *podPlan {
	var desiredReplicas int32
	// Replicas could be nil if autoscaling is disabled.
	if model.Spec.Replicas != nil {
		desiredReplicas = *model.Spec.Replicas
	}

	// TODO: Take into account Pods that are in a deletion state.

	var podForModel func(*kubeaiv1.Model, ModelConfig, string) *corev1.Pod
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

	podMap := make(map[string]corev1.Pod, len(allPods.Items))
	for _, pod := range allPods.Items {
		podMap[pod.Name] = pod
	}

	// Loop through a deterministic list of Pod names and create or delete Pods as needed.
	// Because the client used to list Pods is a cache client, the exact number of Pods
	// returned may not be accurate which is why we don't just compare total count of Pods
	// to desired replicas.
	for i := int32(0); i < desiredReplicas; i++ {
		name := fmt.Sprintf("model-%s-%d", model.Name, i)

		expectedPod := podForModel(model, modelConfig, name)
		// TODO: If collisions become an issue, we can add a Model.Status.CollisionCount (new field)
		// to the PodHash call.
		expectedPodHash := k8sutils.PodHash(expectedPod.Spec, nil)
		k8sutils.ApplyLabel(expectedPod, kubeaiv1.PodSpecHashLabel, expectedPodHash)

		currentPod, ok := podMap[name]
		if ok {
			// Already exists
			// TODO: Compare Pod spec to desired spec and recreate if necessary.
			if podMap[name].Labels[kubeaiv1.PodSpecHashLabel] != expectedPodHash {
				toDelete = append(toDelete, &currentPod)
				toCreate = append(toCreate, expectedPod)
			}
		} else {
			toCreate = append(toCreate, expectedPod)
		}

		// Remove the Pod from the map so we can delete any remaining Pods.
		delete(podMap, name)
	}

	// Delete the remaining pods.
	for _, pod := range podMap {
		toDelete = append(toDelete, &pod)
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
			if apierrors.IsAlreadyExists(err) {
				log.Info("Pod already exists", "podName", pod.Name)
			} else {
				return fmt.Errorf("creating pod: %w", err)
			}
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
			if apierrors.IsNotFound(err) {
				log.Info("Pod already deleted", "podName", pod.Name)
			} else {
				return fmt.Errorf("deleting pod: %w", err)
			}
		}
	}

	return nil
}
