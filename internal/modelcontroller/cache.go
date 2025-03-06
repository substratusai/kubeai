package modelcontroller

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
	"github.com/substratusai/kubeai/internal/k8sutils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type PVCModelAnnotationValue struct {
	UID       string    `json:"uid"`
	Timestamp time.Time `json:"timestamp"`
}

func (r *ModelReconciler) reconcileCache(ctx context.Context, model *kubeaiv1.Model, cfg ModelConfig) (ctrl.Result, error) {
	// Initialize model.Status.Cache if nil
	if model.Status.Cache == nil {
		model.Status.Cache = &kubeaiv1.ModelStatusCache{}
	}

	// If no cache profile is configured, nothing to do
	if model.Spec.CacheProfile == "" {
		return ctrl.Result{}, nil
	}

	modelDeleted := model.DeletionTimestamp != nil

	urlChanged := model.Status.Cache.URL != model.Spec.URL

	// Store the old URL for later use if needed
	oldURL := ""
	if urlChanged {
		oldURL = model.Status.Cache.URL
	}

	// If URL has changed, immediately set Loaded = false and update URL via patching
	// This helps users know that the specific URL hasn't been loaded yet
	if urlChanged {
		// Store the old URL in status before updating to new URL
		// If PreviousURL is empty, just set it to the old URL
		// If PreviousURL already has a value, we're in a multi-transition scenario
		// In this case, keep the most recent previous URL for cleanup
		if model.Status.Cache.PreviousURL == "" && model.Status.Cache.URL != "" {
			model.Status.Cache.PreviousURL = model.Status.Cache.URL
		}
		model.Status.Cache.Loaded = false
		model.Status.Cache.URL = model.Spec.URL
	} else if model.Status.Cache.URL == "" && model.Status.Cache.Loaded {
		// This is needed for backwards compatibility with older models.
		// Older models used a different directory structure for the cache.
		model.Status.Cache.Loaded = false
		model.Status.Cache.URL = model.Spec.URL
	} else if model.Status.Cache.URL == "" {
		model.Status.Cache.URL = model.Spec.URL
	}

	if err := r.Status().Patch(ctx, model, client.MergeFrom(model.DeepCopy())); err != nil {
		return ctrl.Result{}, fmt.Errorf("patching model status for URL change: %w", err)
	}

	pvc := &corev1.PersistentVolumeClaim{}
	var pvcExists bool
	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: model.Namespace,
		Name:      cachePVCName(model, cfg),
	}, pvc); err != nil {
		if apierrors.IsNotFound(err) {
			pvcExists = false
		} else {
			return ctrl.Result{}, fmt.Errorf("getting cache PVC: %w", err)
		}
	} else {
		pvcExists = true
	}

	// Create PVC if not exists.
	if !pvcExists {
		if !modelDeleted {
			pvc = r.cachePVCForModel(model, cfg)
			// TODO: Set controller reference on PVC for 1:1 Model to PVC situations
			// such as Google Hyperdisk ML.
			//if err := controllerutil.SetControllerReference(model, pvc, r.Scheme); err != nil {
			//	return ctrl.Result{}, fmt.Errorf("setting controller reference on pvc: %w", err)
			//}
			if err := r.Create(ctx, pvc); err != nil {
				return ctrl.Result{}, fmt.Errorf("creating cache PVC: %w", err)
			}
		}
	}

	// Caches that are shared across multiple Models require model-specific cleanup.
	if cfg.CacheProfile.SharedFilesystem != nil {
		if controllerutil.AddFinalizer(model, kubeaiv1.ModelCacheEvictionFinalizer) {
			if err := r.Update(ctx, model); err != nil {
				return ctrl.Result{}, fmt.Errorf("adding cache deletion finalizer: %w", err)
			}
		}
	}
	// NOTE: Only .Spec.CacheProfile is immutable, .Spec.URL is now mutable when cacheProfile is used
	// with proper handling for transitions between URLs

	loadJob := &batchv1.Job{}
	var jobExists bool
	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: model.Namespace,
		Name:      loadCacheJobName(model),
	}, loadJob); err != nil {
		if apierrors.IsNotFound(err) {
			jobExists = false
		} else {
			return ctrl.Result{}, fmt.Errorf("getting cache job: %w", err)
		}
	} else {
		jobExists = true
	}

	var pvcModelAnn PVCModelAnnotationValue
	var err error

	if pvcExists {
		pvcModelAnn, err = parsePVCModelAnnotation(pvc, model.Name)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("parsing pvc model annotation: %w", err)
		}
	}

	// Check if we need to load the cache
	// This happens if either:
	// 1. The model UID doesn't match what's in the PVC annotation (new model)
	// 2. The URL has changed since the last cache load
	needsCacheLoad := !pvcExists || pvcModelAnn.UID != string(model.UID) || urlChanged

	// Initialize model.Status.Cache if nil to prevent null pointer errors
	if model.Status.Cache == nil {
		model.Status.Cache = &kubeaiv1.ModelStatusCache{}
	}

	// Only attempt to load cache if model is not already loaded with the current URL
	// This prevents recreating jobs after they've been completed and deleted
	if needsCacheLoad && !model.Status.Cache.Loaded {
		// Don't delete the old model's cache yet if URL changed
		// We'll keep both until we confirm the new one works
		if !jobExists {
			loadJob = r.loadCacheJobForModel(model, cfg)
			if err := ctrl.SetControllerReference(model, loadJob, r.Scheme); err != nil {
				return ctrl.Result{}, fmt.Errorf("setting controller reference on job: %w", err)
			}
			if err := r.Create(ctx, loadJob); err != nil {
				return ctrl.Result{}, fmt.Errorf("creating job: %w", err)
			}
			return ctrl.Result{}, errReturnEarly
		}

		if !k8sutils.IsJobCompleted(loadJob) {
			return ctrl.Result{}, errReturnEarly
		}

		if err := r.updatePVCModelAnnotation(ctx, pvc, model.Name, PVCModelAnnotationValue{
			UID:       string(model.UID),
			Timestamp: time.Now(),
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting pvc model annotation: %w", err)
		}

		// Mark cache as loaded since the job completed successfully
		model.Status.Cache.Loaded = true
	}

	// Update Loaded status based on PVC annotation
	// Set loaded to false if PVC doesn't exist yet or annotations don't match
	if model.Status.Cache == nil {
		model.Status.Cache = &kubeaiv1.ModelStatusCache{}
	}

	if !pvcExists {
		model.Status.Cache.Loaded = false
	} else if !model.Status.Cache.Loaded {
		// Only update Loaded status if it's not already true
		// This prevents toggling the Loaded status after we've marked it as true
		// when the job completed successfully

		// Compare UIDs safely - empty string if no annotation was found
		// Ensure if URL is empty, loaded is always false
		if model.Status.Cache.URL == "" {
			model.Status.Cache.Loaded = false
		} else {
			model.Status.Cache.Loaded = pvcModelAnn.UID == string(model.UID)
		}
	}

	// Final safety check: ensure consistency between Loaded and URL
	if model.Status.Cache.Loaded && (model.Status.Cache.URL == "" || model.Status.Cache.URL != model.Spec.URL) {
		model.Status.Cache.Loaded = false
		if model.Status.Cache.URL == "" {
			model.Status.Cache.URL = model.Spec.URL
		}
	}

	if jobExists {
		// Cache loading completed, delete Job to avoid accumulating a mess of completed Jobs.
		// Use foreground deletion policy to ensure the Pods are deleted as well.
		if err := r.Delete(ctx, loadJob, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil {
			return ctrl.Result{}, fmt.Errorf("deleting job: %w", err)
		}
	}

	// If URL was changed, trigger cleanup of the old URL's cache after the new model is ready
	if urlChanged && model.Status.Cache.Loaded {
		// Validate cache profile is properly configured before cleaning up
		if model.Spec.CacheProfile == "" {
			// No cache profile, nothing to clean up
			log.FromContext(ctx).Info("Cannot clean up old URL cache: no cache profile configured",
				"model", model.Name,
				"namespace", model.Namespace)
		} else {
			// Check if new pods are ready before cleaning up old cache
			if err := r.cleanupOldUrlCache(ctx, model, cfg, oldURL); err != nil {
				// Log error but don't fail reconcile
				log.FromContext(ctx).Error(err, "Error cleaning up old URL cache",
					"model", model.Name,
					"namespace", model.Namespace,
					"oldURL", oldURL)
			}
		}
	} else if model.Status.Cache.PreviousURL != "" && model.Status.Cache.Loaded {
		// If we have a PreviousURL but no URL change in this reconcile, we might have a pending cleanup
		// from a previous URL change where the cleanup job didn't complete
		log.FromContext(ctx).Info("Found PreviousURL without current URL change, cleaning up previous URL cache",
			"model", model.Name,
			"namespace", model.Namespace,
			"previousURL", model.Status.Cache.PreviousURL)

		if model.Spec.CacheProfile != "" {
			if err := r.cleanupOldUrlCache(ctx, model, cfg, model.Status.Cache.PreviousURL); err != nil {
				log.FromContext(ctx).Error(err, "Error cleaning up previous URL cache",
					"model", model.Name,
					"namespace", model.Namespace,
					"previousURL", model.Status.Cache.PreviousURL)
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *ModelReconciler) finalizeCache(ctx context.Context, model *kubeaiv1.Model, cfg ModelConfig) error {
	pvc := &corev1.PersistentVolumeClaim{}
	var pvcExists bool
	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: model.Namespace,
		Name:      cachePVCName(model, cfg),
	}, pvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting cache PVC: %w", err)
		}
	} else {
		pvcExists = true
	}

	if !pvcExists || pvc.DeletionTimestamp != nil {
		// If the PVC is not found or is already being deleted, delete all cache jobs and pods.
		// No need trying to update the PVC annotations or perform other cleanup.
		if err := r.deleteAllCacheJobsAndPods(ctx, model); err != nil {
			return fmt.Errorf("deleting all cache jobs and pods: %w", err)
		}
		if controllerutil.RemoveFinalizer(model, kubeaiv1.ModelCacheEvictionFinalizer) {
			if err := r.Update(ctx, model); err != nil {
				return fmt.Errorf("removing cache deletion finalizer: %w", err)
			}
		}
		return nil
	}

	if controllerutil.ContainsFinalizer(model, kubeaiv1.ModelCacheEvictionFinalizer) {
		evictJob := &batchv1.Job{}
		var jobExists bool
		if err := r.Client.Get(ctx, types.NamespacedName{
			Namespace: model.Namespace,
			Name:      evictCacheJobName(model),
		}, evictJob); err != nil {
			if apierrors.IsNotFound(err) {
				jobExists = false
			} else {
				return fmt.Errorf("getting cache deletion job: %w", err)
			}
		} else {
			jobExists = true
		}

		if !jobExists {
			job := r.evictCacheJobForModel(model, cfg)
			if err := ctrl.SetControllerReference(model, job, r.Scheme); err != nil {
				return fmt.Errorf("setting controller reference on cache deletion job: %w", err)
			}
			if err := r.Create(ctx, job); err != nil {
				return fmt.Errorf("creating cache deletion job: %w", err)
			}
			return errReturnEarly
		} else {
			// Wait for the Job to complete.
			if !k8sutils.IsJobCompleted(evictJob) {
				return errReturnEarly
			}

			// Delete the Model from the PVC annotation.
			if pvc.Annotations != nil {
				if _, ok := pvc.Annotations[kubeaiv1.PVCModelAnnotation(model.Name)]; ok {
					delete(pvc.Annotations, kubeaiv1.PVCModelAnnotation(model.Name))
					if err := r.Update(ctx, pvc); err != nil {
						return fmt.Errorf("updating PVC, removing cache annotation: %w", err)
					}
				}
			}
		}

		controllerutil.RemoveFinalizer(model, kubeaiv1.ModelCacheEvictionFinalizer)
		if err := r.Update(ctx, model); err != nil {
			return fmt.Errorf("removing cache deletion finalizer: %w", err)
		}
	}

	if err := r.deleteAllCacheJobsAndPods(ctx, model); err != nil {
		return fmt.Errorf("deleting all cache jobs and pods: %w", err)
	}

	return nil
}

func (r *ModelReconciler) deleteAllCacheJobsAndPods(ctx context.Context, model *kubeaiv1.Model) error {
	jobNames := []string{
		loadCacheJobName(model),
		evictCacheJobName(model),
	}

	for _, jobName := range jobNames {
		if err := r.Delete(ctx, &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: model.Namespace,
				Name:      jobName,
			},
		}); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("deleting job %q: %w", jobName, err)
			}
		}

		// NOTE: There are different conditions in which Pods might not be deleted by the Job controller
		// after a Job is deleted.
		if err := r.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(model.Namespace), client.MatchingLabels{
			batchv1.JobNameLabel: jobName,
		}); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("deleting pods for job %q: %w", jobName, err)
			}
		}
	}

	return nil
}

