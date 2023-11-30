#!/bin/bash

set -e
set -x
set -u

this_dir=$(dirname "$0")

skaffold run

kubectl apply -f $this_dir/backend.yaml
kubectl create configmap perf-test --from-file $this_dir/k6.js

job_name=perf-test
kubectl create -f $this_dir/job.yaml

# Wait for Job controller to create the Pod.
sleep 3

kubectl wait pods --selector=job-name=$job_name --for=condition=ready --timeout=600s
kubectl logs --selector=job-name=$job_name --tail=-1 --follow
kubectl wait --for=condition=complete --timeout=6s job/$job_name