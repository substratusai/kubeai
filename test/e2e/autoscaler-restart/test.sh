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

replica_log=$TMP_DIR/model-replicas.log
kubectl wait --timeout=60s --for=jsonpath='{.spec.replicas}'=3 model/$model
kubectl get -w -o template model/$model --template='{{ printf "%d\n" .spec.replicas}}' > $replica_log &
kubectl_watch_pid=$!

sleep 60
kubectl delete pods -l app.kubernetes.io/name=kubeai
sleep 120

kill $kubectl_watch_pid

echo "Replica log:"
cat $replica_log
replicas_over_time=$(cat $replica_log | sort | uniq)

# Replicas should have remained at 3
if [ "$replicas_over_time" != "3" ]; then
    echo "TEST FAILURE: Replicas changed during autoscaler restart."
    cat $replica_log
    exit 1
fi