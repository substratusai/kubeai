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

	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/substratusai/lingo/pkg/autoscaler"
	"github.com/substratusai/lingo/pkg/deployments"
	"github.com/substratusai/lingo/pkg/endpoints"
	"github.com/substratusai/lingo/pkg/leader"
	"github.com/substratusai/lingo/pkg/proxy"
	"github.com/substratusai/lingo/pkg/queue"
	"github.com/substratusai/lingo/pkg/stats"
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
		// TODO: Detect from environment.
		namespace = "default"
	)

	var metricsAddr string
	var probeAddr string
	var concurrencyPerReplica int

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8082", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.IntVar(&concurrencyPerReplica, "concurrency", 100, "the number of simultaneous requests that can be processed by each replica")
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
		// LeaderElection is done in the Autoscaler.
		LeaderElection: false,
	})
	if err != nil {
		return fmt.Errorf("starting manager: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("clientset: %w", err)
	}

	const POD_NAME = "POD_NAME"
	podName := os.Getenv(POD_NAME)
	if podName == "" {
		return fmt.Errorf("environment variable must be set: %v", POD_NAME)
	}
	le := leader.NewElection(clientset, podName, namespace)

	queueManager := queue.NewManager(concurrencyPerReplica)

	endpointManager, err := endpoints.NewManager(mgr)
	if err != nil {
		return fmt.Errorf("setting up endpoint manager: %w", err)
	}
	endpointManager.EndpointSizeCallback = queueManager.UpdateQueueSizeForReplicas

	deploymentManager, err := deployments.NewManager(mgr)
	if err != nil {
		return fmt.Errorf("setting up deloyment manager: %w", err)
	}
	deploymentManager.Namespace = namespace
	deploymentManager.ScaleDownPeriod = 30 * time.Second

	autoscaler, err := autoscaler.New(mgr)
	if err != nil {
		return fmt.Errorf("setting up autoscaler: %w", err)
	}
	autoscaler.Interval = 3 * time.Second
	autoscaler.AverageCount = 10 // 10 * 3 seconds = 30 sec avg
	autoscaler.LeaderElection = le
	autoscaler.Deployments = deploymentManager
	autoscaler.ConcurrencyPerReplica = concurrencyPerReplica
	autoscaler.Queues = queueManager
	go autoscaler.Start()

	proxyHandler := &proxy.Handler{
		Deployments: deploymentManager,
		Endpoints:   endpointManager,
		Queues:      queueManager,
	}
	proxyServer := &http.Server{Addr: ":8080", Handler: proxyHandler}

	statsHandler := &stats.Handler{
		Queues: queueManager,
	}
	statsServer := &http.Server{Addr: ":8083", Handler: statsHandler}

	ctx := ctrl.SetupSignalHandler()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer func() {
			statsServer.Shutdown(context.Background())
			proxyServer.Shutdown(context.Background())
			wg.Done()
		}()
		if err := mgr.Start(ctx); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	}()
	if err := statsServer.ListenAndServe(); err != nil {
		setupLog.Error(err, "error serving stats")
		os.Exit(1)
	}
	go func() {
		setupLog.Info("Starting leader election")
		le.Start(ctx)
	}()
	defer func() {
		setupLog.Info("waiting on manager to stop")
		wg.Wait()
		setupLog.Info("manager stopped")
	}()

	if err := proxyServer.ListenAndServe(); err != nil {
		return fmt.Errorf("listen and serve: %w", err)
	}

	return nil
}
