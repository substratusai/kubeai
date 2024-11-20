#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

models_release="kubeai-models"

helm install $models_release $REPO_DIR/charts/models -f - <<EOF
catalog:
  opt-125m-cpu:
    enabled: true
    cacheProfile: e2e-test-kind-pv
EOF

kubectl wait --timeout=300s --for=jsonpath='{.status.cache.loaded}'=true model/opt-125m-cpu

kubectl apply -f $TEST_DIR/cache-mount-pod.yaml
kubectl wait pods --for=condition=Ready cache-mount-pod

model_uid=$(kubectl get models.kubeai.org opt-125m-cpu -o jsonpath='{.metadata.uid}')
kubectl exec cache-mount-pod -- bash -c "stat /test-mount/models/opt-125m-cpu-$model_uid/pytorch_model.bin"
