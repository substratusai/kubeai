package integration

import (
	"net/http/httptest"
	"net/url"
	"strconv"
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

func mockEndpointSlice(t *testing.T, modelName string, testBackend *httptest.Server) {
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
				// Address is not used, see TestMain() where this is re-written at request-time.
				Addresses: []string{"10.0.0.10"},
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

func requireDeploymentReplicas(t *testing.T, deploy *appsv1.Deployment, n int32) {
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := testK8sClient.Get(testCtx, types.NamespacedName{Namespace: deploy.Namespace, Name: deploy.Name}, deploy)
		assert.NoError(t, err, "getting the deployment")
		assert.NotNil(t, deploy.Spec.Replicas, "scale-up should have occurred")
		assert.Equal(t, n, *deploy.Spec.Replicas, "scale-up should have occurred")
	},
		expectedAutoscalingLag+time.Second,
		time.Second/2,
		"waiting for the deployment to be scaled up")
}
