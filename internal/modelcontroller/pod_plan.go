package modelcontroller

import (
	"context"
	"fmt"
	"sort"
	"strings"

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

// calculatePodPlan calculates the Pod plan for the given Model.
// It assumes the list of Pods represents an accurate snapshot of the current state.
// It returns a Pod plan that contains Pods to create and delete.
// If a rollout is required, it will return a Pod plan that:
// - Adds a surge Pod
// - Recreates any out-of-date Pod that is not Ready immediately
// - Waits for all Pods to be Ready before recreating any out-of-date Pods that are Ready
func (r *ModelReconciler) calculatePodPlan(allPods *corev1.PodList, model *kubeaiv1.Model, modelConfig ModelConfig) *podPlan {
	var podForModel *corev1.Pod
	switch model.Spec.Engine {
	case kubeaiv1.OLlamaEngine:
		podForModel = r.oLlamaPodForModel(model, modelConfig)
	case kubeaiv1.FasterWhisperEngine:
		podForModel = r.fasterWhisperPodForModel(model, modelConfig)
	case kubeaiv1.InfinityEngine:
		podForModel = r.infinityPodForModel(model, modelConfig)
	default:
		podForModel = r.vLLMPodForModel(model, modelConfig)
	}
	expectedHash := k8sutils.PodHash(podForModel.Spec)
	podForModel.GenerateName = fmt.Sprintf("model-%s-%s-", model.Name, expectedHash)
	k8sutils.SetLabel(podForModel, kubeaiv1.PodHashLabel, expectedHash)

	var (
		toCreate      []*corev1.Pod
		toDelete      []*corev1.Pod
		toRemain      []*corev1.Pod
		details       []string
		upToDate      []corev1.Pod // pods with new hash
		outOfDate     []corev1.Pod // pods with old hash
		readyUpToDate int          // count of ready pods with new hash
	)

	// Categorize pods
	for _, pod := range allPods.Items {
		if k8sutils.GetLabel(&pod, kubeaiv1.PodHashLabel) == expectedHash {
			upToDate = append(upToDate, pod)
			if k8sutils.PodIsReady(&pod) {
				readyUpToDate++
			}
		} else {
			outOfDate = append(outOfDate, pod)
		}
	}

	// Calculate base desired replicas
	var baseDesiredReplicas int32
	if model.Spec.Replicas != nil {
		baseDesiredReplicas = *model.Spec.Replicas
	}

	// Create pods if needed
	neededPods := int(baseDesiredReplicas) - len(upToDate)
	if neededPods > 0 {
		details = append(details, fmt.Sprintf("Creating %d pods to meet desired replicas", neededPods))
		for i := 0; i < neededPods; i++ {
			toCreate = append(toCreate, podForModel.DeepCopy())
		}
	}

	// Handle pod deletion in three cases:
	// 1. Delete excess pods (for scale down)
	// 2. Delete unready out-of-date pods immediately
	// 3. Delete ready out-of-date pods when we have enough ready up-to-date pods

	// First, handle scale down by marking excess up-to-date pods for deletion
	if len(upToDate) > int(baseDesiredReplicas) {
		// Sort pods so we delete unready ones first
		sortPodsByDeletionOrder(upToDate, expectedHash)
		excess := len(upToDate) - int(baseDesiredReplicas)
		for i := 0; i < excess; i++ {
			pod := &upToDate[i]
			details = append(details, fmt.Sprintf("Deleting excess pod %q", pod.Name))
			toDelete = append(toDelete, pod)
		}
	}

	// Then, delete unready out-of-date pods immediately
	for i := range outOfDate {
		pod := &outOfDate[i]
		if !k8sutils.PodIsReady(pod) {
			details = append(details, fmt.Sprintf("Deleting unready out-of-date pod %q", pod.Name))
			toDelete = append(toDelete, pod)
		}
	}

	// Finally, delete ready out-of-date pods if we have enough ready up-to-date pods
	if readyUpToDate >= int(baseDesiredReplicas) {
		for i := range outOfDate {
			pod := &outOfDate[i]
			if k8sutils.PodIsReady(pod) && !sliceContainsPod(toDelete, pod) {
				details = append(details, fmt.Sprintf("Deleting ready out-of-date pod %q", pod.Name))
				toDelete = append(toDelete, pod)
			}
		}
	}

	// Calculate which pods will remain
	for i := range upToDate {
		pod := &upToDate[i]
		if !sliceContainsPod(toDelete, pod) {
			toRemain = append(toRemain, pod)
		}
	}

	return &podPlan{
		model:    model,
		toCreate: toCreate,
		toDelete: toDelete,
		toRemain: toRemain,
		details:  details,
	}
}

type podPlan struct {
	model    *kubeaiv1.Model
	toCreate []*corev1.Pod
	toDelete []*corev1.Pod
	toRemain []*corev1.Pod
	details  []string
}

func (pp *podPlan) containsActions() bool {
	return len(pp.toCreate) > 0 || len(pp.toDelete) > 0
}

// execute returns true if a Pod was created or deleted.
func (pp *podPlan) execute(ctx context.Context, client client.Client, scheme *runtime.Scheme) (bool, error) {
	log := log.FromContext(ctx)

	detailsCSV := strings.Join(pp.details, ", ")
	log.Info("Executing Pod plan", "modelName", pp.model.Name, "details", detailsCSV)

	var changed bool

	// Delete before create to avoid unnecessary Node scale-ups.
	for _, pod := range pp.toDelete {
		if err := client.Delete(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: pod.Namespace,
				Name:      pod.Name,
			},
		}); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Pod already deleted", "podName", pod.Name)
			} else {
				return changed, fmt.Errorf("deleting pod: %w", err)
			}
		}
		changed = true
	}

	for _, pod := range pp.toCreate {
		if err := ctrl.SetControllerReference(pp.model, pod, scheme); err != nil {
			return changed, fmt.Errorf("setting controller reference: %w", err)
		}
		if err := client.Create(ctx, pod, k8sutils.DefaultCreateOptions()); err != nil {
			if apierrors.IsAlreadyExists(err) {
				log.Info("Pod already exists", "podName", pod.Name)
			} else {
				return changed, fmt.Errorf("creating pod: %w", err)
			}
		}
		changed = true
	}

	return changed, nil
}

