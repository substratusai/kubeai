#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

models_release="kubeai-models"

helm install $models_release $REPO_DIR/charts/models -f - <<EOF
catalog:
  deepseek-r1-1.5b-cpu:
    enabled: true
    features: [TextGeneration]
    url: 'ollama://deepseek-r1:1.5b'
    engine: OLlama
    minReplicas: 1
    resourceProfile: 'cpu:1'
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
  -d '{"model": "deepseek-r1-1.5b-cpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'


DEEPSEEK_POD=$(kubectl get pod -l model=deepseek-r1-1.5b-cpu -o jsonpath='{.items[0].metadata.name}')

# Test to ensure that model url can be updated without requests failing
kubectl patch model deepseek-r1-1.5b-cpu -p '{"spec": {"url": "ollama://qwen2.5:0.5b"}}'

# Continiously run curl requests to the model until the new pod is ready
while true; do
  curl http://localhost:8000/openai/v1/completions \
    --max-time 900 \
    -H "Content-Type: application/json" \
    -d '{"model": "deepseek-r1-1.5b-cpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'

  # Exit once old pod is gone
  if ! kubectl get pod $DEEPSEEK_POD | grep -q "Running"; then
    break
  fi
done
