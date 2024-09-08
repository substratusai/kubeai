package integration

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/substratusai/kubeai/internal/command"
	"github.com/substratusai/kubeai/internal/config"
	"gocloud.dev/pubsub"
	_ "gocloud.dev/pubsub/mempubsub"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// General //

var (
	testEnv       *envtest.Environment
	testK8sClient client.Client
	testCtx       context.Context
	testCancel    context.CancelFunc
	testNS        = "default"
)

// Messenger //

var (
	testRequestsTopic         *pubsub.Topic
	testResponsesSubscription *pubsub.Subscription
)

const (
	memRequestsURL  = "mem://requests"
	memResponsesURL = "mem://responses"
)

// Config //

const (
	resourceProfileCPU       = "cpu"
	resourceProfileNvidiaGPU = "nvidia-gpu-l4"
	testVLLMDefualtImage     = "default-vllm-image:v1.2.3"
	testVLLMCPUImage         = "cpu-vllm-image:v1.2.3"
)

// sysCfg returns the System configuration for testing.
// A function is used to avoid accidental modification of the configuration in test cases
// which would be tricky to debug.
func sysCfg() config.System {
	return config.System{
		SecretNames: config.SecretNames{
			Huggingface: "huggingface",
		},
		ModelServers: config.ModelServers{
			VLLM: config.ModelServer{
				Images: map[string]string{
					"default":  testVLLMDefualtImage,
					"cpu-only": testVLLMCPUImage,
				},
			},
		},
		Messaging: config.Messaging{
			Streams: []config.MessageStream{
				{
					RequestsURL:  memRequestsURL,
					ResponsesURL: memResponsesURL,
				},
			},
		},
		ResourceProfiles: map[string]config.ResourceProfile{
			resourceProfileCPU: {
				Requests: corev1.ResourceList{
					"cpu":    resource.MustParse("1"),
					"memory": resource.MustParse("2Gi"),
				},
				Limits: corev1.ResourceList{
					"memory": resource.MustParse("4Gi"),
				},
				NodeSelector: map[string]string{
					"compute-type": "cpu",
				},
				ImageName: "cpu-only",
			},
			resourceProfileNvidiaGPU: {
				Requests: corev1.ResourceList{
					"nvidia.com/gpu": resource.MustParse("1"),
				},
				Limits: corev1.ResourceList{
					"nvidia.com/gpu": resource.MustParse("1"),
				},
				NodeSelector: map[string]string{
					"compute-type": "gpu",
				},
			},
		},
		AllowPodAddressOverride: true,
	}
}

func TestMain(m *testing.M) {
	testCtx, testCancel = context.WithCancel(ctrl.SetupSignalHandler())

	// Setup Kubernetes environment.
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{"../../charts/kubeai/charts/crds/crds"},
		ErrorIfCRDPathMissing: true,
	}
	k8sCfg, err := testEnv.Start()
	requireNoError(err)

	testK8sClient, err = client.New(k8sCfg, client.Options{Scheme: command.Scheme})
	requireNoError(err)

	// Setup messenger requests.
	testRequestsTopic, err = pubsub.OpenTopic(testCtx, memRequestsURL)
	requireNoError(err)

	// Run the system.
	os.Setenv("POD_NAMESPACE", testNS)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := command.Run(testCtx, k8sCfg, sysCfg()); err != nil && !errors.Is(err, context.Canceled) {
			log.Fatal(err)
		}
	}()

	// Create the responses subscription to use for test assertions.
	// NOTE: This must be done after Run() is called because the mempubsub implementation
	// creates the the topic when OpenTopic() is called  and we need that topic to exist before
	// creating the subscription. We sleep for a few seconds to ensure that the asynchronous
	// execution of OpenTopic() has been run.
	time.Sleep(3 * time.Second)
	testResponsesSubscription, err = pubsub.OpenSubscription(testCtx, memResponsesURL)
	requireNoError(err)

	// Test Cases //

	log.Println("running tests")
	code := m.Run()

	// Teardown //

	// TODO: Run cleanup on ctrl-C, etc.
	log.Println("stopping manager")
	testCancel()
	log.Println("stopping test environment")
	requireNoError(testEnv.Stop())

	log.Println("Waiting for Run() to finish")
	wg.Wait()
	log.Println("Run() finished")

	os.Exit(code)
}

func requireNoError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
