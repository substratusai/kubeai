#!/bin/bash

set -e
set -x
set -u

# TODO: Use an argument when running different cases.
test_case="nginx"

this_dir=$(dirname "$0")

# Deploy lingo.
skaffold run
function cleanup {
	skaffold delete
	kubectl delete -f $this_dir/cases/${test_case}/deployment.yaml
	kubectl delete -f $this_dir/cases/${test_case}/service.yaml
}
trap cleanup EXIT

# Make sure backend is deployed and scaled to zero.
kubectl apply -f $this_dir/cases/${test_case}/service.yaml
kubectl apply -f $this_dir/cases/${test_case}/deployment.yaml
kubectl rollout status -f $this_dir/cases/${test_case}/deployment.yaml

# Upload test script.
kubectl create configmap ${test_case}-load-test --from-file $this_dir/cases/${test_case}/k6.js --dry-run=client -oyaml | kubectl apply -f -

# Delete test Pod if it exists becuase we dont want 2 load tests running at the same time.
kubectl delete -f $this_dir/cases/${test_case}/test-pod.yaml --ignore-not-found=true
kubectl create -f $this_dir/cases/${test_case}/test-pod.yaml

# Tail logs and wait for Pod to complete.
kubectl wait -f $this_dir/cases/${test_case}/test-pod.yaml --for=condition=ready --timeout=30s
kubectl logs -f ${test_case}-load-test --tail=-1 --follow
kubectl wait -f $this_dir/cases/${test_case}/test-pod.yaml --for=condition=complete --timeout=6s

