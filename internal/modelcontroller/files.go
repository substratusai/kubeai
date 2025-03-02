package modelcontroller

import (
	"strings"

	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
	corev1 "k8s.io/api/core/v1"
)

// patchFileVolumes adds volume and volume mount for the model files if specified.
func patchFileVolumes(spec *corev1.PodSpec, model *kubeaiv1.Model) {
	if len(model.Spec.Files) == 0 {
		return
	}

	// Add volume for the ConfigMap
	spec.Volumes = append(spec.Volumes, corev1.Volume{
		Name: modelFilesVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: getModelFilesConfigMapName(model),
				},
			},
		},
	})
	for i := range spec.Containers {
		if spec.Containers[i].Name == serverContainerName {
			// Add volume mounts for each file
			for _, file := range model.Spec.Files {
				spec.Containers[i].VolumeMounts = append(spec.Containers[i].VolumeMounts, corev1.VolumeMount{
					Name:      modelFilesVolumeName,
					MountPath: file.Path,
					SubPath:   fileConfigMapKey(file.Path),
					ReadOnly:  true,
				})
			}
		}
	}
}

// fileConfigMapKey replaces illegal "/" characters in ConfigMap keys.
// ConfigMap keys must consist of alphanumeric characters, '-', '_' or '.'
func fileConfigMapKey(filePath string) string {
	return strings.ReplaceAll(filePath, "/", "_")
}
