#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

model="opt-125m-cpu"

PV_HOST_PATH=/data

kubectl apply -f $REPO_DIR/test/e2e/s3-model/pv.yaml
kubectl apply -f $REPO_DIR/test/e2e/s3-model/pvc.yaml

kubectl create -f $TEST_DIR/s3-instance.yaml
kubectl wait --timeout=3m --for=condition=Ready pod/s3

# Execute into the kind container
kind_container=$(docker ps --filter "name=kind-control-plane" --format "{{.ID}}")
docker exec -i $kind_container bash -c "
  apt update -y && apt install -y python3-pip
  pip install -U "huggingface_hub[cli]" --break-system-packages
  mkdir -p ${PV_HOST_PATH}/models/facebook/opt-125m
  huggingface-cli download facebook/opt-125m --local-dir ${PV_HOST_PATH}/models/facebook/opt-125m \
    --exclude 'tf_model.h5' 'flax_model.msgpack'"

kubectl create -f $TEST_DIR/upload-model-to-s3.yaml
kubectl wait --for=condition=complete --timeout=120s job/upload-model-to-s3

kubectl apply -f $TEST_DIR/model.yaml
kubectl wait --timeout=5m --for=jsonpath='{.status.cache.loaded}'=true model/$model && \
kubectl wait --timeout=20m --for=jsonpath='.status.replicas.ready'=1 model/${model}

sleep 5

# There are 1 replicas so send 10 requests to ensure that both replicas are used.
for i in {1..5}; do
  echo "Sending request $i"
  curl http://localhost:8000/openai/v1/completions \
    --max-time 600 \
    -H "Content-Type: application/json" \
    -d '{"model": "opt-125m-cpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'
done