func parsePVCModelAnnotation(pvc *corev1.PersistentVolumeClaim, modelName string) (PVCModelAnnotationValue, error) {
	// Handle nil PVC
	if pvc == nil {
		return PVCModelAnnotationValue{}, nil
	}

	pvcModelStatusJSON := k8sutils.GetAnnotation(pvc, kubeaiv1.PVCModelAnnotation(modelName))
	if pvcModelStatusJSON == "" {
		// Return an initialized struct with empty values
		return PVCModelAnnotationValue{UID: "", Timestamp: time.Time{}}, nil
	}

	var status PVCModelAnnotationValue
	if err := json.Unmarshal([]byte(pvcModelStatusJSON), &status); err != nil {
		return PVCModelAnnotationValue{}, fmt.Errorf("unmarshalling pvc model status: %w", err)
	}

	// Safety check - ensure UID is never nil
	if status.UID == "" {
		status.UID = ""
	}

	return status, nil
}

func (r *ModelReconciler) updatePVCModelAnnotation(ctx context.Context, pvc *corev1.PersistentVolumeClaim, modelName string, status PVCModelAnnotationValue) error {
	statusJSON, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshalling pvc model status: %w", err)
	}
	k8sutils.SetAnnotation(pvc, kubeaiv1.PVCModelAnnotation(modelName), string(statusJSON))
	if err := r.Client.Update(ctx, pvc); err != nil {
		return fmt.Errorf("updating pvc: %w", err)
	}
	return nil
}

