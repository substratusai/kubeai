package modelcontroller

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "github.com/substratusai/kubeai/api/v1"
	"github.com/substratusai/kubeai/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_applyResourceProfile(t *testing.T) {
	r := ModelReconciler{
		ResourceProfiles: map[string]config.ResourceProfile{
			"my-gpu": {
				Limits: corev1.ResourceList{
					"nvidia.com/gpu": resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					"memory": resource.MustParse("24Gi"),
				},
				NodeSelector: map[string]string{
					"my-gpu": "true",
				},
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "my-gpu-key",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"my-gpu-val"},
										},
									},
								},
							},
						},
					},
				},
				Tolerations: []corev1.Toleration{
					{
						Key:      "my-gpu-toleration",
						Operator: corev1.TolerationOpExists,
						Effect:   corev1.TaintEffectNoSchedule,
					},
				},
			},
			"tolerations-only": {
				Tolerations: []corev1.Toleration{
					{
						Key:      "toleration1",
						Operator: corev1.TolerationOpExists,
						Effect:   corev1.TaintEffectNoSchedule,
					},
					{
						Key:      "toleration2",
						Operator: corev1.TolerationOpExists,
						Effect:   corev1.TaintEffectNoSchedule,
					},
				},
			},
		},
		ModelServers: config.ModelServers{
			VLLM: config.ModelServer{
				Images: map[string]string{
					"default": "default-vllm-image",
				},
			},
		},
	}

	cases := []struct {
		name          string
		input         *v1.Model
		expectChanged bool
		expected      *v1.Model
	}{
		{
			name: "basic",
			input: &v1.Model{
				Spec: v1.ModelSpec{
					ResourceProfile: "my-gpu:1",
					Engine:          v1.VLLMEngine,
				},
			},
			expectChanged: true,
			expected: &v1.Model{
				Spec: v1.ModelSpec{
					ResourceProfile: "my-gpu:1",
					Engine:          v1.VLLMEngine,
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"nvidia.com/gpu": resource.MustParse("1"),
						},
						Requests: corev1.ResourceList{
							"memory": resource.MustParse("24Gi"),
						},
					},
					NodeSelector: map[string]string{
						"my-gpu": "true",
					},
					Image: "default-vllm-image",
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "my-gpu-key",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"my-gpu-val"},
											},
										},
									},
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "my-gpu-toleration",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
		},
		{
			name: "unchanged",
			input: &v1.Model{
				ObjectMeta: metav1.ObjectMeta{
					ManagedFields: []metav1.ManagedFieldsEntry{
						{
							Manager:    "kubeai-manager",
							FieldsType: "FieldsV1",
							FieldsV1: &metav1.FieldsV1{
								Raw: []byte(`{
                            "f:spec": {
                                "f:image": {},
                                "f:tolerations": {},
                                "f:resources": {},
                                "f:affinity": {},
                                "f:nodeSelector": {}
                            }
                        }`),
							},
						},
					},
				},
				Spec: v1.ModelSpec{
					ResourceProfile: "my-gpu:1",
					Engine:          v1.VLLMEngine,
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"nvidia.com/gpu": resource.MustParse("1"),
						},
						Requests: corev1.ResourceList{
							"memory": resource.MustParse("24Gi"),
						},
					},
					NodeSelector: map[string]string{
						"my-gpu": "true",
					},
					Image: "default-vllm-image",
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "my-gpu-key",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"my-gpu-val"},
											},
										},
									},
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "my-gpu-toleration",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
			expectChanged: false,
		},
		{
			name: "should not change",
			input: &v1.Model{
				Spec: v1.ModelSpec{
					ResourceProfile: "my-gpu:1",
					Engine:          v1.VLLMEngine,
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"custom.com/gpu": resource.MustParse("3"),
						},
						Requests: corev1.ResourceList{
							"memory": resource.MustParse("26Gi"),
						},
					},
					NodeSelector: map[string]string{
						"my-user-specified": "val",
					},
					Image: "default-vllm-image",
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "my-user-key",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"my-user-val"},
											},
										},
									},
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "my-user-toleration",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
			expectChanged: false,
		},
		{
			name: "toleration-addition",
			input: &v1.Model{
				ObjectMeta: metav1.ObjectMeta{
					ManagedFields: []metav1.ManagedFieldsEntry{
						{
							Manager:    "kubeai-manager",
							FieldsType: "FieldsV1",
							FieldsV1: &metav1.FieldsV1{
								Raw: []byte(`{
                            "f:spec": {
                                "f:image": {},
                                "f:tolerations": {}
                            }
                        }`),
							},
						},
					},
				},
				Spec: v1.ModelSpec{
					ResourceProfile: "tolerations-only:1",
					Engine:          v1.VLLMEngine,
					Tolerations: []corev1.Toleration{
						{
							Key:      "toleration1",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					Image: "default-vllm-image",
				},
			},
			expectChanged: true,
			expected: &v1.Model{
				Spec: v1.ModelSpec{
					ResourceProfile: "tolerations-only:1",
					Engine:          v1.VLLMEngine,
					Resources:       &corev1.ResourceRequirements{},
					Tolerations: []corev1.Toleration{
						{
							Key:      "toleration1",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
						{
							Key:      "toleration2",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					Image: "default-vllm-image",
				},
			},
		},
		{
			name: "toleration-changed",
			input: &v1.Model{
				ObjectMeta: metav1.ObjectMeta{
					ManagedFields: []metav1.ManagedFieldsEntry{
						{
							Manager:    "kubeai-manager",
							FieldsType: "FieldsV1",
							FieldsV1: &metav1.FieldsV1{
								Raw: []byte(`{
                            "f:spec": {
                                "f:image": {},
                                "f:tolerations": {}
                            }
                        }`),
							},
						},
					},
				},
				Spec: v1.ModelSpec{
					ResourceProfile: "tolerations-only:1",
					Engine:          v1.VLLMEngine,
					Tolerations: []corev1.Toleration{
						{
							Key:      "toleration1",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
						{
							Key:      "toleration3",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					Image: "default-vllm-image",
				},
			},
			expectChanged: true,
			expected: &v1.Model{
				Spec: v1.ModelSpec{
					ResourceProfile: "tolerations-only:1",
					Engine:          v1.VLLMEngine,
					Resources:       &corev1.ResourceRequirements{},
					Tolerations: []corev1.Toleration{
						{
							Key:      "toleration1",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
						{
							Key:      "toleration2",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					Image: "default-vllm-image",
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			model := c.input
			clone := model.DeepCopy()
			changed, err := r.applyResourceProfile(model)
			require.NoError(t, err)
			if c.expectChanged {
				requireEqualProfileFields(t, c.expected, model)
			} else {
				requireEqualProfileFields(t, clone, model)
			}
			require.Equal(t, c.expectChanged, changed)
		})
	}
}

func requireEqualProfileFields(t *testing.T, a, b *v1.Model) {
	requireEqualJSON(t, a.Spec.ResourceProfile, b.Spec.ResourceProfile)
	requireEqualJSON(t, a.Spec.Resources, b.Spec.Resources)
	requireEqualJSON(t, a.Spec.NodeSelector, b.Spec.NodeSelector)
	requireEqualJSON(t, a.Spec.Affinity, b.Spec.Affinity)
	requireEqualJSON(t, a.Spec.Tolerations, b.Spec.Tolerations)
}

func requireEqualJSON(t *testing.T, a, b interface{}) {
	jsonA, err := json.Marshal(a)
	require.NoError(t, err)
	jsonB, err := json.Marshal(b)
	require.NoError(t, err)
	require.JSONEq(t, string(jsonA), string(jsonB))
}
