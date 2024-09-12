package k8sutils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/substratusai/kubeai/internal/k8sutils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestKubeAIManagedFields(t *testing.T) {
	obj := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:    "kubeai-manager",
					FieldsType: "FieldsV1",
					FieldsV1: &metav1.FieldsV1{
						Raw: []byte(`{
                            "f:metadata": {
                                "f:labels": {
                                    ".": {},
                                    "f:features.kubeai.org/TextGeneration": {}
                                }
                            },
                            "f:spec": {
                                "f:image": {},
                                "f:replicas": {},
                                "f:resources": {
                                    ".": {},
                                    "f:requests": {
                                        ".": {},
                                        "f:cpu": {},
                                        "f:memory": {}
                                    }
                                }
                            }
                        }`),
					},
				},

				{
					Manager:    "other-manager",
					FieldsType: "FieldsV1",
					FieldsV1: &metav1.FieldsV1{
						Raw: []byte(`{
                            "f:spec": {
                                "f:abc": {}
                            }
                        }`),
					},
				},
			},
		},
	}

	set, err := k8sutils.KubeAIManagedFields(obj)
	require.NoError(t, err)

	cases := []struct {
		name     string
		manager  string
		field    []string
		expected bool
	}{
		{
			name:     "field is managed",
			manager:  "manager",
			field:    []string{"spec", "image"},
			expected: true,
		},
		{
			name:     "field does not exist",
			manager:  "manager",
			field:    []string{"spec", "doesnotexist"},
			expected: false,
		},
		{
			name:     "other manager",
			manager:  "other-manager",
			field:    []string{"spec", "abc"},
			expected: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, k8sutils.ManagesField(set, c.field...))
		})
	}
}
