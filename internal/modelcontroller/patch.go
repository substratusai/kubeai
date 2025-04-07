package modelcontroller

import (
	"encoding/json"
	"fmt"

	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	corev1 "k8s.io/api/core/v1"
)

func (r *ModelReconciler) patchPod(pod *corev1.Pod) error {
	for _, p := range r.ModelServerPods.ModelPodPatches {
		// Marshal the pod patch
		pb, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("marshal pod patch: %w", err)
		}
		// Decode the pod patch
		patch, err := jsonpatch.DecodePatch(pb)
		if err != nil {
			return fmt.Errorf("decode pod patch: %w", err)
		}
		// Marshal the pod to be patched
		podB, err := pod.Marshal()
		if err != nil {
			return fmt.Errorf("marshal pod: %w", err)
		}

		// Apply the patch to the pod
		patchedPodB, err := patch.Apply(podB)
		if err != nil {
			return fmt.Errorf("apply pod patch: %w", err)
		}
		// Unmarshal the patched pod
		patchedPod := &corev1.Pod{}
		if err := json.Unmarshal(patchedPodB, patchedPod); err != nil {
			return fmt.Errorf("unmarshal patched pod: %w", err)
		}
		pod = patchedPod
	}
	return nil
}