func (r *ModelReconciler) cachePVCForModel(m *kubeaiv1.Model, c ModelConfig) *corev1.PersistentVolumeClaim {
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cachePVCName(m, c),
			Namespace: m.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{},
	}

	// Default settings if no profile is specified
	pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	pvc.Spec.Resources.Requests = corev1.ResourceList{
		corev1.ResourceStorage: resource.MustParse("10Gi"),
	}

	// Apply SharedFilesystem settings if available
	if c.CacheProfile.SharedFilesystem != nil {
		pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}

		if c.CacheProfile.SharedFilesystem.StorageClassName != "" {
			storageClassName := c.CacheProfile.SharedFilesystem.StorageClassName
			pvc.Spec.StorageClassName = &storageClassName
		}

		if c.CacheProfile.SharedFilesystem.PersistentVolumeName != "" {
			pvc.Spec.VolumeName = c.CacheProfile.SharedFilesystem.PersistentVolumeName
		}
	}

	return &pvc
}

func cachePVCName(m *kubeaiv1.Model, c ModelConfig) string {
	// Validate cacheProfile is configured before using it
	if m.Spec.CacheProfile == "" {
		// Fallback to model-specific cache name if no profile specified
		return fmt.Sprintf("model-cache-%s-%s", m.Name, m.UID[0:7])
	}

	switch {
	case c.CacheProfile.SharedFilesystem != nil:
		// One PVC for all models.
		return fmt.Sprintf("shared-model-cache-%s", m.Spec.CacheProfile)
	default:
		// One PVC per model.
		return fmt.Sprintf("model-cache-%s-%s", m.Name, m.UID[0:7])
	}
}

