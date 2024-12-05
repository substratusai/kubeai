#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

models_release="kubeai-models"

pip install -U "huggingface_hub[cli]"

PV_HOST_PATH=/tmp/data

mkdir -p ${PV_HOST_PATH}

huggingface-cli download facebook/opt-125m --local-dir ${PV_HOST_PATH} \
  --exclude "tf_model.h5" --exclude "flax_model.msgpack"

kubectl apply -f $REPO_DIR/test/e2e/engine-vllm-pvc/pvc.yaml

helm install $models_release $REPO_DIR/charts/models -f - <<EOF
catalog:
  opt-125m-cpu:
    enabled: true
    url: pvc://model-pvc
    minReplicas: 1
EOF

sleep 5

curl http://localhost:8000/openai/v1/completions \
  --max-time 900 \
  -H "Content-Type: application/json" \
  -d '{"model": "opt-125m-cpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'
