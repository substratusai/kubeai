package modelcontroller

import (
	"encoding/json"
	"fmt"

	"github.com/substratusai/kubeai/internal/config"
	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	corev1 "k8s.io/api/core/v1"
)

func applyJSONPatchToPod(patches []config.JSONPatch, pod *corev1.Pod) error {
	if len(patches) == 0 {
		return nil
	}

	pb, err := json.Marshal(patches)
	if err != nil {
		return fmt.Errorf("marshal pod patch: %w", err)
	}

	patch, err := jsonpatch.DecodePatch(pb)
	if err != nil {
		return fmt.Errorf("decode pod patch: %w", err)
	}

	podJson, err := json.Marshal(pod)
	if err != nil {
		return fmt.Errorf("marshal pod: %w", err)
	}

	patchedPodJson, err := patch.Apply(podJson)
	if err != nil {
		return fmt.Errorf("apply pod patch: %w", err)
	}

	patchedPod := &corev1.Pod{}
	if err := json.Unmarshal(patchedPodJson, patchedPod); err != nil {
		return fmt.Errorf("unmarshal patched pod: %w", err)
	}
	*pod = *patchedPod
	return nil
}
