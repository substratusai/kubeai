#!/bin/bash

source $REPO_ROOT/test/e2e/common.sh

helm install kubeai-models $REPO_ROOT/charts/models -f - <<EOF
catalog:
  gemma2-2b-cpu:
    enabled: true
    minReplicas: 1
  qwen2-500m-cpu:
    enabled: true
  nomic-embed-text-cpu:
    enabled: true
EOF

curl http://localhost:8000/openai/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "gemma2-2b-cpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'
