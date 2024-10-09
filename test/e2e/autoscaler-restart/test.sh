#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

model="opt-125m-cpu"

kubectl create configmap k6 --from-file $TEST_DIR/k6.js
kubectl create -f $TEST_DIR/k6-pod.yaml

cleanup() {
    echo "Running autoscaler-restart cleanup..."
    kubectl delete configmap k6
    kubectl delete -f $TEST_DIR/k6-pod.yaml
    kubectl delete -f $TEST_DIR/model.yaml
}
trap cleanup EXIT

kubectl apply -f $TEST_DIR/model.yaml

kubectl get -w -o template model/$model --template={{.spec.replicas}} &
kubectl_watch_pid=$!

sleep 60
kill $kubectl_watch_pid
