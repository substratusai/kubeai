package endpoints

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
	"github.com/substratusai/kubeai/internal/k8sutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewResolver(mgr ctrl.Manager) (*Resolver, error) {
	r := &Resolver{}
	r.Client = mgr.GetClient()
	r.endpoints = map[string]*group{}
	r.ExcludePods = map[string]struct{}{}
	if err := r.SetupWithManager(mgr); err != nil {
		return nil, err
	}
	return r, nil
}

type Resolver struct {
	client.Client

	endpointsMtx sync.Mutex
	// map[<model-name>]endpointGroup
	endpoints map[string]*group

	selfIPsMtx sync.RWMutex
	selfIPs    []string

	ExcludePods map[string]struct{}
}

func (r *Resolver) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{NeedLeaderElection: ptr.To(false)}).
		For(&corev1.Pod{}).
		Complete(r)
}

func (r *Resolver) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	labels := pod.GetLabels()
	if labels == nil {
		return ctrl.Result{}, nil
	}

	const (
		selfLabelKey = "app.kubernetes.io/name"
		selfLabelVal = "kubeai"
	)
	if labels[selfLabelKey] == selfLabelVal {
		var podList corev1.PodList
		if err := r.List(ctx, &podList, client.InNamespace(pod.Namespace), client.MatchingLabels{selfLabelKey: selfLabelVal}); err != nil {
			return ctrl.Result{}, fmt.Errorf("listing matching pods: %w", err)
		}
		var selfIPs []string
		for _, p := range podList.Items {
			if k8sutils.PodIsReady(&p) {
				selfIPs = append(selfIPs, p.Status.PodIP)
			}
		}
		r.selfIPsMtx.Lock()
		r.selfIPs = selfIPs
		r.selfIPsMtx.Unlock()
		return ctrl.Result{}, nil
	}

	modelName, ok := labels[kubeaiv1.PodModelLabel]
	if !ok {
		return ctrl.Result{}, nil
	}

	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.InNamespace(pod.Namespace), client.MatchingLabels{kubeaiv1.PodModelLabel: modelName}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing matching pods: %w", err)
	}

	observedEndpoints := map[string]endpoint{}
	for _, pod := range podList.Items {
		if _, exclude := r.ExcludePods[pod.Name]; exclude {
			continue
		}
		if !k8sutils.PodIsReady(&pod) {
			continue
		}

		// The Model controller should always set the port annotation in the Pods it creates
		// to communicate the port that the given backend listens on.
		port := getPodAnnotation(pod, kubeaiv1.ModelPodPortAnnotation)
		if port == "" {
			log.Printf("ERROR: No port annotation %q found for pod %s, skipping", kubeaiv1.ModelPodPortAnnotation, pod.Name)
			continue
		}

		// Allow overriding the IP address of the pod.
		ip := getPodAnnotation(pod, kubeaiv1.ModelPodIPAnnotation)
		if ip == "" {
			ip = pod.Status.PodIP
		}

		// If the pod has no IP address, skip it.
		if ip == "" {
			continue
		}

		observedEndpoints[pod.Namespace+"/"+pod.Name] = endpoint{
			address:  ip + ":" + port,
			adapters: getEndpointAdapters(pod),
		}
	}

	r.getEndpoints(modelName).reconcileEndpoints(observedEndpoints)

	return ctrl.Result{}, nil
}

func getEndpointAdapters(pod corev1.Pod) map[string]struct{} {
	adapters := map[string]struct{}{}

	for k := range pod.GetLabels() {
		if strings.HasPrefix(k, kubeaiv1.PodAdapterLabelPrefix) {
			adapters[strings.TrimPrefix(k, kubeaiv1.PodAdapterLabelPrefix)] = struct{}{}
		}
	}

	return adapters
}

func getPodAnnotation(pod corev1.Pod, key string) string {
	if ann := pod.GetAnnotations(); ann != nil {
		return ann[key]
	}
	return ""
}

func (r *Resolver) getEndpoints(model string) *group {
	r.endpointsMtx.Lock()
	e, ok := r.endpoints[model]
	if !ok {
		e = newEndpointGroup()
		r.endpoints[model] = e
	}
	r.endpointsMtx.Unlock()
	return e
}

func (r *Resolver) GetSelfIPs() []string {
	r.selfIPsMtx.RLock()
	defer r.selfIPsMtx.RUnlock()
	return r.selfIPs
}

type AddressRequest struct {
	Model   string
	Adapter string
	Prefix  string
}

// AwaitBestAddress returns the "IP:Port" with the lowest number of in-flight requests. It will block until an endpoint
// becomes available or the context times out. It returns a function that should be called when the
// request is complete to decrement the in-flight count.
func (r *Resolver) AwaitBestAddress(ctx context.Context, req AddressRequest) (string, func(), error) {
	return r.getEndpoints(req.Model).getBestAddr(ctx, req.Adapter, req.Prefix, false)
}

// GetAllHosts retrieves the list of all hosts for a given model.
func (r *Resolver) GetAllAddresses(model string) []string {
	return r.getEndpoints(model).getAllAddrs()
}
