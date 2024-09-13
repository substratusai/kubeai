package integration

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestProxy(t *testing.T) {
	backendComplete := make(chan struct{})

	t.Cleanup(func() {
		// Finish all requests
		close(backendComplete)
		//testCancel()
	})

	m := modelForTest(t)

	// Create the Model object in the Kubernetes cluster.
	require.NoError(t, testK8sClient.Create(testCtx, m))

	backendRequests := &atomic.Int32{}
	totalBackendRequests := &atomic.Int32{}
	testModelBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/metrics" {
			vllmMetrics := fmt.Sprintf(`# HELP vllm:num_requests_running Number of requests currently running on GPU.
# TYPE vllm:num_requests_running gauge
vllm:num_requests_running{model_name="%s"} %d.0
# HELP vllm:num_requests_waiting Number of requests waiting to be processed.
# TYPE vllm:num_requests_waiting gauge
vllm:num_requests_waiting{model_name="%s"} %d.0
`, m.Name, backendRequests.Load(), m.Name, 0)
			t.Log("Serving metrics request from testBackends")
			fmt.Println(vllmMetrics)
			w.Write([]byte(vllmMetrics))
			return
		}

		log.Println("Serving request from testBackend")
		totalBackendRequests.Add(1)
		backendRequests.Add(1)
		defer backendRequests.Add(-1)
		log.Println("Added request to backend:", backendRequests.Load())
		<-backendComplete
		log.Println("Sending response from backend")
		w.WriteHeader(200)
	}))

	updateModelWithBackend(t, m, testModelBackend)

	// Wait for controller cache to sync.
	time.Sleep(3 * time.Second)

	// Send request number 1
	var wg sync.WaitGroup
	sendRequests(t, &wg, m.Name, 1, http.StatusOK)

	requireModelReplicas(t, m, 1, "Replicas should be scaled up to 1 to process messaging request", time.Second)
	requireModelPods(t, m, 1, "Pod should be created for the messaging request", time.Second)
	markAllModelPodsReady(t, m)
	completeRequests(backendComplete, 1)
	require.Equal(t, int32(1), totalBackendRequests.Load(), "ensure the request made its way to the backend")

	const autoscaleUpWait = 10 * time.Second
	// Ensure the deployment is autoscaled past 1.
	// Simulate the backend processing the request.
	sendRequests(t, &wg, m.Name, 2, http.StatusOK)
	requireModelReplicas(t, m, 2, "Replicas should be scaled up to 2 to process pending messaging request", autoscaleUpWait)
	requireModelPods(t, m, 2, "2 Pods should be created for the messaging requests", time.Second)
	markAllModelPodsReady(t, m)

	// Make sure deployment will not be scaled past default max (3).
	sendRequests(t, &wg, m.Name, 2, http.StatusOK)
	require.Never(t, func() bool {
		assert.NoError(t, testK8sClient.Get(testCtx, client.ObjectKeyFromObject(m), m))
		return *m.Spec.Replicas > m.Spec.Autoscaling.MaxReplicas
	}, autoscaleUpWait, time.Second/10, "Replicas should not be scaled past MaxReplicas")

	completeRequests(backendComplete, 4)
	require.Equal(t, int32(5), totalBackendRequests.Load(), "ensure all the requests made their way to the backend")

	// Ensure the deployment is autoscaled back down to MinReplicas.
	const autoscaleDownWait = 10 * time.Second
	requireModelReplicas(t, m, m.Spec.Autoscaling.MinReplicas, "Replicas should scale back to MinReplicas", autoscaleDownWait)
	requireModelPods(t, m, int(m.Spec.Autoscaling.MinReplicas), "Pods should be removed", time.Second)

	t.Log("Waiting for all requests to complete")
	wg.Wait()
	t.Log("All requests completed")
}