// sortPodsByDeletionOrder ensures Pods that are to be deleted/recreated
// first are lower index.
func sortPodsByDeletionOrder(pods []corev1.Pod, expectedHash string) {
	sort.SliceStable(pods, func(i, j int) bool {
		// Not ready Pods should be deleted first.
		iReady := k8sutils.PodIsReady(&pods[i])
		jReady := k8sutils.PodIsReady(&pods[j])
		if iReady != jReady {
			return !iReady
		}

		// Unscheduled Pods should be deleted first.
		iScheduled := k8sutils.PodIsScheduled(&pods[i])
		jScheduled := k8sutils.PodIsScheduled(&pods[j])
		if iScheduled != jScheduled {
			return !iScheduled
		}

		// Delete Pods that are from older hash first
		iHash := k8sutils.GetLabel(&pods[i], kubeaiv1.PodHashLabel)
		jHash := k8sutils.GetLabel(&pods[j], kubeaiv1.PodHashLabel)
		if iHash != jHash {
			return iHash != expectedHash
		}

		// Younger Pods should be deleted first.
		iCreationTime := pods[i].CreationTimestamp.Time
		jCreationTime := pods[j].CreationTimestamp.Time
		return iCreationTime.After(jCreationTime)
	})
}

func sliceContainsPod(pods []*corev1.Pod, pod *corev1.Pod) bool {
	for _, p := range pods {
		if p.Name == pod.Name && p.Namespace == pod.Namespace {
			return true
		}
	}
	return false
}
