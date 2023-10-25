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
	r.endpoints = map[string]*endpoints{}
	if err := r.SetupWithManager(mgr); err != nil {
		return nil, err
	}
	return r, nil
}

type EndpointsManager struct {
	client.Client

	endpointsMtx sync.Mutex
	endpoints    map[string]*endpoints
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

	var ips []string
	for _, sliceItem := range sliceList.Items {
		for _, endpointItem := range sliceItem.Endpoints {
			ips = append(ips, endpointItem.Addresses...)
		}
	}
	r.getEndpoints(serviceName).set(ips)

	return ctrl.Result{}, nil
}

func (r *EndpointsManager) getEndpoints(service string) *endpoints {
	r.endpointsMtx.Lock()
	e, ok := r.endpoints[service]
	if !ok {
		e = newEndpoints()
		r.endpoints[service] = e
	}
	r.endpointsMtx.Unlock()
	return e
}

func (r *EndpointsManager) GetIPs(ctx context.Context, service string) []string {
	return r.getEndpoints(service).get()
}