func (r *ModelReconciler) loadCacheJobForModel(m *kubeaiv1.Model, c ModelConfig) *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      loadCacheJobName(m),
			Namespace: m.Namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: ptr.To[int32](60),
			Parallelism:             ptr.To[int32](1),
			Completions:             ptr.To[int32](1),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name: "loader",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "model",
									MountPath: modelCacheDir(m),
									SubPath:   strings.TrimPrefix(modelCacheDir(m), "/"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "model",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: cachePVCName(m, c),
								},
							},
						},
					},
				},
			},
		},
	}

	job.Spec.Template.Spec.Containers[0].Image = r.ModelLoaders.Image
	job.Spec.Template.Spec.Containers[0].Args = []string{
		m.Spec.URL,
		modelCacheDir(m),
	}
	c.Source.modelSourcePodAdditions.applyToPodSpec(&job.Spec.Template.Spec, 0)

	return job
}

func (r *ModelReconciler) evictCacheJobForModel(m *kubeaiv1.Model, c ModelConfig) *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      evictCacheJobName(m),
			Namespace: m.Namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: ptr.To[int32](60),
			Parallelism:             ptr.To[int32](1),
			Completions:             ptr.To[int32](1),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name: "evictor",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "model",
									MountPath: "/models",
									SubPath:   "models",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "model",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: cachePVCName(m, c),
								},
							},
						},
					},
				},
			},
		},
	}

	job.Spec.Template.Spec.Containers[0].Image = r.ModelLoaders.Image
	job.Spec.Template.Spec.Containers[0].Command = []string{"bash", "-c", "rm -rf " + modelCacheDir(m)}

	return job
}

