package modelcontroller

import (
	"fmt"
	"strings"

	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func (r *ModelReconciler) cachePVCForModel(m *kubeaiv1.Model, c ModelConfig) *corev1.PersistentVolumeClaim {
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cachePVCName(m, c),
			Namespace: m.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{},
	}
	if c.CacheProfile.SharedFilesystem != nil {
		pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
		storageClassName := c.CacheProfile.SharedFilesystem.StorageClassName
		pvc.Spec.StorageClassName = &storageClassName
		pvc.Spec.Resources.Requests = corev1.ResourceList{
			// https://discuss.huggingface.co/t/how-to-get-model-size/11038/7
			corev1.ResourceStorage: resource.MustParse("10Gi"),
		}
	}
	return &pvc
}

func cachePVCName(m *kubeaiv1.Model, c ModelConfig) string {
	switch {
	case c.CacheProfile.SharedFilesystem != nil:
		// One PVC for all models.
		return fmt.Sprintf("model-%s-cache", m.Spec.CacheProfile)
	default:
		// One PVC per model.
		return fmt.Sprintf("model-%s-%s-cache", m.Name, m.Spec.CacheProfile)
	}
}

func (r *ModelReconciler) cacheJobForModel(m *kubeaiv1.Model, c ModelConfig) *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cacheJobName(m),
			Namespace: m.Namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: ptr.To[int32](60),
			Parallelism:             ptr.To[int32](1),
			Completions:             ptr.To[int32](1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsForModel(m),
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name: "downloader",
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

	switch c.Source.typ {
	case modelSourceTypeHuggingface:
		job.Spec.Template.Spec.Containers[0].Image = r.ModelDownloaders.Huggingface.Image
		job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name:  "MODEL_DIR",
				Value: modelCacheDir(m),
			},
			corev1.EnvVar{
				Name:  "MODEL_REPO",
				Value: c.Source.huggingface.repo,
			},
		)
	default:
		panic("unsupported model source, this point should not be reached")
	}

	return job
}

func modelCacheDir(m *kubeaiv1.Model) string {
	return fmt.Sprintf("/models/%s", m.Name)
}

func cacheJobName(m *kubeaiv1.Model) string {
	return fmt.Sprintf("model-%s-cache", m.Name)
}

func patchServerCacheVolumes(podSpec *corev1.PodSpec, m *kubeaiv1.Model, c ModelConfig) {
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
