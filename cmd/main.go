/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"net/http"
	"os"
	"sync"
	"time"

	"sigs.k8s.io/yaml"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
	"github.com/substratusai/kubeai/internal/leader"
	"github.com/substratusai/kubeai/internal/messenger"
	"github.com/substratusai/kubeai/internal/modelautoscaler"
	"github.com/substratusai/kubeai/internal/modelcontroller"
	"github.com/substratusai/kubeai/internal/modelproxy"
	"github.com/substratusai/kubeai/internal/modelresolver"
	"github.com/substratusai/kubeai/internal/modelscaler"
	"github.com/substratusai/kubeai/internal/openaiserver"

	// +kubebuilder:scaffold:imports

	"github.com/substratusai/kubeai/internal/config"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(kubeaiv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	var allowPodAddressOverride bool
	flag.BoolVar(&allowPodAddressOverride, "allow-pod-address-override", false, "If set, the controller will allow the pod address to be overridden by the Model objects. This is useful for development purposes.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	namespace, found := os.LookupEnv("POD_NAMESPACE")
	if !found {
		setupLog.Error(errors.New("POD_NAMESPACE not set"), "POD_NAMESPACE not set")
		os.Exit(1)
	}

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./config.yaml"
	}

	cfg, err := loadConfigFile(configPath)
	if err != nil {
		setupLog.Error(err, "unable to load config file")
		os.Exit(1)
	}
	{
		cfgYaml, err := yaml.Marshal(cfg)
		if err != nil {
			setupLog.Error(err, "unable to marshal config")
			os.Exit(1)
		}
		setupLog.Info("loaded config", "config", string(cfgYaml))
	}

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		// TODO(user): TLSOpts is used to allow configuring the TLS config used for the server. If certificates are
		// not provided, self-signed certificates will be generated by default. This option is not recommended for
		// production environments as self-signed certificates do not offer the same level of trust and security
		// as certificates issued by a trusted Certificate Authority (CA). The primary risk is potentially allowing
		// unauthorized access to sensitive metrics data. Consider replacing with CertDir, CertName, and KeyName
		// to provide certificates, ensuring the server communicates using trusted and secure certificates.
		TLSOpts: tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		// TODO: Consolidate controller and autoscaler leader election.
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "cc6bca10.substratus.ai",
		Cache: cache.Options{
			Scheme: scheme, //mgr.GetScheme(),
			DefaultNamespaces: map[string]cache.Config{
				// Restrict operations to this Namespace.
				// (this should also be enforced by Namespaced RBAC rules)
				namespace: {},
			},
		},
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to create clientset")
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		setupLog.Error(err, "unable to get hostname")
		return
	}
	leaderElection := leader.NewElection(clientset, hostname, namespace)

	modelResolver, err := modelresolver.NewManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to setup model resolver")
		os.Exit(1)
	}

	modelReconciler := &modelcontroller.ModelReconciler{
		Client:                  mgr.GetClient(),
		Scheme:                  mgr.GetScheme(),
		Namespace:               namespace,
		AllowPodAddressOverride: allowPodAddressOverride,
		HuggingfaceSecretName:   cfg.SecretNames.Huggingface,
		ResourceProfiles:        cfg.ResourceProfiles,
		ModelServers:            cfg.ModelServers,
	}
	if err = modelReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Model")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// TODO: Make consecutive scale downs configurable.
	modelScaler := modelscaler.NewModelScaler(mgr.GetClient(), namespace, 3)

	// TODO: Get values from config.
	modelAutoscaler := modelautoscaler.New(
		10*time.Second,
		10,
		leaderElection,
		modelScaler,
		modelResolver,
		1000,
	)
	go modelAutoscaler.Start()

	modelProxy := modelproxy.NewHandler(modelScaler, modelResolver, 3, nil)
	openaiHandler := openaiserver.NewHandler(mgr.GetClient(), modelProxy)
	mux := http.NewServeMux()
	mux.Handle("/openai/", openaiHandler)
	apiServer := &http.Server{Addr: ":8000", Handler: mux}

	httpClient := &http.Client{}

	var msgrs []*messenger.Messenger
	for i, stream := range cfg.Messaging.Streams {
		msgr, err := messenger.NewMessenger(
			ctx,
			stream.RequestsURL,
			stream.ResponsesURL,
			stream.MaxHandlers,
			cfg.Messaging.ErrorMaxBackoff.Duration,
			modelScaler,
			modelResolver,
			httpClient,
		)
		if err != nil {
			setupLog.Error(err, "unable to create messenger", "index", i)
			os.Exit(1)
		}
		msgrs = append(msgrs, msgr)
	}
	// TODO: Start messgers

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		setupLog.Info("starting api server", "addr", apiServer.Addr)
		if err := apiServer.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				setupLog.Error(err, "error serving api server")
				os.Exit(1)
			}
		}
	}()
	go func() {
		setupLog.Info("starting leader election")
		err := leaderElection.Start(ctx)
		if err != nil {
			setupLog.Error(err, "starting leader election")
			os.Exit(1)
		}
	}()
	for i := range msgrs {
		go func() {
			setupLog.Info("Starting messenger", "index", i)
			err := msgrs[i].Start(ctx)
			if err != nil {
				setupLog.Error(err, "starting messenger")
				os.Exit(1)
			}
		}()
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	apiServer.Shutdown(context.Background())

	setupLog.Info("waiting for all servers to shutdown")
	wg.Wait()
	setupLog.Info("exiting")
}

func loadConfigFile(path string) (config.System, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return config.System{}, err
	}
	var cfg config.System
	if err := yaml.Unmarshal(contents, &cfg); err != nil {
		return config.System{}, err
	}
	return cfg, nil
}
