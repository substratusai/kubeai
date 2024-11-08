package modelcontroller

import (
	"context"
	"fmt"

	v1 "github.com/substratusai/kubeai/api/v1"
	"github.com/substratusai/kubeai/internal/k8sutils"
	corev1 "k8s.io/api/core/v1"
)

const (
	adapterLoaderContainerName = "adapter-loader"
)

// reconcileAdaptersLoader
func (r *ModelReconciler) reconcileAdapters(ctx context.Context, pods []*corev1.Pod, adapters []v1.Adapter) error {
	type reconcileParam struct {
		pod      *corev1.Pod
		adapters []v1.Adapter
	}
	var reconcileList []reconcileParam

	for _, pod := range pods {
		param := reconcileParam{
			pod: pod,
		}

		for _, adapter := range adapters {
			if k8sutils.GetLabel(pod, v1.PodAdapterLabel(adapter.ID)) != k8sutils.StringHash(adapter.URL) {
				param.adapters = append(param.adapters, adapter)
			}
		}

		reconcileList = append(reconcileList, param)
	}

	for _, param := range reconcileList {
		// TODO: Parallelize
		if err := r.ensureAdapterLoader(ctx, param.pod, len(param.adapters) > 0); err != nil {
			return fmt.Errorf("reconcile adapter loader for pod %q: %w", param.pod.Namespace+"/"+param.pod.Name, err)
		}
		if !k8sutils.EphemeralContainerIsRunning(param.pod, adapterLoaderContainerName) {
			return errReturnEarly
		}
		for _, adapter := range param.adapters {
			if err := r.execAdapterLoad(ctx, param.pod, adapter); err != nil {
				return fmt.Errorf("exec adapter load for pod %q: %w", param.pod.Namespace+"/"+param.pod.Name, err)
			}
		}
	}

	return nil
}

// ensureAdapterLoader ensures that the adapter loader ephemeral container is present or not.
func (r *ModelReconciler) ensureAdapterLoader(ctx context.Context, pod *corev1.Pod, enabled bool) error {
	if !enabled {
		changed := k8sutils.RemoveEphemeralContainer(pod, adapterLoaderContainerName)
		if !changed {
			return nil
		}
		if err := r.Client.SubResource("ephemeralcontainers").Update(ctx, pod); err != nil {
			return fmt.Errorf("update pod ephemeral containers: %w", err)
		}
		return nil
	}

	container := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            adapterLoaderContainerName,
			Image:           r.ModelLoaders.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"sleep", "infinity"},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      adaptersVolName,
					MountPath: adaptersRootDir,
				},
			},
		},
		TargetContainerName: serverContainerName,
	}

	changed := k8sutils.AddEphemeralContainer(pod, container)
	if changed {
		if err := r.Client.SubResource("ephemeralcontainers").Update(ctx, pod); err != nil {
			return fmt.Errorf("update pod ephemeral containers: %w", err)
		}
	}

	return nil
}

// execAdapterLoad executes the adapter load command in the adapter loader container.
func (r *ModelReconciler) execAdapterLoad(ctx context.Context, pod *corev1.Pod, adapter v1.Adapter) error {
	if err := r.execPod(ctx, pod, adapterLoaderContainerName, []string{
		"load", adapter.URL, adapterDir(adapter),
	}); err != nil {
		return fmt.Errorf("exec adapter load: %w", err)
	}
	return nil
}

const (
	adaptersVolName = "adapters"
	adaptersRootDir = "/adapters"
)

func adapterDir(a v1.Adapter) string {
	return fmt.Sprintf("%s/%s", adaptersRootDir, a.ID)
}

func patchServerAdapterVolume(podSpec *corev1.PodSpec) {
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: adaptersVolName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	for i := range podSpec.Containers {
		if podSpec.Containers[i].Name == serverContainerName {
			podSpec.Containers[i].VolumeMounts = append(podSpec.Containers[i].VolumeMounts, corev1.VolumeMount{
				Name:      adaptersVolName,
				MountPath: adaptersRootDir,
				ReadOnly:  true,
			})
		}
	}
}
