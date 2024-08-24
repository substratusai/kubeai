package modelcontroller

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "github.com/substratusai/kubeai/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPodPlan(t *testing.T) {
}

func Test_sortPodsByIndex(t *testing.T) {
	cases := []struct {
		name string
		pods []corev1.Pod
		want []corev1.Pod
	}{
		{
			name: "empty",
			pods: []corev1.Pod{},
			want: []corev1.Pod{},
		},
		{
			name: "swapped",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "should-be-second",
						Labels: map[string]string{
							v1.PodIndexLabel: "1",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "should-be-first",
						Labels: map[string]string{
							v1.PodIndexLabel: "0",
						},
					},
				},
			},
			want: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "should-be-first",
						Labels: map[string]string{
							v1.PodIndexLabel: "0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "should-be-second",
						Labels: map[string]string{
							v1.PodIndexLabel: "1",
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sortPodsByIndex(c.pods)
			require.Equal(t, c.want, c.pods)
		})
	}
}
