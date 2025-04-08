package modelcontroller

import (
	"encoding/json"
	"fmt"

	"github.com/substratusai/kubeai/internal/config"
	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	corev1 "k8s.io/api/core/v1"
)

func patchPod(patches []config.Patch, pod *corev1.Pod) error {
	pb, err := json.Marshal(patches)
	if err != nil {
		return fmt.Errorf("marshal pod patch: %w", err)
	}

	patch, err := jsonpatch.DecodePatch(pb)
	if err != nil {
		return fmt.Errorf("decode pod patch: %w", err)
	}

	podB, err := json.Marshal(pod)
	if err != nil {
		return fmt.Errorf("marshal pod: %w", err)
	}

	patchedPodB, err := patch.Apply(podB)
	if err != nil {
		return fmt.Errorf("apply pod patch: %w", err)
	}

	patchedPod := &corev1.Pod{}
	if err := json.Unmarshal(patchedPodB, patchedPod); err != nil {
		return fmt.Errorf("unmarshal patched pod: %w", err)
	}
	*pod = *patchedPod
	return nil
}
