#!/usr/bin/env bash

set -xe

DELETE_CLUSTER=${DELETE_CLUSTER:-true}
# This is possible because of kind extraPortMappings
HOST=127.0.0.1
PORT=30080
BASE_URL="http://$HOST:$PORT/v1"


if kind get clusters | grep -q substratus-test; then
  echo "Cluster substratus-tests already exists.. reusing it"
  else
  kind create cluster --config - << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: substratus-test
nodes:
- role: control-plane
  # port forward 80 on the host to 80 on this node
  extraPortMappings:
  - containerPort: 30080
    hostPort: 30080
    listenAddress: "127.0.0.1"
EOF
  if [ "$DELETE_CLUSTER" = true ]; then
    echo "Going to delete cluster substratus-test on exit"
    trap "kind delete cluster --name=substratus-test" EXIT
  fi
fi

if ! kubectl get deployment proxy-controller; then
  skaffold run
fi


kubectl wait --for=condition=available --timeout=30s deployment/proxy-controller


if ! helm repo list | grep -q substratusai; then
  helm repo add substratusai https://substratusai.github.io/helm/
fi
helm repo update
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

if [ ! -d "$VENV_DIR" ]; then
  python3 -m venv "$VENV_DIR"
fi
source "$VENV_DIR/bin/activate"
pip3 install openai==1.2.3

# Send 60 requests in parallel to stapi backend using openai python client and threading
python3 $SCRIPT_DIR/test_openai_embedding.py \
  --requests 60 --timeout 300 --base-url "${BASE_URL}" \
  --model text-embedding-ada-002

# Ensure replicas has been scaled up to 1 after sending 60 requests
replicas=$(kubectl get deployment stapi-minilm-l6-v2 -o jsonpath='{.spec.replicas}')
if [ "$replicas" -eq 1 ]; then
  echo "Test passed: Expected 1 replica after sending requests 60 requests"
  else
  echo "Test failed: Expected 1 replica after sending requests 60 requests, got $replicas"
  exit 1
fi

echo "Waiting for deployment to scale down back to 0 within 2 minutes"
for i in {1..15}; do
  if [ "$i" -eq 15 ]; then
    echo "Test failed: Expected 0 replica after not having requests for more than 1 minute, got $replicas"
    exit 1
  fi
  replicas=$(kubectl get deployment stapi-minilm-l6-v2 -o jsonpath='{.spec.replicas}')
  if [ "$replicas" -eq 0 ]; then
    echo "Test passed: Expected 0 replica after not having requests for more than 1 minute"
    break
  fi
  sleep 8
done

# Scale up again after scaling to 0 is broken right now
# requests=500
# echo "Send $requests requests in parallel to stapi backend using openai python client and threading"
# python3 $SCRIPT_DIR/test_openai_embedding.py \
#   --requests $requests --timeout 600 --base-url "${BASE_URL}" \
#   --model text-embedding-ada-002
# 
# replicas=$(kubectl get deployment stapi-minilm-l6-v2 -o jsonpath='{.spec.replicas}')
# if [ "$replicas" -ge 2 ]; then
#   echo "Test passed: Expected 2 or more replicas after sending more than $requests requests, got $replicas"
#   else
#   echo "Test failed: Expected 2 or more replicas after sending more than $requests requests, got $replicas"
#   exit 1
# fi
