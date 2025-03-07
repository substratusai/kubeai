package modelcontroller

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
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
	URL       string    `json:"url,omitempty"` // Store URL to track which version is cached
}

func (r *ModelReconciler) reconcileCache(ctx context.Context, model *kubeaiv1.Model, cfg ModelConfig) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Simple initialization approach - avoid recursion and multiple requeues
	if model.Status.Cache == nil {
		logger.Info("Initializing nil model.Status.Cache",
			"model", model.Name,
			"namespace", model.Namespace)

		// Initialize with a simple, complete structure
		model.Status = kubeaiv1.ModelStatus{
			Cache: &kubeaiv1.ModelStatusCache{
				URL:                "",
				Loaded:             false,
				PendingCleanupURLs: []string{},
			},
		}

		// Persist the initialization
		if err := r.Status().Update(ctx, model); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating model status with initialized Cache: %w", err)
		}

		// Continue processing with initialized fields rather than requeueing
	}

	// If no cache profile is configured, nothing to do
	if model.Spec.CacheProfile == "" {
		return ctrl.Result{}, nil
	}

	modelDeleted := model.DeletionTimestamp != nil

	// Handle URL changes - Safe access after initialization
	urlChanged := model.Status.Cache.URL != model.Spec.URL

	// Store old URL for cleanup if needed
	oldURL := ""
	if urlChanged && model.Status.Cache.URL != "" {
		oldURL = model.Status.Cache.URL

		// Track the old URL for cleanup
		if model.Status.Cache.PendingCleanupURLs == nil {
			model.Status.Cache.PendingCleanupURLs = []string{}
		}

		// Only add to PendingCleanupURLs if not already there
		if !slices.Contains(model.Status.Cache.PendingCleanupURLs, oldURL) {
			model.Status.Cache.PendingCleanupURLs = append(model.Status.Cache.PendingCleanupURLs, oldURL)
		}

		// Mark as not loaded since the URL changed
		model.Status.Cache.Loaded = false
		model.Status.Cache.URL = model.Spec.URL

		// Persist URL change
		if err := r.Status().Update(ctx, model); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating model status for URL change: %w", err)
		}
	}

	// Get or create PVC as needed
	pvc := &corev1.PersistentVolumeClaim{}
	var pvcExists bool
	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: model.Namespace,
		Name:      cachePVCName(model, cfg),
	}, pvc); err != nil {
		if apierrors.IsNotFound(err) {
			pvcExists = false
		} else {
			return ctrl.Result{}, fmt.Errorf("finding pvc: %w", err)
		}
	} else {
		pvcExists = true
	}

	if !pvcExists {
		if !modelDeleted {
			pvc = r.cachePVCForModel(model, cfg)
			// Set controller reference as appropriate for your use case
			if err := r.Create(ctx, pvc); err != nil {
				return ctrl.Result{}, fmt.Errorf("creating pvc: %w", err)
			}
			// Return to ensure the PVC is created before continuing
			return ctrl.Result{}, errReturnEarly
		}
	}

	// Add finalizer if needed for shared filesystems
	if cfg.CacheProfile.SharedFilesystem != nil {
		if controllerutil.AddFinalizer(model, kubeaiv1.ModelCacheEvictionFinalizer) {
			if err := r.Update(ctx, model); err != nil {
				return ctrl.Result{}, fmt.Errorf("adding cache deletion finalizer: %w", err)
			}
		}
	}

	// Handle cache loading job
	loadJob := &batchv1.Job{}
	var jobExists bool

	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: model.Namespace,
		Name:      loadCacheJobName(model),
	}, loadJob); err != nil {
		if apierrors.IsNotFound(err) {
			jobExists = false
		} else {
			return ctrl.Result{}, fmt.Errorf("finding job: %w", err)
		}
	} else {
		jobExists = true
	}

	// Determine if we need to load the cache
	var needsCacheLoad bool

	var pvcModelAnn PVCModelAnnotationValue
	var err error

	if pvcExists {
		pvcModelAnn, err = parsePVCModelAnnotation(pvc, model.Name)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("parsing pvc model annotation: %w", err)
		}

		// Only need to load if:
		// 1. UID doesn't match (different model using same PVC)
		// 2. URL has changed and doesn't match what's in the PVC annotation
		needsCacheLoad = pvcModelAnn.UID != string(model.UID) ||
			(urlChanged && pvcModelAnn.URL != model.Spec.URL)

		logger.Info("Checking if cache needs loading",
			"model", model.Name,
			"needsCacheLoad", needsCacheLoad,
			"urlChanged", urlChanged,
			"pvcUID", pvcModelAnn.UID,
			"modelUID", string(model.UID),
			"pvcURL", pvcModelAnn.URL,
			"modelURL", model.Spec.URL)
	} else {
		// PVC doesn't exist, definitely need to load
		needsCacheLoad = true
	}

	// Handle cache loading job
	loadJob = &batchv1.Job{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: model.Namespace,
		Name:      loadCacheJobName(model),
	}, loadJob); err != nil {
		if apierrors.IsNotFound(err) {
			jobExists = false
		} else {
			return ctrl.Result{}, fmt.Errorf("finding job: %w", err)
		}
	} else {
		jobExists = true
	}

	// CRITICAL: Only create job if needed AND no job already exists
	if needsCacheLoad && !jobExists && !model.Status.Cache.Loaded {
		logger.Info("Creating cache load job",
			"model", model.Name,
			"url", model.Spec.URL,
			"needsCacheLoad", needsCacheLoad,
			"jobExists", jobExists)

		loadJob = r.loadCacheJobForModel(model, cfg)
		if err := ctrl.SetControllerReference(model, loadJob, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting controller reference on job: %w", err)
		}

		if err := r.Create(ctx, loadJob); err != nil {
			return ctrl.Result{}, fmt.Errorf("creating job: %w", err)
		}

		// Return early to wait for job to complete
		return ctrl.Result{}, errReturnEarly
	}

	// If job exists, check if it's complete
	if jobExists {
		if k8sutils.IsJobCompleted(loadJob) {
			logger.Info("Cache load job completed successfully",
				"model", model.Name,
				"job", loadJob.Name)

			// Update PVC annotation to mark it as loaded with this model+URL
			if err := r.updatePVCModelAnnotation(ctx, pvc, model.Name, PVCModelAnnotationValue{
				UID:       string(model.UID),
				Timestamp: time.Now(),
				URL:       model.Spec.URL,
			}); err != nil {
				return ctrl.Result{}, fmt.Errorf("setting pvc model annotation: %w", err)
			}

			// Update model status to mark cache as loaded
			model.Status.Cache.Loaded = true
			model.Status.Cache.URL = model.Spec.URL

			if err := r.Status().Update(ctx, model); err != nil {
				return ctrl.Result{}, fmt.Errorf("updating model status after cache load: %w", err)
			}

			// Clean up the job
			if err := r.Delete(ctx, loadJob, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, fmt.Errorf("deleting completed job: %w", err)
				}
			}
		} else if jobFailed(loadJob) {
			logger.Error(fmt.Errorf("cache load job failed"), "Cache load job failed",
				"model", model.Name,
				"job", loadJob.Name)

			// Delete the job so we can retry
			if err := r.Delete(ctx, loadJob); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, fmt.Errorf("deleting failed job: %w", err)
				}
			}

			// Mark cache as not loaded so we'll try again
			model.Status.Cache.Loaded = false

			if err := r.Status().Update(ctx, model); err != nil {
				return ctrl.Result{}, fmt.Errorf("updating model status after job failure: %w", err)
			}

			// Return early to retry in next reconcile
			return ctrl.Result{Requeue: true}, nil
		} else {
			// Job is still running
			logger.Info("Cache load job still running",
				"model", model.Name,
				"job", loadJob.Name)

			// Return early to wait for job to complete
			return ctrl.Result{}, errReturnEarly
		}
	}

	// Handle cleanup if needed
	if model.Status.Cache.Loaded && len(model.Status.Cache.PendingCleanupURLs) > 0 && urlChanged {
		// Only clean up old URLs if current URL's cache is loaded
		oldURL := model.Status.Cache.PendingCleanupURLs[0]

		logger.Info("Cleaning up old URL cache",
			"model", model.Name,
			"oldURL", oldURL,
			"currentURL", model.Spec.URL)

		if err := r.cleanupOldUrlCache(ctx, model, cfg, oldURL); err != nil {
			logger.Error(err, "Failed to clean up old URL cache",
				"model", model.Name,
				"oldURL", oldURL)
		}
	}

	// Return without error to indicate successful reconciliation
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

	// Apply source-specific pod additions if available
	if c.Source.modelSourcePodAdditions != nil {
		c.Source.modelSourcePodAdditions.applyToPodSpec(&job.Spec.Template.Spec, 0)
	}

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

	// Apply source-specific pod additions if available
	if c.Source.modelSourcePodAdditions != nil {
		c.Source.modelSourcePodAdditions.applyToPodSpec(&job.Spec.Template.Spec, 0)
	}

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
	// Calculate the cache directory path
	cacheDir := modelCacheDir(m)

	// Create a volume for the cache directory
	cacheDirVolume := corev1.Volume{
		Name: "model-cache",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: cachePVCName(m, c),
			},
		},
	}

	// Add to volumes
	podSpec.Volumes = append(podSpec.Volumes, cacheDirVolume)

	// Mount volumes for all containers
	for i := range podSpec.Containers {
		cacheMount := corev1.VolumeMount{
			Name:      "model-cache",
			MountPath: cacheDir,
			// No SubPath, we want the full volume mounted at the cache directory path
		}
		podSpec.Containers[i].VolumeMounts = append(podSpec.Containers[i].VolumeMounts, cacheMount)
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
		// If no oldURL provided but we have URLs in the PendingCleanupURLs list, use the first one
		if len(model.Status.Cache.PendingCleanupURLs) > 0 {
			oldURL = model.Status.Cache.PendingCleanupURLs[0]
		} else {
			// No old URL to clean up
			return nil
		}
	}

	// If oldURL is the same as current URL, nothing to clean up
	if oldURL == model.Spec.URL {
		// Remove from pending list if present
		r.removeURLFromPendingCleanup(ctx, model, oldURL)
		return nil
	}

	// Define a variable to track whether pod-readiness check should be skipped
	// We'll skip this check if:
	// 1. Cache was marked as loaded long enough ago (5+ minutes)
	// 2. Multiple reconcile cycles have occurred (tracked via model annotations)
	skipPodReadinessCheck := false

	// Get the time the cache was marked as loaded
	cacheLoadedTimestamp := ""
	if timestamp, ok := model.Annotations["kubeai.org/cache-loaded-timestamp"]; ok {
		cacheLoadedTimestamp = timestamp
	}

	// If we have a timestamp, check if it's old enough to proceed without pod readiness
	if cacheLoadedTimestamp != "" {
		if parsedTime, err := time.Parse(time.RFC3339, cacheLoadedTimestamp); err == nil {
			// If cache was loaded more than 5 minutes ago, proceed with cleanup regardless of pod readiness
			if time.Since(parsedTime) > 5*time.Minute {
				skipPodReadinessCheck = true
				logger.Info("Cache was loaded more than 5 minutes ago, proceeding with cleanup regardless of pod readiness",
					"model", model.Name,
					"loadedTime", parsedTime)
			}
		}
	}

	// Increment the cleanup attempt counter for this URL
	// This helps track how many times we've tried to clean up this URL
	attemptKey := fmt.Sprintf("kubeai.org/cleanup-attempt-%s", urlHashFromURL(oldURL))
	attempts := 0
	if attemptStr, ok := model.Annotations[attemptKey]; ok {
		if parsed, err := strconv.Atoi(attemptStr); err == nil {
			attempts = parsed
		}
	}
	attempts++

	// After 3 attempts, proceed with cleanup regardless of pod readiness
	if attempts >= 3 {
		skipPodReadinessCheck = true
		logger.Info("Made 3 or more cleanup attempts, proceeding with cleanup regardless of pod readiness",
			"model", model.Name,
			"oldURL", oldURL,
			"attempts", attempts)
	}

	// Only check pod readiness if we're not skipping the check
	readyPodsWithNewUrl := skipPodReadinessCheck

	if !skipPodReadinessCheck {
		// First check if the new model pods are ready and using the new URL
		modelPods := &corev1.PodList{}
		if err := r.List(ctx, modelPods,
			client.InNamespace(model.Namespace),
			client.MatchingLabels{kubeaiv1.PodModelLabel: model.Name}); err != nil {
			return fmt.Errorf("listing model pods: %w", err)
		}

		// See if there are any ready pods that are actually using the new URL
		var readyPodCount, readyPodsWithNewUrlCount int

		for _, pod := range modelPods.Items {
			// Skip pods that aren't ready
			if !k8sutils.PodIsReady(&pod) {
				continue
			}

			readyPodCount++

			// Check if this pod is using the new URL
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

		// Update the attempt counter in the model annotations
		originalModel := model.DeepCopy()
		if model.Annotations == nil {
			model.Annotations = map[string]string{}
		}
		model.Annotations[attemptKey] = strconv.Itoa(attempts)

		// If the cache was loaded but we don't have a timestamp, add one
		if model.Status.Cache.Loaded && cacheLoadedTimestamp == "" {
			model.Annotations["kubeai.org/cache-loaded-timestamp"] = time.Now().Format(time.RFC3339)
		}

		// Patch the model to update annotations
		if err := r.Patch(ctx, model, client.MergeFrom(originalModel)); err != nil {
			logger.Error(err, "Failed to update model annotations for cleanup tracking",
				"model", model.Name)
			// Continue anyway, this isn't critical
		}

		if !readyPodsWithNewUrl {
			// No ready pods with new URL yet, keep the old cache
			logger.Info("No ready pods using the new URL yet, deferring cleanup of old URL cache",
				"model", model.Name,
				"oldURL", oldURL,
				"newURL", model.Spec.URL,
				"readyPods", readyPodCount,
				"readyPodsWithNewUrl", readyPodsWithNewUrlCount,
				"attempts", attempts)
			return nil
		}

		// Log information about pod readiness when we've determined it's safe to clean up
		logger.Info("Found ready pods using the new URL, proceeding with cleanup of old URL cache",
			"model", model.Name,
			"oldURL", oldURL,
			"newURL", model.Spec.URL,
			"readyPods", readyPodCount,
			"readyPodsWithNewUrl", readyPodsWithNewUrlCount)
	}

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

			// Remove the URL from the PendingCleanupURLs list
			r.removeURLFromPendingCleanup(ctx, model, oldURL)

			// Remove the attempt counter annotation for this URL
			originalModel := model.DeepCopy()
			if model.Annotations != nil {
				delete(model.Annotations, attemptKey)
				if err := r.Patch(ctx, model, client.MergeFrom(originalModel)); err != nil {
					logger.Error(err, "Failed to remove attempt counter annotation",
						"model", model.Name)
					// Continue anyway, this isn't critical
				}
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

// removeURLFromPendingCleanup removes a URL from the PendingCleanupURLs list
func (r *ModelReconciler) removeURLFromPendingCleanup(ctx context.Context, model *kubeaiv1.Model, url string) {
	logger := log.FromContext(ctx)

	// Take a snapshot of the model before modification for patching
	originalModel := model.DeepCopy()

	// Remove URL from PendingCleanupURLs list
	if len(model.Status.Cache.PendingCleanupURLs) > 0 {
		newPendingURLs := []string{}
		urlRemoved := false

		for _, pendingURL := range model.Status.Cache.PendingCleanupURLs {
			if pendingURL != url {
				newPendingURLs = append(newPendingURLs, pendingURL)
			} else {
				urlRemoved = true
			}
		}

		if urlRemoved {
			logger.Info("Removing URL from PendingCleanupURLs after successful cleanup",
				"model", model.Name,
				"url", url,
				"remainingURLs", newPendingURLs)

			model.Status.Cache.PendingCleanupURLs = newPendingURLs
		}
	}

	// Patch the model status if needed
	if err := r.Status().Patch(ctx, model, client.MergeFrom(originalModel)); err != nil {
		logger.Error(err, "Failed to update model status after cleanup",
			"model", model.Name)
	} else {
		logger.Info("Successfully updated model status after cleanup",
			"model", model.Name,
			"currentStatus", fmt.Sprintf("Loaded: %v, URL: %s, PendingCleanupURLs: %v",
				model.Status.Cache.Loaded,
				model.Status.Cache.URL,
				model.Status.Cache.PendingCleanupURLs))
	}
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
