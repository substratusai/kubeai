package integration

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"

	"github.com/substratusai/kubeai/internal/command"
	"github.com/substratusai/kubeai/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	testEnv       *envtest.Environment
	testK8sClient client.Client
	testCtx       context.Context
	testCancel    context.CancelFunc
	testNS        = "default"
)

const (
	resourceProfileCPU       = "cpu"
	resourceProfileNvidiaGPU = "nvidia-gpu-l4"
	testVLLMDefualtImage     = "default-vllm-image:v1.2.3"
	testVLLMCPUImage         = "cpu-vllm-image:v1.2.3"
)

var sysCfg = config.System{
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

func TestMain(m *testing.M) {
	testCtx, testCancel = context.WithCancel(ctrl.SetupSignalHandler())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{"../../charts/kubeai/charts/crds/crds"},
		ErrorIfCRDPathMissing: true,
	}
	k8sCfg, err := testEnv.Start()
	requireNoError(err)

	testK8sClient, err = client.New(k8sCfg, client.Options{Scheme: command.Scheme})
	requireNoError(err)

	os.Setenv("POD_NAMESPACE", testNS)
	go func() {
		if err := command.Run(testCtx, k8sCfg, sysCfg); err != nil && !errors.Is(err, context.Canceled) {
			log.Fatal(err)
		}
	}()

	log.Println("running tests")
	code := m.Run()

	// TODO: Run cleanup on ctrl-C, etc.
	log.Println("stopping manager")
	testCancel()
	log.Println("stopping test environment")
	requireNoError(testEnv.Stop())

	os.Exit(code)
}

func requireNoError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
