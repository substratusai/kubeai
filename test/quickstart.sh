#!/usr/bin/env bash

set -xe

KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-kubeai-tests}

if kind get clusters | grep -q ${KIND_CLUSTER_NAME}; then
  echo "Cluster ${KIND_CLUSTER_NAME} already exists.. reusing it"
  else
  kind create cluster --name ${KIND_CLUSTER_NAME}
fi

# Capture PID and run skaffold devin background
skaffold dev &
skaffold_pid=$!

error_handler() {
  local exit_status=$?  # Capture the exit status of the last command
  if [ $exit_status -ne 0 ]; then
    echo "An error occurred. Exiting with status $exit_status. Leaving kind cluster intact for debugging"
  elif [ "$TEST_CLEANUP" != "false" ]; then
    echo "Exiting normally. Deleting kind cluster"
    kind delete cluster --name=${KIND_CLUSTER_NAME}
  fi
  # Send exit signal to skaffold and wait for it to exit
  kill "$skaffold_pid"
  wait "$skaffold_pid"
}

trap 'error_handler' ERR EXIT

# Get the helm release name
release_name=$(helm list -n default | grep substratus | awk '{print $1}')

# wait for kubeai pod to be ready
while ! kubectl get pod -l app.kubernetes.io/name=kubeai | grep -q Running; do
  sleep 5
  if (( SECONDS >= 600 )); then
    echo "kubeai pod did not start in time"
    exit 1
  fi
done
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=kubeai \
  --timeout=300s

# Ensure the model count is 0
curl -s -X GET "http://localhost:8000/openai/v1/models" | jq '. | length == 0'


helm upgrade --reuse-values --install kubeai charts/kubeai -f - <<EOF
models:
  catalog:
    gemma2-2b-cpu:
      enabled: true
      minReplicas: 1
    qwen2-500m-cpu:
      enabled: true
    nomic-embed-text-cpu:
      enabled: true
EOF


while ! kubectl get pod -l model=gemma2-2b-cpu | grep -q Running; do
  sleep 5
  if (( SECONDS >= 600 )); then
    echo "gemma 2 2b pod did not start in time"
    exit 1
  fi
done
kubectl wait --for=condition=ready pod \
  -l model=gemma2-2b-cpu \
  --timeout=600s

curl -s -X GET "http://localhost:8000/openai/v1/models" | jq '. | length == 3'

curl http://localhost:8000/openai/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "gemma2-2b-cpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'

