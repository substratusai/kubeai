package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	disv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestIntegration(t *testing.T) {
	const modelName = "test-model-a"
	deploy := testDeployment(modelName)

	require.NoError(t, testK8sClient.Create(testCtx, deploy))

	backendRequests := &atomic.Int32{}
	testBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendRequests.Add(1)
		w.WriteHeader(200)
	}))
	testBackendURL, err := url.Parse(testBackend.URL)
	require.NoError(t, err)
	testBackendPort, err := strconv.Atoi(testBackendURL.Port())
	require.NoError(t, testK8sClient.Create(testCtx, endpointSlice(modelName, testBackendURL.Hostname(), int32(testBackendPort))))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(3 * time.Second)

		body := []byte(fmt.Sprintf(`{"model": %q}`, modelName))
		req, err := http.NewRequest(http.MethodPost, testServer.URL, bytes.NewReader(body))
		requireNoError(err)

		res, err := testHTTPClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, 200, res.StatusCode)
		require.Equal(t, int32(1), backendRequests.Load())
	}()

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := testK8sClient.Get(testCtx, types.NamespacedName{Namespace: deploy.Namespace, Name: deploy.Name}, deploy)
		assert.NoError(t, err, "getting the deployment")
		assert.GreaterOrEqual(t, deploy.Spec.Replicas, 1, "scale-up should have occurred")
	}, 3*time.Second, time.Second/2, "waiting for the deployment to be scaled up")

	t.Logf("Waiting for wait group")
	wg.Wait()
}

func testDeployment(name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-deploy",
			Namespace: testNamespace,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"lingo.substratus.ai/models": name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "model",
							Image: "some-model:1.2.3",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}
}

func endpointSlice(name, ip string, port int32) *disv1.EndpointSlice {
	return &disv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-deploy",
			Namespace: testNamespace,
			Labels: map[string]string{
				disv1.LabelServiceName: name + "-deploy",
			},
		},
		AddressType: disv1.AddressTypeIPv4,
		Endpoints: []disv1.Endpoint{
			{
				Addresses: []string{ip},
				Conditions: disv1.EndpointConditions{
					Ready:       ptr.To(true),
					Serving:     ptr.To(true),
					Terminating: ptr.To(false),
				},
			},
		},
		Ports: []disv1.EndpointPort{
			{
				Name:     ptr.To("http"),
				Port:     ptr.To(port),
				Protocol: ptr.To(corev1.ProtocolTCP),
			},
		},
	}
}
