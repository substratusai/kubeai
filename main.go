package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("run: %v", err)
	}
}

func run() error {
	const (
		namespace = "default"
	)

	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var concurrencyPerReplica int
	var maxQueueSize int

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8082", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&concurrencyPerReplica, "concurrency", 100, "the number of simultaneous requests that can be processed by each replica")
	flag.IntVar(&maxQueueSize, "max-queue-size", 60000, "the maximum size of the queue that holds requests")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// TODO: Add Deployments to cache list.
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				namespace: {},
			},
		},
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "af3bat4f.substratus.ai",
	})
	if err != nil {
		return fmt.Errorf("starting manager: %w", err)
	}

	fifo := NewFIFOQueueManager(concurrencyPerReplica, maxQueueSize)

	endpoints, err := NewEndpointsManager(mgr)
	if err != nil {
		return fmt.Errorf("setting up endpoint manager: %w", err)
	}
	endpoints.EndpointSizeCallback = fifo.UpdateQueueSize

	scaler, err := NewDeploymentManager(mgr)
	if err != nil {
		return fmt.Errorf("setting up autoscaler: %w", err)
	}
	scaler.Namespace = namespace
	scaler.ScaleDownPeriod = 30 * time.Second

	autoscaler := NewAutoscaler()
	autoscaler.Interval = 3 * time.Second
	autoscaler.AverageCount = 10 // 10 * 3 seconds = 30 sec avg
	autoscaler.Scaler = scaler
	autoscaler.FIFO = fifo
	go autoscaler.Start()

	// Change the global defaults and remove limits on max conns
	defaultTransport := http.DefaultTransport.(*http.Transport)
	defaultTransport.MaxIdleConns = 0
	defaultTransport.MaxIdleConnsPerHost = 0
	defaultTransport.MaxConnsPerHost = 0
	handler := &Handler{
		Deployments: scaler,
		Endpoints:   endpoints,
		FIFO:        fifo,
	}
	server := &http.Server{Addr: ":8080", Handler: handler}

	ctx := ctrl.SetupSignalHandler()

	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		defer func() {
			server.Shutdown(context.Background())
			wg.Done()
		}()
		if err := mgr.Start(ctx); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	}()
	defer func() {
		setupLog.Info("waiting on manager to stop")
		wg.Wait()
		setupLog.Info("manager stopped")
	}()

	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("listen and serve: %w", err)
	}

	return nil
}
