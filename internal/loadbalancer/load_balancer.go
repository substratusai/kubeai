package loadbalancer

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	v1 "github.com/substratusai/kubeai/api/v1"
	"github.com/substratusai/kubeai/internal/apiutils"
	"github.com/substratusai/kubeai/internal/k8sutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(mgr ctrl.Manager) (*LoadBalancer, error) {
	r := &LoadBalancer{}
	r.Client = mgr.GetClient()
	r.groups = map[string]*group{}
	r.ExcludePods = map[string]struct{}{}
	if err := r.SetupWithManager(mgr); err != nil {
		return nil, err
	}
	return r, nil
}

type LoadBalancer struct {
	client.Client

	endpointsMtx sync.Mutex
	// map[<model-name>]endpointGroup
	groups map[string]*group

	selfIPsMtx sync.RWMutex
	selfIPs    []string

	ExcludePods map[string]struct{}
}

func (r *LoadBalancer) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{NeedLeaderElection: ptr.To(false)}).
		For(&corev1.Pod{}).
		Complete(r)
}

func (r *LoadBalancer) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	modelName, ok := labels[v1.PodModelLabel]
	if !ok {
		return ctrl.Result{}, nil
	}

	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.InNamespace(pod.Namespace), client.MatchingLabels{v1.PodModelLabel: modelName}); err != nil {
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
		port := getPodAnnotation(pod, v1.ModelPodPortAnnotation)
		if port == "" {
			log.Printf("ERROR: No port annotation %q found for pod %s, skipping", v1.ModelPodPortAnnotation, pod.Name)
			continue
		}

		// Allow overriding the IP address of the pod.
		ip := getPodAnnotation(pod, v1.ModelPodIPAnnotation)
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

	var model v1.Model
	if err := r.Client.Get(ctx, client.ObjectKey{Name: modelName, Namespace: pod.Namespace}, &model); err != nil {
		return ctrl.Result{}, fmt.Errorf("getting model %s: %w", modelName, err)
	}
	r.getOrCreateEndpointGroup(modelName, model.Spec.LoadBalancing).reconcileEndpoints(observedEndpoints)

	return ctrl.Result{}, nil
}

func getEndpointAdapters(pod corev1.Pod) map[string]struct{} {
	adapters := map[string]struct{}{}

	for k := range pod.GetLabels() {
		if strings.HasPrefix(k, v1.PodAdapterLabelPrefix) {
			adapters[strings.TrimPrefix(k, v1.PodAdapterLabelPrefix)] = struct{}{}
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

// getOrCreateEndpointGroup returns the endpoint group for the given model.
// If the group does not exist, it is created.
// This assumes that the existance of the model is already checked.
func (r *LoadBalancer) getOrCreateEndpointGroup(modelName string, lb v1.LoadBalancing) *group {
	r.endpointsMtx.Lock()
	g, ok := r.groups[modelName]
	if !ok {
		g = newEndpointGroup(lb)
		r.groups[modelName] = g
	}
	r.endpointsMtx.Unlock()
	return g
}

func (r *LoadBalancer) getEndpointGroup(modelName string) (*group, bool) {
	r.endpointsMtx.Lock()
	defer r.endpointsMtx.Unlock()
	g, ok := r.groups[modelName]
	return g, ok
}

func (r *LoadBalancer) GetSelfIPs() []string {
	r.selfIPsMtx.RLock()
	defer r.selfIPsMtx.RUnlock()
	return r.selfIPs
}

// AwaitBestAddress returns the "IP:Port" with the lowest number of in-flight requests. It will block until an endpoint
// becomes available or the context times out. It returns a function that should be called when the
// request is complete to decrement the in-flight count.
func (r *LoadBalancer) AwaitBestAddress(ctx context.Context, req *apiutils.Request) (string, func(), error) {
	return r.getOrCreateEndpointGroup(req.Model, req.LoadBalancing).getBestAddr(ctx, req, false)
}

// GetAllHosts retrieves the list of all hosts for a given model.
func (r *LoadBalancer) GetAllAddresses(model string) []string {
	grp, ok := r.getEndpointGroup(model)
	if !ok {
		return nil
	}
	return grp.getAllAddrs()
}
