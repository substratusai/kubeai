#!/usr/bin/env bash

set -e

kind create cluster --name=substratus-test
# trap "kind delete cluster --name=substratus-test" EXIT

skaffold run

kubectl wait --for=condition=available --timeout=30s deployment/proxy-controller

kubectl port-forward svc/proxy-controller 8080:80 &

helm upgrade --install stapi-minilm-l6-v2 substratusai/stapi -f - << EOF
model: all-MiniLM-L6-v2
replicaCount: 0
deploymentAnnotations:
  lingo.substratus.ai/models: text-embedding-ada-002
EOF

# need to wait for a bit for the port-forward to be ready
sleep 5

replicas=$(kubectl get deployment stapi-minilm-l6-v2 -o jsonpath='{.spec.replicas}')
if [ "$replicas" -ne 0 ]; then
  echo "Expected 0 replica before sending requests, got $replicas"
  exit 1
fi

SCRIPT_DIR=$(dirname "$0")
VENV_DIR=$SCRIPT_DIR/.venv

python3 -m venv "$VENV_DIR"
source "$VENV_DIR/bin/activate"
pip3 install openai==1.2.3

# Send 60 requests in parallel to stapi backend using openai python client and threading
python3 $SCRIPT_DIR/test_openai_embedding.py --requests 60 --model text-embedding-ada-002

# Ensure replicas has been scaled up to 1 after sending 60 requests
replicas=$(kubectl get deployment stapi-minilm-l6-v2 -o jsonpath='{.spec.replicas}')
if [ "$replicas" -eq 1 ]; then
  echo "Test passed: Expected 1 replica after sending requests 60 requests"
  else
  echo "Test failed: Expected 1 replica after sending requests 60 requests, got $replicas"
  exit 1
fi


# Send 500 requests in parallel to stapi backend using openai python client and threading
SCRIPT_DIR=$(dirname "$0")
python3 $SCRIPT_DIR/test_openai_embedding.py --requests 500 --model text-embedding-ada-002

# Ensure replicas has been scaled up to more than 1 after sending 500 parallel requests
replicas=$(kubectl get deployment stapi-minilm-l6-v2 -o jsonpath='{.spec.replicas}')
if [ "$replicas" -ge 2 ]; then
  echo "Test passed: Expected 2 or more replicas after sending more than 500 requests, got $replicas"
  else
  echo "Test failed: Expected 2 or more replicas after sending more than 500 requests, got $replicas"
  exit 1
fi
