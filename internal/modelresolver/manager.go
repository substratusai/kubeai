package modelresolver

import (
	"context"
	"fmt"
	"log"
	"sync"

	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
	"github.com/substratusai/kubeai/internal/k8sutils"
	corev1 "k8s.io/api/core/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewManager(mgr ctrl.Manager) (*Manager, error) {
	r := &Manager{}
	r.Client = mgr.GetClient()
	r.endpoints = map[string]*endpointGroup{}
	r.ExcludePods = map[string]struct{}{}
	if err := r.SetupWithManager(mgr); err != nil {
		return nil, err
	}
	return r, nil
}

type Manager struct {
	client.Client

	endpointsMtx sync.Mutex
	endpoints    map[string]*endpointGroup

	ExcludePods map[string]struct{}
}

func (r *Manager) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}

func (r *Manager) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	labels := pod.GetLabels()
	if labels == nil {
		return ctrl.Result{}, nil
	}
	modelName, ok := labels[kubeaiv1.PodModelLabel]
	if !ok {
		return ctrl.Result{}, nil
	}

	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.MatchingLabels{kubeaiv1.PodModelLabel: modelName}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing matching pods: %w", err)
	}

	addrs := map[string]struct{}{}
	for _, pod := range podList.Items {
		if _, exclude := r.ExcludePods[pod.Name]; exclude {
			continue
		}
		if pod.Status.PodIP == "" || !k8sutils.PodIsReady(&pod) {
			continue
		}
		var port string
		if ann := pod.GetAnnotations(); ann != nil {
			port = ann[kubeaiv1.ModelPodPortAnnotation]
		}
		if port == "" {
			log.Printf("ERROR: No port annotation %q found for pod %s, skipping", kubeaiv1.ModelPodPortAnnotation, pod.Name)
			continue
		}

		// Allow overriding the IP address of the pod.
		if ann := pod.GetAnnotations(); ann != nil {
			if ip, ok := ann[kubeaiv1.ModelPodIPAnnotation]; ok {
				addrs[ip+":"+port] = struct{}{}
				continue
			}
		}

		addrs[pod.Status.PodIP+":"+port] = struct{}{}
	}

	r.getEndpoints(modelName).setAddrs(addrs)

	return ctrl.Result{}, nil
}

func (r *Manager) getEndpoints(service string) *endpointGroup {
	r.endpointsMtx.Lock()
	e, ok := r.endpoints[service]
	if !ok {
		e = newEndpointGroup()
		r.endpoints[service] = e
	}
	r.endpointsMtx.Unlock()
	return e
}

// AwaitBestAddress returns the "IP:Port" with the lowest number of in-flight requests. It will block until an endpoint
// becomes available or the context times out. It returns a function that should be called when the
// request is complete to decrement the in-flight count.
func (r *Manager) AwaitBestAddress(ctx context.Context, model string) (string, func(), error) {
	return r.getEndpoints(model).getBestAddr(ctx)
}

// GetAllHosts retrieves the list of all hosts for a given model.
func (r *Manager) GetAllAddresses(model string) []string {
	return r.getEndpoints(model).getAllAddrs()
}
