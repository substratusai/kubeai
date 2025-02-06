#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

model="opt-125m-cpu"

# Run a constant-user load generation pod.
# This should trigger the autoscaler to scale up the model.
kubectl create configmap k6 --from-file $TEST_DIR/k6.js
kubectl create -f $TEST_DIR/k6-pod.yaml

kubectl apply -f $TEST_DIR/model.yaml

kubectl wait --timeout=3m --for=condition=Ready pod/k6

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