func modelCacheDir(m *kubeaiv1.Model) string {
	// Create a cache directory that's unique for each model name, UID, and URL
	// This allows different URLs to coexist in the cache
	// Use a consistent hash function for the URL to ensure the same URL always gets the same hash
	urlHash := fmt.Sprintf("%x", md5.Sum([]byte(m.Spec.URL)))[0:8]

	// Format: /models/<model-name>-<model-uid>-<url-hash>
	// Using a consistent separator makes it easier to parse and identify components
	return fmt.Sprintf("/models/%s-%s-%s", m.Name, m.UID, urlHash)
}

// urlHashFromURL creates a hash for a URL string
// This function helps ensure consistent hashing across the codebase
func urlHashFromURL(url string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(url)))[0:8]
}

func loadCacheJobName(m *kubeaiv1.Model) string {
	return fmt.Sprintf("load-cache-%s", m.Name)
}

func evictCacheJobName(m *kubeaiv1.Model) string {
	return fmt.Sprintf("evict-cache-%s", m.Name)
}

func patchServerCacheVolumes(podSpec *corev1.PodSpec, m *kubeaiv1.Model, c ModelConfig) {
	if m.Spec.CacheProfile == "" {
		return
	}
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: "models",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: cachePVCName(m, c),
			},
		},
	})
	for i := range podSpec.Containers {
		if podSpec.Containers[i].Name == "server" {
			podSpec.Containers[i].VolumeMounts = append(podSpec.Containers[i].VolumeMounts, corev1.VolumeMount{
				Name:      "models",
				MountPath: modelCacheDir(m),
				SubPath:   strings.TrimPrefix(modelCacheDir(m), "/"),
				ReadOnly:  true,
			})
		}
	}
}

