#!/usr/bin/env bash

set -xe

script_dir=$(dirname "$0")

if [[ -z "$TEST_CASE" ]]; then
    echo "Must provide TEST_CASE in environment" 1>&2
    exit 1
fi

# Create a kind cluster to run all tests.
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
  elif [ "$TEST_CLEANUP" != "false" ]; then
    echo "Exiting normally. Deleting kind cluster"
    kind delete cluster --name=substratus-test
  fi
}

trap 'error_handler' ERR EXIT

# Export Node name for subtests.
export KIND_NODE=$(kind get nodes --name=substratus-test)

skaffold run


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

$script_dir/$TEST_CASE/test.sh