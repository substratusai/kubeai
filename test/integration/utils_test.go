package integration

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/substratusai/kubeai/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func modelForTest(t *testing.T) *v1.Model {
	return &v1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.ToLower(t.Name()),
			Namespace: testNS,
			Annotations: map[string]string{
				"test-annotation": "test",
			},
			Labels: map[string]string{
				"test-label": "test",
			},
		},
		Spec: v1.ModelSpec{
			Owner:           "test",
			URL:             "hf://test-org/test-model",
			Features:        []v1.ModelFeature{v1.ModelFeatureTextGeneration},
			Engine:          v1.VLLMEngine,
			ResourceProfile: resourceProfileCPU + ":3",
			MinReplicas:     0,
			MaxReplicas:     3,
			Args:            []string{"--test-arg"},
			Env:             map[string]string{"TEST_ENV": "test"},
		},
	}
}

func updateModel(t *testing.T, m *v1.Model, modify func(), msg string) {
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(m), m))
		modify()
		assert.NoError(t, testK8sClient.Update(testCtx, m))
	}, 2*time.Second, time.Second/10, "Updating Model should succeed: "+msg)
}

func requireModelReplicas(t *testing.T, m *v1.Model, expectedReplicas int32, msg string, after time.Duration) {
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(m), m))
		assert.NotNil(t, m.Spec.Replicas)
		assert.Equal(t, expectedReplicas, *m.Spec.Replicas)
	}, after, time.Second/10, "Model Replicas should match: "+msg)
}

func requireModelPods(t *testing.T, m *v1.Model, expectedPods int, msg string, after time.Duration) {
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		podList := &corev1.PodList{}
		assert.NoError(t, testK8sClient.List(testCtx, podList, client.InNamespace(testNS), client.MatchingLabels{"model": m.Name}))
		assert.Len(t, podList.Items, expectedPods)
	}, after, time.Second/10, "Model Pods should match: "+msg)
}

func markAllModelPodsReady(t *testing.T, m *v1.Model) {
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		podList := &corev1.PodList{}
		require.NoError(t, testK8sClient.List(testCtx, podList, client.InNamespace(testNS), client.MatchingLabels{"model": m.Name}))
		for _, pod := range podList.Items {
			pod.Status.Phase = corev1.PodRunning
			pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
			require.NoError(t, testK8sClient.Status().Update(testCtx, &pod))
		}
	}, 2*time.Second, time.Second/10, "All model Pods should be marked ready")
}

func completeBackendRequests(c chan struct{}, n int) {
	for i := 0; i < n; i++ {
		c <- struct{}{}
	}
}

func mustFindPodContainerByName(t assert.TestingT, pod *corev1.Pod, name string) corev1.Container {
	for _, c := range pod.Spec.Containers {
		if c.Name == name {
			return c
		}
	}
	assert.Fail(t, "Container not found: "+name)
	return corev1.Container{}
}

func updateModelWithBackend(t *testing.T, m *v1.Model, testModelBackend *httptest.Server) {
	t.Logf("testBackend URL: %s", testModelBackend.URL)
	u, err := url.Parse(testModelBackend.URL)
	require.NoError(t, err)

	updateModel(t, m, func() {
		m.ObjectMeta.Annotations[v1.ModelPodIPAnnotation] = u.Hostname()
		m.ObjectMeta.Annotations[v1.ModelPodPortAnnotation] = u.Port()
	}, "Set model IP/port annotations to direct requests to testBackend instead of the Pod's (non-existant) IP")
}

func sendRequests(t *testing.T, wg *sync.WaitGroup, modelName string, n int, expCode int) {
	for i := 0; i < n; i++ {
		sendRequest(t, wg, modelName, expCode)
	}
}

func sendRequest(t *testing.T, wg *sync.WaitGroup, modelName string, expCode int) {
	t.Helper()
	wg.Add(1)
	go func() {
		defer wg.Done()

		body := []byte(fmt.Sprintf(`{"model": %q}`, modelName))
		req, err := http.NewRequest(http.MethodPost, "http://localhost:8000/openai/v1/completions", bytes.NewReader(body))
		requireNoError(err)

		res, err := testHTTPClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, expCode, res.StatusCode)
	}()
}

func completeRequests(c chan struct{}, n int) {
	for i := 0; i < n; i++ {
		c <- struct{}{}
	}
}
