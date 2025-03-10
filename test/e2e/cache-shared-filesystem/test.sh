#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

models_release="kubeai-models"

helm install $models_release $REPO_DIR/charts/models -f - <<EOF
catalog:
  opt-125m-cpu:
    enabled: true
    cacheProfile: e2e-test-kind-pv
    minReplicas: 1
EOF

kubectl wait --timeout=300s --for=jsonpath='{.status.cache.loaded}'=true model/opt-125m-cpu

kubectl apply -f $TEST_DIR/cache-mount-pod.yaml
sleep 5
kubectl wait pods --timeout=300s --for=condition=Ready cache-mount-pod

model_uid=$(kubectl get models.kubeai.org opt-125m-cpu -o jsonpath='{.metadata.uid}')
model_url=$(kubectl get models.kubeai.org opt-125m-cpu -o jsonpath='{.spec.url}')
# Calculate URL hash the same way as in modelCacheDir function
url_hash=$(echo -n $model_url | md5sum | cut -c1-8)

# Check if the file exists in the new cache directory structure
kubectl exec cache-mount-pod -- bash -c "stat /test-mount/models/opt-125m-cpu-$model_uid-$url_hash/pytorch_model.bin"

curl http://localhost:8000/openai/v1/completions \
  --max-time 900 \
  -H "Content-Type: application/json" \
  -d '{"model": "opt-125m-cpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'

old_pod=$(kubectl get pod -l model=opt-125m-cpu -o jsonpath='{.items[0].metadata.name}')

# Verify that updating the model URL will create a new cache directory
new_model_url="hf://facebook/opt-350m"
new_model_url_hash=$(echo -n $new_model_url | md5sum | cut -c1-8)
kubectl patch model opt-125m-cpu --type=merge -p "{\"spec\": {\"url\": \"$new_model_url\"}}"

sleep 5
# Ensure cache gets invalidated so download is triggered again
kubectl wait --timeout=300s --for=jsonpath='{.status.cache.loaded}'=false model/opt-125m-cpu
# Ensure the old cache is still present while new model is being downloaded
kubectl exec cache-mount-pod -- bash -c "stat /test-mount/models/opt-125m-cpu-$model_uid-$url_hash/pytorch_model.bin"
# Wait for the new model to be downloaded to cache
kubectl wait --timeout=300s --for=jsonpath='{.status.cache.loaded}'=true model/opt-125m-cpu
# Verify the new cache is present
kubectl exec cache-mount-pod -- bash -c "stat /test-mount/models/opt-125m-cpu-$model_uid-$new_model_url_hash/pytorch_model.bin"

curl http://localhost:8000/openai/v1/completions \
  --max-time 900 \
  -H "Content-Type: application/json" \
  -d '{"model": "opt-125m-cpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'


check_old_pod_gone() {
  ! kubectl get pod $old_pod | grep -q "Running"
}

# Make a request to the model
make_request() {
  curl http://localhost:8000/openai/v1/completions \
    --max-time 900 \
    -H "Content-Type: application/json" \
    -d '{"model": "opt-125m-cpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'

  check_old_pod_gone
}

retry 120 make_request