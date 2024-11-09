package modelcontroller

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/substratusai/kubeai/api/v1"
	"github.com/substratusai/kubeai/internal/k8sutils"
	"github.com/substratusai/kubeai/internal/vllmclient"
	corev1 "k8s.io/api/core/v1"
)

const (
	adapterLoaderContainerName = "adapter-loader"
)

func (r *ModelReconciler) reconcileAdapters(ctx context.Context, pods []*corev1.Pod, adapters []v1.Adapter) error {
	type reconcileParam struct {
		pod         *corev1.Pod
		toEnsure    []v1.Adapter
		toRemoveIDs []string
		engine      string
	}
	var reconcileList []reconcileParam

	for _, pod := range pods {
		param := reconcileParam{
			pod: pod,
		}

		if pod.Labels == nil {
			continue
		}
		switch pod.Labels[appKubernetesIOName] {
		case strings.ToLower(v1.VLLMEngine):
			param.engine = v1.VLLMEngine
		default:
			continue
		}

		deletionCandidates := getLabelledAdapters(pod)

		for _, adapter := range adapters {
			if k8sutils.GetLabel(pod, v1.PodAdapterLabel(adapter.ID)) != k8sutils.StringHash(adapter.URL) {
				param.toEnsure = append(param.toEnsure, adapter)
			} else {
				// Matches, so don't delete.
				delete(deletionCandidates, adapter.ID)
			}
		}

		for adapterID := range deletionCandidates {
			param.toRemoveIDs = append(param.toRemoveIDs, adapterID)
		}

		reconcileList = append(reconcileList, param)
	}

	for _, param := range reconcileList {
		// TODO: Parallelize
		addr := getPodModelServerAddr(param.pod)
		if err := r.ensureAdapterLoader(ctx, param.pod /*, len(param.toEnsure) > 0 || len(param.toRemoveIDs) > 0*/); err != nil {
			return fmt.Errorf("reconcile adapter loader for pod %q: %w", param.pod.Namespace+"/"+param.pod.Name, err)
		}
		if !k8sutils.EphemeralContainerIsRunning(param.pod, adapterLoaderContainerName) {
			return errReturnEarly
		}
		for _, adapter := range param.toEnsure {
			if err := r.execAdapterLoad(ctx, param.pod, adapter); err != nil {
				return fmt.Errorf("exec adapter load for pod %q: %w", param.pod.Namespace+"/"+param.pod.Name, err)
			}
			switch param.engine {
			case v1.VLLMEngine:
				if err := r.VLLMClient.LoadLoraAdapter(ctx, addr, vllmclient.LoadAdapterRequest{
					LoraName: adapter.ID,
					LoraPath: adapterDir(adapter),
				}); err != nil {
					return fmt.Errorf("load vllm adapter %q: %w", adapter.ID, err)
				}
			}
			if err := r.updatePodAddLabel(ctx, param.pod, v1.PodAdapterLabel(adapter.ID), k8sutils.StringHash(adapter.URL)); err != nil {
				return fmt.Errorf("update pod labels for pod %q: %w", param.pod.Namespace+"/"+param.pod.Name, err)
			}
		}
		for _, adapterID := range param.toRemoveIDs {
			if err := r.execAdapterUnload(ctx, param.pod, adapterID); err != nil {
				return fmt.Errorf("exec adapter unload for pod %q: %w", param.pod.Namespace+"/"+param.pod.Name, err)
			}
			switch param.engine {
			case v1.VLLMEngine:
				if err := r.VLLMClient.UnloadLoraAdapter(ctx, addr, vllmclient.UnloadAdapterRequest{
					LoraName: adapterID,
				}); err != nil {
					return fmt.Errorf("unload vllm adapter %q: %w", adapterID, err)
				}
			}
			if err := r.updatePodRemoveLabel(ctx, param.pod, v1.PodAdapterLabel(adapterID)); err != nil {
				return fmt.Errorf("update pod labels for pod %q: %w", param.pod.Namespace+"/"+param.pod.Name, err)
			}
		}
	}

	return nil
}

func getPodModelServerAddr(pod *corev1.Pod) string {
	// Example:
	//
	// kind: Pod
	// annotations:
	//   model-pod-ip: "127.0.0.1"
	//   model-pod-port: "7000"
	//
	var ip, port string
	if pod.Annotations != nil {
		if v, ok := pod.Annotations[v1.ModelPodIPAnnotation]; ok {
			ip = v
		}
		if v, ok := pod.Annotations[v1.ModelPodPortAnnotation]; ok {
			port = v
		}
	}
	if ip == "" {
		ip = pod.Status.PodIP
	}
	return fmt.Sprintf("http://%s:%s", ip, port)
}

// ensureAdapterLoader ensures that the adapter loader ephemeral container is present or not.
func (r *ModelReconciler) ensureAdapterLoader(ctx context.Context, pod *corev1.Pod /*, enabled bool*/) error {
	// NOTE: Ephemeral containers cannot be removed once added.
	/*
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
	*/

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

func (r *ModelReconciler) execAdapterUnload(ctx context.Context, pod *corev1.Pod, adapterID string) error {
	if err := r.execPod(ctx, pod, adapterLoaderContainerName, []string{
		"rm", "-rf", adapterDir(v1.Adapter{ID: adapterID}),
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

func getLabelledAdapters(pod *corev1.Pod) map[string]struct{} {
	ids := make(map[string]struct{})
	for k := range pod.Labels {
		if strings.HasPrefix(k, v1.PodAdapterLabelPrefix) {
			ids[strings.TrimPrefix(k, v1.PodAdapterLabelPrefix)] = struct{}{}
		}
	}
	return ids
}
