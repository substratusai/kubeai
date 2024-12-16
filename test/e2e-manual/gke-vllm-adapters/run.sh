#!/usr/bin/env bash

set -ex

skaffold run -f ./skaffold.yaml --tail --port-forward --profile kubeai-only-gke --default-repo us-central1-docker.pkg.dev/substratus-dev

kubectl apply -f ./model.yaml

kubectl port-forward svc/kubeai 8000:80 &

# raw model
curl -v http://localhost:8000/openai/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "tiny-llama", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'

# with adapter
curl -v http://localhost:8000/openai/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "tiny-llama_colorist", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'
