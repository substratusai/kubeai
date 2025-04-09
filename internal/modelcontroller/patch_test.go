package modelcontroller

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/substratusai/kubeai/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func Test_applyJSONPatchToPod(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		modelPod    *corev1.Pod
		patches     []config.JSONPatch
		want        *corev1.Pod
		errorString *string
	}{
		"no-patch": {
			modelPod: &corev1.Pod{},
			patches:  nil,
			want:     &corev1.Pod{},
		},
		"error-patch": {
			modelPod: &corev1.Pod{},
			patches: []config.JSONPatch{
				{Op: "invalid", Path: "/spec/containers/0/image", Value: "new-image"},
			},
			want:        &corev1.Pod{},
			errorString: ptr.To("apply pod patch: Unexpected kind: invalid"),
		},
		"replace-image": {
			modelPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test-image",
						},
					},
				},
			},
			patches: []config.JSONPatch{
				{Op: "replace", Path: "/spec/containers/0/image", Value: "new-image"},
			},
			want: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "new-image",
						},
					},
				},
			},
		},
		"add-preemption-policy": {
			modelPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
				},
				Spec: corev1.PodSpec{},
			},
			patches: []config.JSONPatch{
				{Op: "add", Path: "/spec/preemptionPolicy", Value: "Never"},
			},
			want: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
				},
				Spec: corev1.PodSpec{
					PreemptionPolicy: ptr.To(corev1.PreemptNever),
				},
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := applyJSONPatchToPod(c.patches, c.modelPod)
			if c.errorString != nil {
				require.ErrorContains(t, err, *c.errorString)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, c.want, c.modelPod, "expected pod to be patched correctly")
		})
	}
}
