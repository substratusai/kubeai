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
)

func TestProxy(t *testing.T) {
	m := modelForTest(t)

	// Create the Model object in the Kubernetes cluster.
	require.NoError(t, testK8sClient.Create(testCtx, m))

	backendComplete := make(chan struct{})
	backendRequests := &atomic.Int32{}
	waitingMetrics := &atomic.Int32{}
	runningMetrics := &atomic.Int32{}
	testModelBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/metrics" {
			vllmMetrics := fmt.Sprintf(`
vllm:num_requests_running{model_name="%s"} %d.0
vllm:num_requests_waiting{model_name="%s"} %d.0
`, m.Name, runningMetrics.Load(), m.Name, waitingMetrics.Load())
			w.Write([]byte(vllmMetrics))
			return
		}

		log.Println("Serving request from testBackend")
		backendRequests.Add(1)
		<-backendComplete
		w.WriteHeader(200)
	}))

	updateModelWithBackend(t, m, testModelBackend)

	// Wait for controller cache to sync.
	time.Sleep(3 * time.Second)

	// Send request number 1
	var wg sync.WaitGroup
	sendRequests(t, &wg, m.Name, 1, http.StatusOK)

	requireModelReplicas(t, m, 1, "Replicas should be scaled up to 1 to process messaging request")
	requireModelPods(t, m, 1, "Pod should be created for the messaging request")
	markAllModelPodsReady(t, m)
	// TODO: Is eventually needed?
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Equal(t, int32(1), backendRequests.Load(), "ensure the request made its way to the backend")
	}, time.Second/10, 3*time.Second)
	completeRequests(backendComplete, 1)

	// Ensure the deployment is autoscaled past 1.
	// Simulate the backend processing the request.
	sendRequests(t, &wg, m.Name, 2, http.StatusOK)
	waitingMetrics.Add(1)
	runningMetrics.Add(1)
	requireModelReplicas(t, m, 2, "Replicas should be scaled up to 2 to process pending messaging request")
	requireModelPods(t, m, 2, "2 Pods should be created for the messaging requests")
	markAllModelPodsReady(t, m)

	completeRequests(backendComplete, 2)
	waitingMetrics.Add(-1)
	runningMetrics.Add(-1)
}
