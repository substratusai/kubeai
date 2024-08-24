package modelcontroller

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

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

	var existingSparePod corev1.Pod
	var spareExists bool
	existingReplicaPodMap := make(map[string]corev1.Pod)
	readyReplicasByHash := make(map[string]int32)
	newestReplicaPodByHash := make(map[string]corev1.Pod)

	sortPodsByIndex(allPods.Items)

	const spareSuffix = "spare"

	for _, pod := range allPods.Items {
		// Omit the spare Pod from calculations.
		if strings.HasSuffix(pod.Name, spareSuffix) {
			existingSparePod = pod
			spareExists = true
			continue
		}

		existingReplicaPodMap[pod.Name] = pod

		hash := k8sutils.GetLabel(&pod, kubeaiv1.PodHashLabel)
		if k8sutils.PodIsReady(&pod) {
			readyReplicasByHash[hash]++
		}

		newest := newestReplicaPodByHash[hash]
		if pod.CreationTimestamp.After(newest.CreationTimestamp.Time) {
			newestReplicaPodByHash[hash] = pod
		}
	}

	type podPair struct {
		current  *corev1.Pod
		expected *corev1.Pod
	}
	var readyAndNeedsRecreation []podPair

	nameForPodIndex := func(i int32, spare bool) string {
		if spare {
			return fmt.Sprintf("model-%s-%s", model.Name, spareSuffix)
		}
		return fmt.Sprintf("model-%s-%d", model.Name, i)
	}

	whatToDoAboutExpectedPod := func(i int32, spare bool) (pair podPair, create, recreate bool) {
		name := nameForPodIndex(i, spare)
		defer delete(existingReplicaPodMap, name)

		expectedPod := podForModel(model, modelConfig, name)
		pair.expected = expectedPod
		// TODO: If hash collisions become an issue, we can add a Model.Status.CollisionCount (new field)
		// to the PodHash call.
		expectedPodHash := k8sutils.PodHash(expectedPod.Spec)
		k8sutils.SetLabel(expectedPod, kubeaiv1.PodHashLabel, expectedPodHash)
		k8sutils.SetLabel(expectedPod, kubeaiv1.PodIndexLabel, fmt.Sprintf("%d", i))

		var currentPod corev1.Pod
		var exists bool
		if spare {
			currentPod = existingSparePod
			exists = spareExists
		} else {
			currentPod, exists = existingReplicaPodMap[name]
		}
		if exists {
			// Already exists, compare to see if it needs to be recreated.
			if k8sutils.GetLabel(&currentPod, kubeaiv1.PodHashLabel) != expectedPodHash {
				// Mark that the Pod needs to be recreated.
				pair.current = &currentPod
				recreate = true
			}
		} else {
			create = true
		}

		return
	}

	// Loop through a deterministic list of Pod names and create or delete Pods as needed.
	// Because the client used to list Pods is a cache client, the exact number of Pods
	// returned may not be accurate which is why we don't just compare total count of Pods
	// to desired replicas.
	for i := int32(0); i < desiredReplicas; i++ {
		pair, create, recreate := whatToDoAboutExpectedPod(i, false)
		if create {
			details = append(details, fmt.Sprintf("Creating Pod %q replica", pair.expected.Name))
			toCreate = append(toCreate, pair.expected)
		} else if recreate {
			if k8sutils.PodIsReady(pair.current) {
				readyAndNeedsRecreation = append(readyAndNeedsRecreation, pair)
			} else {
				// Recreate the Pod immediately if it is not ready.
				details = append(details, fmt.Sprintf("Recreating outdated Pod %q immediately because it is not ready", pair.current.Name))
				toDelete = append(toDelete, pair.current)
				toCreate = append(toCreate, pair.expected)
			}
		}
	}

	/*
		   Pod update rollout sequence:

		   - Detect rollout needed
		   - Ensure "-spare" pod exists

		   NOTE: Any old-hash unready Pods will be deleted and recreated immediately.

		   ... Re-reconcile ...

		   - Delete & recreate 1 old pod (lowest index first) if EITHER:
		      A. The most recent pod with the new hash is ready.
			    OR
		      B. The most recent pod with the new hash has taken too long to become ready.

		   TODO: Consider implementing a naming convention for Pods that allows
		   the surge-Pod to be kept around instead of deleted after the rollout
		   is complete.
		   This could probably be done by including the hash in the Pod names.
		   Note: This is probably not a huge deal since Model rollouts should be
		   infrequent.
	*/

	if len(readyAndNeedsRecreation) > 0 {
		spare, createSpare, recreateSpare := whatToDoAboutExpectedPod(0, true)
		if createSpare {
			details = append(details, fmt.Sprintf("Creating spare Pod %q because Pod updates need to be rolled out", spare.expected.Name))
			toCreate = append(toCreate, spare.expected)
		} else if recreateSpare {
			details = append(details, fmt.Sprintf("Recreating spare Pod %q because it is outdated", spare.current.Name))
			// The spare Pod needs to be recreated - must be from an
			// older update that never fully rolled out.
			toDelete = append(toDelete, spare.current)
			toCreate = append(toCreate, spare.expected)
		} else if spareExists {
			// We must have already created the spare Pod for the newest
			// rollout. Check to see if we should progress to recreate
			// an old Pod.

			hash := k8sutils.GetLabel(readyAndNeedsRecreation[0].expected, kubeaiv1.PodHashLabel)

			lastUpdatedPod, nonSpareUpdatedPodExists := newestReplicaPodByHash[hash]
			if !nonSpareUpdatedPodExists {
				lastUpdatedPod = existingSparePod
			}

			// We add a wait period because it is possible that there might not
			// be enough capacity to run all Pods in a replicas+1 scenario.
			//
			// NOTE: When checking the age of the Pod, we are relying on a
			// synced cache. This is not guaranteed to be accurate, but it is likely
			// due to the sleep in the Reconcile loop after Pod changes are made.
			timeSince := time.Since(lastUpdatedPod.CreationTimestamp.Time)
			timeoutReached := timeSince > r.ModelRollouts.PodReadinessWaitPeriod.Duration
			newestUpdatedPodReady := k8sutils.PodIsReady(&lastUpdatedPod)

			if newestUpdatedPodReady || timeoutReached {
				if newestUpdatedPodReady {
					details = append(details, fmt.Sprintf("Pod %q is ready, rollout progressing by replacing Pod %q", lastUpdatedPod.Name, readyAndNeedsRecreation[0].current.Name))
				} else if timeoutReached {
					details = append(details, fmt.Sprintf("Timed out (%s) waiting for Pod %q to be ready, rollout progressing by replacing Pod %q", timeSince.String(), lastUpdatedPod.Name, readyAndNeedsRecreation[0].current.Name))
				}
				// All Pods with the new hash are ready OR
				// We are done waiting for the most recent Pod to be ready.
				toDelete = append(toDelete, readyAndNeedsRecreation[0].current)
				toCreate = append(toCreate, readyAndNeedsRecreation[0].expected)
			}
		}
	} else {
		// Account for deleting the spare after a rollout is complete.
		if spareExists {
			hash := k8sutils.GetLabel(&existingSparePod, kubeaiv1.PodHashLabel)

			spareNotReady := !k8sutils.PodIsReady(&existingSparePod)
			allReplicasReady := readyReplicasByHash[hash] == desiredReplicas
			zeroReplicas := desiredReplicas == 0

			// NOTE: When checking the age of the Pod, we are relying on a
			// synced cache. This is not guaranteed to be accurate, but it is likely
			// due to the sleep in the Reconcile loop after Pod changes are made.
			var timeSince time.Duration
			var timeout bool
			if newest, ok := newestReplicaPodByHash[hash]; ok {
				timeSince = time.Since(newest.CreationTimestamp.Time)
				timeout = timeSince > r.ModelRollouts.PodReadinessWaitPeriod.Duration
			}
			if zeroReplicas || spareNotReady || allReplicasReady || timeout {
				if zeroReplicas {
					details = append(details, fmt.Sprintf("Rollout complete, no replicas desired, deleting spare Pod %q", existingSparePod.Name))
				} else if spareNotReady {
					details = append(details, fmt.Sprintf("Rollout complete, spare Pod %q not ready, deleting spare", existingSparePod.Name))
				} else if allReplicasReady {
					details = append(details, fmt.Sprintf("Rollout complete, all Pods ready, deleting spare Pod %q", existingSparePod.Name))
				} else if timeout {
					details = append(details, fmt.Sprintf("Rollout complete, timed out (%s) waiting for all Pods to be ready, deleting spare Pod %q", timeSince.String(), existingSparePod.Name))
				}
				// All Pods with the new hash are ready or timeout has been reached.
				// Delete the spare.
				toDelete = append(toDelete, &existingSparePod)
			}
		}
	}

	// Delete the excess pods.
	for _, pod := range existingReplicaPodMap {
		details = append(details, fmt.Sprintf("Deleting excess Pod %q", pod.Name))
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
				return changed, fmt.Errorf("deleting pod: %w", err)
			}
		}
		changed = true
	}

	for _, pod := range pp.toCreate {
		if err := ctrl.SetControllerReference(pp.model, pod, scheme); err != nil {
			return changed, fmt.Errorf("setting controller reference: %w", err)
		}
		log.Info("Creating Pod", "podName", pod.Name)
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

func sortPodsByIndex(pods []corev1.Pod) {
	sort.Slice(pods, func(i, j int) bool {
		return k8sutils.PodIndex(&pods[i]) < k8sutils.PodIndex(&pods[j])
	})
}
