package modelcontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
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

func (r *ModelReconciler) calculatePodPlan(log logr.Logger, allPods *corev1.PodList, model *kubeaiv1.Model, modelConfig ModelConfig) *podPlan {

	var details []string

	var desiredReplicas int32
	// NOTE: Replicas could be nil if autoscaling is disabled.
	if model.Spec.Replicas != nil {
		desiredReplicas = *model.Spec.Replicas
	}

	var podForModel func(*kubeaiv1.Model, ModelConfig, string) *corev1.Pod
	switch model.Spec.Engine {
	case kubeaiv1.OLlamaEngine:
		podForModel = r.oLlamaPodForModel
	case kubeaiv1.FasterWhisperEngine:
		podForModel = r.fasterWhisperPodForModel
	case kubeaiv1.InfinityEngine:
		podForModel = r.infinityPodForModel
	default:
		podForModel = r.vLLMPodForModel
	}

	var (
		toCreate []*corev1.Pod
		toDelete []*corev1.Pod
	)

	podMap := make(map[string]corev1.Pod, len(allPods.Items))
	readyByHash := make(map[string]int32)
	totalsByHash := make(map[string]int32)
	newestCreationTimestampByHash := make(map[string]metav1.Time)
	for _, pod := range allPods.Items {
		podMap[pod.Name] = pod

		hash := k8sutils.GetLabel(&pod, kubeaiv1.PodHashLabel)
		if k8sutils.PodIsReady(&pod) {
			readyByHash[hash]++
		}
		totalsByHash[hash]++

		newest := newestCreationTimestampByHash[hash]
		if pod.CreationTimestamp.After(newest.Time) {
			newestCreationTimestampByHash[hash] = pod.CreationTimestamp
		}
	}

	type podPair struct {
		current  *corev1.Pod
		expected *corev1.Pod
	}
	var readyAndNeedsRecreation []podPair

	nameForPodIndex := func(i int32) string {
		return fmt.Sprintf("model-%s-%d", model.Name, i)
	}

	whatToDoAboutExpectedPod := func(i int32) (pair podPair, create, recreate bool) {
		name := nameForPodIndex(i)
		defer delete(podMap, name)

		expectedPod := podForModel(model, modelConfig, name)
		pair.expected = expectedPod
		// TODO: If collisions become an issue, we can add a Model.Status.CollisionCount (new field)
		// to the PodHash call.
		expectedPodHash := k8sutils.PodHash(expectedPod.Spec)
		k8sutils.SetLabel(expectedPod, kubeaiv1.PodHashLabel, expectedPodHash)

		currentPod, ok := podMap[name]
		if ok {
			// Already exists, compare to see if it needs to be recreated.
			// TODO: Consider implementing a rollout strategy.
			// Right now, we just delete and recreate all Pods.
			// This is probably OK for now since the proxy will handle
			// the queuing of incoming requests until the new Pods are ready.
			// (albeit with some failures).
			if k8sutils.GetLabel(&currentPod, kubeaiv1.PodHashLabel) != expectedPodHash {
				// Recreate the Pod.
				// needRecreation = append(needRecreation, podPair{current: &currentPod, expected: expectedPod})
				pair.current = &currentPod
				recreate = true
			}
		} else {
			// toCreate = append(toCreate, expectedPod)
			create = true
		}

		// Remove the Pod from the map so we can delete any remaining Pods.
		//delete(podMap, name)
		return
	}

	// Loop through a deterministic list of Pod names and create or delete Pods as needed.
	// Because the client used to list Pods is a cache client, the exact number of Pods
	// returned may not be accurate which is why we don't just compare total count of Pods
	// to desired replicas.
	for i := int32(0); i < desiredReplicas; i++ {
		pair, create, recreate := whatToDoAboutExpectedPod(i)
		if create {
			toCreate = append(toCreate, pair.expected)
		} else if recreate {
			if k8sutils.PodIsReady(pair.current) {
				readyAndNeedsRecreation = append(readyAndNeedsRecreation, pair)
			} else {
				// Recreate the Pod immediately if it is not ready.
				toDelete = append(toDelete, pair.current)
				toCreate = append(toCreate, pair.expected)
			}
		}
	}

	/*
		   Pod update rollout sequence:

		   - Detect rollout needed
		   - Ensure replicas+1 pods exist

		   NOTE: Any old-hash unready Pods will be deleted and recreated immediately.

		   ... Re-reconcile ...

		   - Delete & recreate 1 old pod if EITHER:
		      A. All pods with new hash are ready
			    OR
		      B. Most recently created pod with new hash is unready and time-since-creation > rolloutProgressWaitPeriod

		   Post rollout:

		   - Remove the `+1` replica if EITHER:
		      A. All pods with the new hash are ready.
			    OR
			  B. The newest pod has a time-since-creation > rolloutProgressWaitPeriod

		   TODO: Consider implementing a naming convention for Pods that allows
		   the surge-Pod to be kept around instead of deleted after the rollout
		   is complete.
		   This could probably be done by including the hash in the Pod names.
		   Note: This is probably not a huge deal since Model rollouts should be
		   infrequent.
	*/

	if len(readyAndNeedsRecreation) > 0 {
		// Account for the fact that we need to need replicas + 1 Pods while updating.
		// replicas + 1, i.e. index [desiredReplicas]
		pair, create, recreate := whatToDoAboutExpectedPod(desiredReplicas)
		if create {
			toCreate = append(toCreate, pair.expected)
		} else if recreate {
			// The +1 Pod needs to be recreated - must be from an
			// older update that never fully rolled out.
			toDelete = append(toDelete, pair.current)
			toCreate = append(toCreate, pair.expected)
		} else {
			// We must have already created the +1 Pod for the newest
			// rollout. Check to see if we should progress to recreate
			// an old Pod.

			lastRecreationIdx := len(readyAndNeedsRecreation) - 1
			hash := k8sutils.GetLabel(readyAndNeedsRecreation[lastRecreationIdx].expected, kubeaiv1.PodHashLabel)

			// Use the following conditions to determine if we should proceeed
			// with the rollout. We use the old-ready-Pods count as the bar for
			// moving forward because it is likely to be consistent as it has
			// had a long time for the cache to update:
			//
			// (all new ready Pods) == (all expected Pods) - (all old ready Pods)
			allReady := readyByHash[hash] == (desiredReplicas+1)-int32(len(readyAndNeedsRecreation))
			//
			// We add a wait period because it is possible that there might not
			// be enough capacity to run all Pods in a replicas+1 scenario.
			timeSince := time.Since(newestCreationTimestampByHash[hash].Time)
			timeoutReached := timeSince > r.ModelRollouts.PodReadinessWaitPeriod.Duration
			if allReady || timeoutReached {
				log.Info("Rollout progressing", "allReady", allReady, "timeoutReached", timeoutReached)
				// All Pods with the new hash are ready OR
				// We are done waiting for the most recent Pod to be ready.
				// Delete the oldest Pod with the old hash and create a new Pod.
				toDelete = append(toDelete, readyAndNeedsRecreation[lastRecreationIdx].current)
				toCreate = append(toCreate, readyAndNeedsRecreation[lastRecreationIdx].expected)
			}
		}
	} else {
		// Make sure we don't prematurely delete the excess Pod after a rollout is complete
		// by checking that all other replicas are Ready first.
		// NOTE: This point could also be reached on scale-down.
		var hash string
		if len(allPods.Items) > 0 && desiredReplicas > 0 {
			hash = k8sutils.GetLabel(&allPods.Items[0], kubeaiv1.PodHashLabel)

			timeSince := time.Since(newestCreationTimestampByHash[hash].Time)

			if int(readyByHash[hash]) != int(desiredReplicas+1) &&
				timeSince < r.ModelRollouts.PodReadinessWaitPeriod.Duration {
				// Avoid deleting the +1 Pod by removing it from the map
				// before all excess Pods in this map are added to the toDelete list.
				name := nameForPodIndex(desiredReplicas)
				pod, ok := podMap[name]
				if ok {
					// Only keep Pod around if it is Ready.
					if k8sutils.PodIsReady(&pod) {
						delete(podMap, name)
					}
				}
			}
		}
	}

	// Delete the remaining pods.
	for _, pod := range podMap {
		toDelete = append(toDelete, &pod)
	}

	return &podPlan{
		model:    model,
		toCreate: toCreate,
		toDelete: toDelete,
		details:  details,
	}
}

type podPlan struct {
	model    *kubeaiv1.Model
	toCreate []*corev1.Pod
	toDelete []*corev1.Pod
	details  []string
}

func (pp *podPlan) execute(ctx context.Context, client client.Client, scheme *runtime.Scheme) error {
	log := log.FromContext(ctx)

	// Delete before create to avoid unnecessary Node scale-ups.
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

	return nil
}
