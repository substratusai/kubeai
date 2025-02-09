#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

model="opt-125m-cpu"

# Run a constant-user load generation pod.
# This should trigger the autoscaler to scale up the model.
kubectl create configmap k6 --from-file $TEST_DIR/k6.js
kubectl create -f $TEST_DIR/k6-pod.yaml

kubectl apply -f $TEST_DIR/model.yaml

kubectl wait --timeout=3m --for=condition=Ready pod/k6
kubectl wait --timeout=60s --for=jsonpath='{.spec.replicas}'=3 model/$model

# Stop load generation pod.
kubectl delete --now -f $TEST_DIR/k6-pod.yaml
# Restart KubeAI without load.
kubectl delete pods -l app.kubernetes.io/name=kubeai

# Model should be scaled down.
kubectl wait --timeout=120s --for=jsonpath='{.spec.replicas}'=0 model/$model