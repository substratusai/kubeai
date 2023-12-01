#!/bin/bash

set -e
set -x
set -u

this_dir=$(dirname "$0")

# Deploy lingo.
skaffold run
function cleanup {
	skaffold delete
	kubectl delete -f $this_dir/backend-deployment.yaml
	kubectl delete -f $this_dir/backend-service.yaml
}
trap cleanup EXIT

# Make sure backend is deployed and scaled to zero.
kubectl apply -f $this_dir/backend-service.yaml
kubectl apply -f $this_dir/backend-deployment.yaml
kubectl rollout status -f $this_dir/backend-deployment.yaml

# Upload test script.
kubectl create configmap perf-test --from-file $this_dir/k6.js --dry-run=client -oyaml | kubectl apply -f -

# Delete test Pod if it exists becuase we dont want 2 load tests running at the same time.
kubectl delete -f $this_dir/test-pod.yaml --ignore-not-found=true
kubectl create -f $this_dir/test-pod.yaml

# Tail logs and wait for Pod to complete.
kubectl wait -f $this_dir/test-pod.yaml --for=condition=ready --timeout=30s
kubectl logs -f perf-test --tail=-1 --follow
kubectl wait -f $this_dir/test-pod.yaml --for=condition=complete --timeout=6s

