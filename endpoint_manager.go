package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	disv1 "k8s.io/api/discovery/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewEndpointsManager(mgr ctrl.Manager) (*EndpointsManager, error) {
	r := &EndpointsManager{}
	r.Client = mgr.GetClient()
	r.endpoints = map[string]*endpointGroup{}
	if err := r.SetupWithManager(mgr); err != nil {
		return nil, err
	}
	return r, nil
}

type EndpointsManager struct {
	client.Client

	EndpointSizeCallback func(deploymentName string, size int)

	endpointsMtx sync.Mutex
	endpoints    map[string]*endpointGroup
}

func (r *EndpointsManager) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&disv1.EndpointSlice{}).
		Complete(r)
}

func (r *EndpointsManager) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var slice disv1.EndpointSlice
	if err := r.Get(ctx, req.NamespacedName, &slice); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	const serviceNameLabel = "kubernetes.io/service-name"
	serviceName, ok := slice.GetLabels()[serviceNameLabel]
	if !ok {
		log.Printf("no service label on endpointslice: %v", req.Name)
		return ctrl.Result{}, nil
	}

	var sliceList disv1.EndpointSliceList
	if err := r.List(ctx, &sliceList, client.MatchingLabels{serviceNameLabel: serviceName}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing endpointslices: %w", err)
	}

	ips := map[string]struct{}{}
	var port int32
	for _, sliceItem := range sliceList.Items {
		if len(sliceItem.Ports) > 0 {
			p := sliceItem.Ports[0].Port
			if p != nil {
				port = *p
			}
		}
		for _, endpointItem := range sliceItem.Endpoints {
			ready := endpointItem.Conditions.Ready
			if ready != nil && *ready {
				for _, ip := range endpointItem.Addresses {
					ips[ip] = struct{}{}
				}
			}
		}
	}

	priorLen := r.getEndpoints(serviceName).lenIPs()
	r.getEndpoints(serviceName).setIPs(ips, port)

	if priorLen != len(ips) {
		// TODO: Currently Service name needs to match Deployment name, however
		// this shouldn't be the case. We should be able to reference deployment
		// replicas by something else.
		r.EndpointSizeCallback(serviceName, len(ips))
	}

	return ctrl.Result{}, nil
}

func (r *EndpointsManager) getEndpoints(service string) *endpointGroup {
	r.endpointsMtx.Lock()
	e, ok := r.endpoints[service]
	if !ok {
		e = newEndpoints()
		r.endpoints[service] = e
	}
	r.endpointsMtx.Unlock()
	return e
}

func (r *EndpointsManager) GetHost(ctx context.Context, service string) string {
	return r.getEndpoints(service).getHost()
}