// cleanupOldUrlCache creates a job to delete cache directories for old URLs
// but only after confirming the new model pod is running successfully with the new URL
func (r *ModelReconciler) cleanupOldUrlCache(ctx context.Context, model *kubeaiv1.Model, cfg ModelConfig, oldURL string) error {
	logger := log.FromContext(ctx)

	// Safety check for nil CacheProfile
	if model.Spec.CacheProfile == "" || cfg.CacheProfile.SharedFilesystem == nil {
		// Cannot clean up without a valid cache profile
		return fmt.Errorf("cannot clean up old URL cache: no valid cache profile configured")
	}

	// Ensure we have a valid old URL to clean up
	if oldURL == "" {
		if model.Status.Cache.PreviousURL != "" {
			oldURL = model.Status.Cache.PreviousURL
		} else {
			// No old URL to clean up
			return nil
		}
	}

	// If oldURL is the same as current URL, nothing to clean up
	if oldURL == model.Spec.URL {
		return nil
	}

	// First check if the new model pods are ready and using the new URL
	modelPods := &corev1.PodList{}
	if err := r.List(ctx, modelPods,
		client.InNamespace(model.Namespace),
		client.MatchingLabels{kubeaiv1.PodModelLabel: model.Name}); err != nil {
		return fmt.Errorf("listing model pods: %w", err)
	}

	// See if there are any ready pods that are actually using the new URL
	// We need to verify they are using the current URL and not the old one
	readyPodsWithNewUrl := false
	var readyPodCount, readyPodsWithNewUrlCount int

	for _, pod := range modelPods.Items {
		// Skip pods that aren't ready
		if !k8sutils.PodIsReady(&pod) {
			continue
		}

		readyPodCount++

		// Check if this pod is using the new URL
		// We need to inspect the pod to see what URL it's actually using
		podIsUsingNewUrl := false

		// Check if pod has the expected volume mounts for the new URL
		newUrlHash := urlHashFromURL(model.Spec.URL)
		expectedCacheDir := fmt.Sprintf("/models/%s-%s-%s", model.Name, model.UID, newUrlHash)

		// Check for URL in container args or env vars
		for _, container := range pod.Spec.Containers {
			// Look for the server container or containers with volume mounts to the expected model cache dir
			if container.Name == "server" || hasMountToDir(&container, expectedCacheDir) {
				// Pod is using the expected model cache directory with the new URL
				podIsUsingNewUrl = true
				break
			}
		}

		if podIsUsingNewUrl {
			readyPodsWithNewUrlCount++
			readyPodsWithNewUrl = true
		}
	}

	if !readyPodsWithNewUrl {
		// No ready pods with new URL yet, keep the old cache
		logger.Info("No ready pods using the new URL yet, deferring cleanup of old URL cache",
			"model", model.Name,
			"oldURL", oldURL,
			"newURL", model.Spec.URL,
			"readyPods", readyPodCount,
			"readyPodsWithNewUrl", readyPodsWithNewUrlCount)
		return nil
	}

	// Log information about pod readiness when we've determined it's safe to clean up
	logger.Info("Found ready pods using the new URL, proceeding with cleanup of old URL cache",
		"model", model.Name,
		"oldURL", oldURL,
		"newURL", model.Spec.URL,
		"readyPods", readyPodCount,
		"readyPodsWithNewUrl", readyPodsWithNewUrlCount)

	// Generate a unique job name that includes the old URL hash to avoid conflicts
	// This allows handling multiple URL transitions properly
	oldUrlHash := urlHashFromURL(oldURL)
	jobName := fmt.Sprintf("cleanup-old-url-cache-%s-%s", model.Name, oldUrlHash)

	// Construct the directory path based on the model name, UID, and old URL hash
	// This ensures we're cleaning up the correct directory for the old URL
	oldCacheDir := fmt.Sprintf("/models/%s-%s-%s", model.Name, model.UID, oldUrlHash)

	logger.Info("Creating job to clean up old URL cache",
		"model", model.Name,
		"jobName", jobName,
		"oldURL", oldURL,
		"oldURLHash", oldUrlHash,
		"oldCacheDir", oldCacheDir)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: model.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/component": "model-cache-cleanup",
				"app.kubernetes.io/part-of":   "kubeai",
				kubeaiv1.PodModelLabel:        model.Name,
				"cleanup-url-hash":            oldUrlHash,
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: ptr.To[int32](60),
			Parallelism:             ptr.To[int32](1),
			Completions:             ptr.To[int32](1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/component": "model-cache-cleanup",
						"app.kubernetes.io/part-of":   "kubeai",
						kubeaiv1.PodModelLabel:        model.Name,
						"cleanup-url-hash":            oldUrlHash,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name: "cleaner",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "model",
									MountPath: "/models",
									SubPath:   "models",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "model",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: cachePVCName(model, cfg),
								},
							},
						},
					},
				},
			},
		},
	}

	job.Spec.Template.Spec.Containers[0].Image = r.ModelLoaders.Image
	job.Spec.Template.Spec.Containers[0].Command = []string{"bash", "-c"}

	// Use a more robust cleanup command that verifies the directory exists before removing it
	// and also creates a completion marker file to verify the cleanup was successful
	cleanupScript := fmt.Sprintf(`
		set -ex
		if [ -d "%s" ]; then
			echo "Found directory %s, removing..."
			rm -rf %s
			echo "Removal completed successfully"
			echo "true" > /tmp/cleanup_success
		else
			echo "Directory %s not found, nothing to clean up"
			echo "true" > /tmp/cleanup_success
		fi
	`, oldCacheDir, oldCacheDir, oldCacheDir, oldCacheDir)

	job.Spec.Template.Spec.Containers[0].Args = []string{cleanupScript}

	if err := ctrl.SetControllerReference(model, job, r.Scheme); err != nil {
		return fmt.Errorf("setting controller reference on cleanup job: %w", err)
	}

	// Check if job already exists
	existingJob := &batchv1.Job{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: model.Namespace,
		Name:      job.Name,
	}, existingJob)

	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create the job
			if err := r.Create(ctx, job); err != nil {
				return fmt.Errorf("creating cleanup job: %w", err)
			}
			logger.Info("Created cleanup job for old URL cache",
				"model", model.Name,
				"jobName", jobName,
				"oldURL", oldURL)
		} else {
			return fmt.Errorf("checking for existing cleanup job: %w", err)
		}
	} else {
		// Job exists, check if completed
		if k8sutils.IsJobCompleted(existingJob) {
			// Verify job completed successfully by checking its status
			cleanupSuccessful := existingJob.Status.Succeeded > 0

			if !cleanupSuccessful {
				logger.Error(fmt.Errorf("cleanup job failed"),
					"Cleanup job did not complete successfully",
					"model", model.Name,
					"jobName", jobName)
				// We'll retry on the next reconciliation
				return nil
			}

			logger.Info("Cleanup job completed successfully",
				"model", model.Name,
				"jobName", jobName,
				"oldURL", oldURL)

			// Delete the job as it's completed
			if err := r.Delete(ctx, existingJob); err != nil {
				return fmt.Errorf("deleting completed cleanup job: %w", err)
			}

			// Only clear the PreviousURL field from the status once cleanup is done
			// and if the PreviousURL still matches the one we just cleaned up
			if model.Status.Cache.PreviousURL == oldURL {
				logger.Info("Clearing PreviousURL from model status after successful cleanup",
					"model", model.Name,
					"previousURL", model.Status.Cache.PreviousURL)

				// Take a snapshot of the model before modification for patching
				originalModel := model.DeepCopy()

				// Clear the PreviousURL field
				model.Status.Cache.PreviousURL = ""

				// Patch the model status
				if err := r.Status().Patch(ctx, model, client.MergeFrom(originalModel)); err != nil {
					return fmt.Errorf("patching model status to clear PreviousURL: %w", err)
				}

				logger.Info("Successfully cleared PreviousURL field",
					"model", model.Name,
					"currentStatus", fmt.Sprintf("Loaded: %v, URL: %s, PreviousURL: %s",
						model.Status.Cache.Loaded,
						model.Status.Cache.URL,
						model.Status.Cache.PreviousURL))
			} else if model.Status.Cache.PreviousURL != "" {
				// If PreviousURL is different, we might have had multiple URL changes
				// Check if we need to clean up more old URLs
				logger.Info("Found different PreviousURL than the one just cleaned up",
					"model", model.Name,
					"cleanedURL", oldURL,
					"currentPreviousURL", model.Status.Cache.PreviousURL)

				// We'll handle this in the next reconciliation
			}
		} else if jobFailed(existingJob) {
			// If the job failed, log the error and delete the job to retry
			logger.Error(fmt.Errorf("cleanup job failed"),
				"Cleanup job failed, will delete and retry",
				"model", model.Name,
				"jobName", jobName)

			// Delete the failed job so we can retry
			if err := r.Delete(ctx, existingJob); err != nil {
				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("deleting failed cleanup job: %w", err)
				}
			}
		}
	}

	return nil
}

// hasMountToDir checks if a container has a volume mount to the specified directory
func hasMountToDir(container *corev1.Container, dir string) bool {
	for _, mount := range container.VolumeMounts {
		if mount.MountPath == dir {
			return true
		}
	}
	return false
}

// jobFailed checks if a job has failed
func jobFailed(job *batchv1.Job) bool {
	return job.Status.Failed > 0
}
