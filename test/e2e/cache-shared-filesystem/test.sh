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
echo "Initial model UID: $model_uid"

# Get the initial file size of pytorch_model.bin
echo "Getting initial file size of pytorch_model.bin..."
initial_size=$(kubectl exec cache-mount-pod -- bash -c "stat -c %s /test-mount/models/opt-125m-cpu-$model_uid/pytorch_model.bin")
echo "Initial pytorch_model.bin size: $initial_size bytes"

# Verify that modifying the url works and results in a new model download
kubectl patch model opt-125m-cpu --type=merge \
  -p '{"spec":{"url":"hf://facebook/opt-350m"}}'

sleep 5
kubectl wait --timeout=300s --for=jsonpath='{.status.cache.loaded}'=true \
  model/opt-125m-cpu

# Get the new file size of pytorch_model.bin
echo "Getting new file size of pytorch_model.bin..."
new_size=$(kubectl exec cache-mount-pod -- bash -c "stat -c %s /test-mount/models/opt-125m-cpu-$model_uid/pytorch_model.bin")
echo "New pytorch_model.bin size: $new_size bytes"

if [ "$new_size" -gt "$initial_size" ]; then
  echo "SUCCESS: The new model file is larger ($new_size > $initial_size), indicating the model was properly updated"
  exit 0
else
  echo "ERROR: The new model file is not larger than the initial file ($new_size <= $initial_size)"
  exit 1
fi
