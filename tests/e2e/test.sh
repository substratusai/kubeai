#!/usr/bin/env bash

set -xe

# This is possible because of kind extraPortMappings
HOST=127.0.0.1
PORT=30080
BASE_URL="http://$HOST:$PORT/v1"
REPLICAS=${REPLICAS:-3}
REQUESTS=${REQUESTS:-60}
EXPECTED_REPLICAS=${EXPECTED_REPLICAS:-1}


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
fi

error_handler() {
  local exit_status=$?  # Capture the exit status of the last command
  if [ $exit_status -ne 0 ]; then
    echo "An error occurred. Exiting with status $exit_status. Leaving kind cluster intact for debugging"
  else
    echo "Exiting normally. Deleting kind cluster"
    kind delete cluster --name=substratus-test
  fi
}

trap 'error_handler' ERR EXIT


if ! kubectl get deployment lingo; then
  skaffold run
fi

kubectl patch deployment lingo --patch "{\"spec\": {\"replicas\": $REPLICAS}}"

kubectl logs -f deployment/lingo &

kubectl wait --for=condition=available --timeout=30s deployment/lingo


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
  --requests ${REQUESTS} --timeout 300 --base-url "${BASE_URL}" \
  --model text-embedding-ada-002

# Ensure replicas has been scaled up to 1 after sending 60 requests
replicas=$(kubectl get deployment stapi-minilm-l6-v2 -o jsonpath='{.spec.replicas}')
if [ "$replicas" -ge "${EXPECTED_REPLICAS}" ]; then
  echo "Test passed: Expected ${EXPECTED_REPLICAS} or more replicas and got ${replicas} after sending requests ${REQUESTS} requests"
  else
  echo "Test failed: Expected ${EXPECTED_REPLICAS} or more replicas after sending requests ${REQUESTS} requests, got ${replicas}"
  exit 1
fi

# Verify that leader election works by forcing a 120 second apiserver outage
KIND_NODE=$(kind get nodes --name=substratus-test)
docker exec ${KIND_NODE} iptables -I INPUT -p tcp --dport 6443 -j DROP
sleep 120
docker exec ${KIND_NODE} iptables -D INPUT -p tcp --dport 6443 -j DROP
echo "Waiting for K8s to recover from apiserver outage"
sleep 30
until kubectl get deployment stapi-minilm-l6-v2; do
  sleep 1
done

# rerun kubectl logs because previous one got killed when apiserver was down
kubectl logs --tail=500 -f deployment/lingo &

echo "Waiting for deployment to scale down back to 0 within ~2 minutes"
for i in {1..30}; do
  if [ "$i" -eq 30 ]; then
    echo "Test failed: Expected 0 replica after not having requests for more than 1 minute, got $replicas"
    exit 1
  fi
  replicas=$(kubectl get deployment stapi-minilm-l6-v2 -o jsonpath='{.spec.replicas}')
  if [ "$replicas" -eq 0 ]; then
    echo "Test passed: Expected 0 replica after not having requests for more than 1 minute"
    break
  fi
  sleep 6
done
