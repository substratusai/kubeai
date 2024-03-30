package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gocloud.dev/pubsub"
)

func TestSubscriber(t *testing.T) {

	const modelName = "test-model-a-for-subscriber"
	deploy := testDeployment(modelName)

	require.NoError(t, testK8sClient.Create(testCtx, deploy))

	testBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Serving request from testBackend")

		var reqBody struct {
			Model string `json:"model"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&reqBody))
		require.Equal(t, modelName, reqBody.Model)

		w.Write([]byte(fmt.Sprintf(`{"model": %q, "choices": [{"text": "hey"}]}`, reqBody.Model)))
	}))
	t.Logf("testBackend URL: %s", testBackend.URL)

	// Mock an EndpointSlice.
	testBackendURL, err := url.Parse(testBackend.URL)
	require.NoError(t, err)
	testBackendPort, err := strconv.Atoi(testBackendURL.Port())
	require.NoError(t, err)
	require.NoError(t, testK8sClient.Create(testCtx,
		endpointSlice(
			modelName,
			testBackendURL.Hostname(),
			int32(testBackendPort),
		),
	))

	// Wait for deployment mapping to sync.
	time.Sleep(3 * time.Second)

	// Send request id "a"
	sendRequestMessage(t, modelName, "a")

	requireDeploymentReplicas(t, deploy, 1)

	shouldReceiveResponseMessage(t, modelName, "a")
}

func shouldReceiveResponseMessage(t *testing.T, model, id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Logf("Waiting for response message for model %q and id %q", model, id)
	respA, err := testResponsesSubscription.Receive(ctx)
	require.NoError(t, err)
	respA.Ack()
	require.JSONEq(t, fmt.Sprintf(`{"metadata": {"my-id": %q}, "model": %q, "choices": [{"text": "hey"}]}`, id, model), string(respA.Body))
}

func sendRequestMessage(t *testing.T, modelName string, id string) {
	body := []byte(fmt.Sprintf(`
{
	"metadata": {"my-id": %q},
	"model": %q
}`,
		id, modelName))

	err := testRequestsTopic.Send(context.Background(), &pubsub.Message{
		Body: body,
	})
	require.NoError(t, err)
}
