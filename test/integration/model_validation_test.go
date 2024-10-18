package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "github.com/substratusai/kubeai/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestModelValidation(t *testing.T) {
	metadata := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{
			Name:      name,
			Namespace: testNS,
		}

	}
	cases := []struct {
		model          v1.Model
		update         func(*v1.Model)
		expValid       bool
		expErrContain  string
		expErrContains []string
	}{
		{
			model: v1.Model{
				ObjectMeta: metadata("empty-invalid"),
				Spec:       v1.ModelSpec{},
			},
			expErrContain: "Required value",
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("missing-url-invalid"),
				Spec: v1.ModelSpec{
					Engine:   "Infinity",
					Features: []v1.ModelFeature{},
				},
			},
			expErrContain: "url",
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("malformed-url-invalid"),
				Spec: v1.ModelSpec{
					URL:      "not-a-url",
					Engine:   "Infinity",
					Features: []v1.ModelFeature{},
				},
			},
			expErrContain: "url",
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("missing-engine-invalid"),
				Spec: v1.ModelSpec{
					URL:      "hf://test-repo/test-model",
					Features: []v1.ModelFeature{},
				},
			},
			expErrContain: "spec.engine",
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("minimum-valid"),
				Spec: v1.ModelSpec{
					URL:      "hf://test-repo/test-model",
					Engine:   "VLLM",
					Features: []v1.ModelFeature{},
				},
			},
			expValid: true,
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("invalid-engine"),
				Spec: v1.ModelSpec{
					URL:      "hf://test-repo/test-model",
					Engine:   "NotAValidEngine",
					Features: []v1.ModelFeature{},
				},
			},
			expErrContain: "NotAValidEngine",
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("invalid-feature"),
				Spec: v1.ModelSpec{
					URL:      "hf://test-repo/test-model",
					Engine:   "VLLM",
					Features: []v1.ModelFeature{"NotAValidFeature"},
				},
			},
			expErrContain: "NotAValidFeature",
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("replicas-0-1-2-valid"),
				Spec: v1.ModelSpec{
					URL:         "hf://test-repo/test-model",
					Engine:      "VLLM",
					Features:    []v1.ModelFeature{},
					MinReplicas: 0,
					Replicas:    ptr.To[int32](1),
					MaxReplicas: ptr.To[int32](2),
				},
			},
			expValid: true,
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("replicas-1-nil-2-valid"),
				Spec: v1.ModelSpec{
					URL:         "hf://test-repo/test-model",
					Engine:      "VLLM",
					Features:    []v1.ModelFeature{},
					MinReplicas: 1,
					Replicas:    nil,
					MaxReplicas: ptr.To[int32](2),
				},
			},
			expValid: true,
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("replicas-1-2-nil-valid"),
				Spec: v1.ModelSpec{
					URL:         "hf://test-repo/test-model",
					Engine:      "VLLM",
					Features:    []v1.ModelFeature{},
					MinReplicas: 1,
					Replicas:    ptr.To[int32](2),
					MaxReplicas: nil,
				},
			},
			expValid: true,
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("replicas-1-3-2-invalid"),
				Spec: v1.ModelSpec{
					URL:         "hf://test-repo/test-model",
					Engine:      "VLLM",
					Features:    []v1.ModelFeature{},
					MinReplicas: 1,
					Replicas:    ptr.To[int32](3),
					MaxReplicas: ptr.To[int32](2),
				},
			},
			expErrContain: "replicas should be in the range minReplicas..maxReplicas",
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("replicas-3-nil-2-invalid"),
				Spec: v1.ModelSpec{
					URL:         "hf://test-repo/test-model",
					Engine:      "VLLM",
					Features:    []v1.ModelFeature{},
					MinReplicas: 3,
					MaxReplicas: ptr.To[int32](2),
				},
			},
			expErrContain: "minReplicas should be less than or equal to maxReplicas",
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("cache-profile-with-hf-url-valid"),
				Spec: v1.ModelSpec{
					URL:          "hf://test-repo/test-model",
					Engine:       "VLLM",
					Features:     []v1.ModelFeature{},
					CacheProfile: "some-cache-profile",
				},
			},
			expValid: true,
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("cache-profile-with-non-hf-url-invalid"),
				Spec: v1.ModelSpec{
					URL:          "ollama://test-repo/test-model",
					Engine:       "VLLM",
					Features:     []v1.ModelFeature{},
					CacheProfile: "some-cache-profile",
				},
			},
			expErrContains: []string{
				"cacheProfile is only supported with a huggingface url",
				"hf://",
			},
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("update-no-changes-valid"),
				Spec: v1.ModelSpec{
					URL:      "hf://test-repo/test-model",
					Engine:   "VLLM",
					Features: []v1.ModelFeature{},
				},
			},
			update:   func(m *v1.Model) { /* No changes */ },
			expValid: true,
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("mutate-url-invalid"),
				Spec: v1.ModelSpec{
					URL:      "hf://test-repo/test-model",
					Engine:   "VLLM",
					Features: []v1.ModelFeature{},
				},
			},
			update: func(m *v1.Model) {
				m.Spec.URL = "hf://update-test-repo/update-test-model"
			},
			expErrContain: "url is immutable",
		},
		{
			model: v1.Model{
				ObjectMeta: metadata("mutate-cacheprofile-invalid"),
				Spec: v1.ModelSpec{
					URL:          "hf://test-repo/test-model",
					Engine:       "VLLM",
					Features:     []v1.ModelFeature{},
					CacheProfile: "some-cache-profile",
				},
			},
			update: func(m *v1.Model) {
				m.Spec.CacheProfile = "some-updated-cache-profile"
			},
			expErrContain: "cacheProfile is immutable",
		},
	}
	for i := range cases {
		// Copy case to avoid parallel access issues with the use of range.
		c := cases[i]
		t.Run(c.model.Name, func(t *testing.T) {
			t.Parallel()

			validateErr := func(err error) {
				if c.expValid {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					if c.expErrContain != "" {
						require.Contains(t, err.Error(), c.expErrContain)
					}
					for _, expErrContain := range c.expErrContains {
						require.Contains(t, err.Error(), expErrContain)
					}
				}
			}

			if c.update == nil {
				validateErr(testK8sClient.Create(testCtx, &c.model))
			} else {
				require.NoError(t, testK8sClient.Create(testCtx, &c.model))
				c.update(&c.model)
				validateErr(testK8sClient.Update(testCtx, &c.model))
			}
		})
	}
}
