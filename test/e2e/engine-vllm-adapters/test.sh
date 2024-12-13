#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

models_release="kubeai-models"

helm install $models_release $REPO_DIR/charts/models -f - <<EOF
catalog:
  opt-125m-cpu:
    enabled: true
    minReplicas: 1
    adapters:
    - name: colorist
      url: hf://samos123/opt-125m-colorist
EOF

sleep 5

# Test the base model
curl http://localhost:8000/openai/v1/completions \
  --max-time 600 \
  -H "Content-Type: application/json" \
  -d '{"model": "opt-125m-cpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'

# Test the adapter model
curl http://localhost:8000/openai/v1/completions \
  --max-time 600 \
  -H "Content-Type: application/json" \
  -d '{"model": "opt-125m-cpu_colorist", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'
