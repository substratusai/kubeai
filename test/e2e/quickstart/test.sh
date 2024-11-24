#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

models_release="kubeai-models"

helm install $models_release $REPO_DIR/charts/models -f - <<EOF
catalog:
  gemma2-2b-cpu:
    enabled: true
    minReplicas: 1
  qwen2-500m-cpu:
    enabled: true
  nomic-embed-text-cpu:
    enabled: true
EOF

# Use a timeout with curl to ensure that the test fails and all
# debugging information is printed if the request takes too long.
curl http://localhost:8000/openai/v1/completions \
  --max-time 900 \
  -H "Content-Type: application/json" \
  -d '{"model": "gemma2-2b-cpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'
