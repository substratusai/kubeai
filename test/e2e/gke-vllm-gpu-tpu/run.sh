#!/usr/bin/env bash

set -ex

# Spin up latest release and run test GPU and TPU on GKE autopilot.

helm install kubeai ./charts/kubeai \
  -f ./charts/kubeai/values-gke.yaml \
  -f - <<EOF
secrets:
  huggingface:
    token: "${HF_TOKEN}"
modelLoaders:
  huggingface:
    image: "substratusai/huggingface-model-loader:main"
image:
  tag: "main"
  pullPolicy: "Always"
EOF

sleep 5

kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=kubeai --timeout=300s

helm install kubeai-models ./charts/models -f - <<EOF
catalog:
  llama-3.1-8b-instruct-fp8-l4:
    enabled: true
    cacheProfile: premium-filestore
  llama-3.1-8b-instruct-tpu:
    enabled: true
    cacheProfile: premium-filestore
EOF

kubectl port-forward svc/kubeai 8000:80 &

# test scale from 0 on gpu
curl -v http://localhost:8000/openai/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "llama-3.1-8b-instruct-fp8-l4", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'

# test scale from 0 on tpu
curl -v http://localhost:8000/openai/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "llama-3.1-8b-instruct-tpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'
