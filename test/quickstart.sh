#!/usr/bin/env bash

set -xe

KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-kubeai-tests}

if kind get clusters | grep -q ${KIND_CLUSTER_NAME}; then
  echo "Cluster ${KIND_CLUSTER_NAME} already exists.. reusing it"
  else
  kind create cluster --name ${KIND_CLUSTER_NAME}
fi

# Capture PID and run skaffold in background.
skaffold run --tail --port-forward &
skaffold_pid=$!

error_handler() {
  local exit_status=$?  # Capture the exit status of the last command
  if [ $exit_status -ne 0 ]; then
    echo "An error occurred. Exiting with status $exit_status. Leaving kind cluster intact for debugging"
  elif [ "$TEST_CLEANUP" != "false" ]; then
    echo "Exiting normally. Deleting kind cluster"
    kind delete cluster --name=${KIND_CLUSTER_NAME}
  fi
  # Send exit signal to skaffold and wait for it to exit.
  kill "$skaffold_pid"
  wait "$skaffold_pid"
}

trap 'error_handler' ERR EXIT

function wait_for_pod_ready() {
  local label="$1"
  local start_time=$SECONDS

  while ! kubectl get pod -l "$label" | grep -q Running; do
    sleep 5
    if (( SECONDS - start_time >= 300 )); then
      echo "Pods with label $label did not start in time."
      kubectl describe pod -l "$label" || true
      kubectl logs -l "$label" || true
      exit 1
    fi
  done

  kubectl wait --for=condition=ready pod -l "$label" --timeout=1200s
  exit_code=$?
  if [ $exit_code -ne 0 ]; then
    kubectl describe pod -l "$label"
    kubectl logs -l "$label"
    echo "Pods with label $label did not get ready in time."
    exit 1
  fi
}

# wait for kubeai pod to be ready.
wait_for_pod_ready app.kubernetes.io/name=kubeai

kubectl get pods -w | awk '{print "[pods -w] " $0}' &
kubectl get events -w | awk '{print "[events -w] " $0}' &

# Ensure the model count is 0.
curl -s -X GET "http://localhost:8000/openai/v1/models" | jq '. | length == 0'

# By using the --reuse-values flag we can just append models to the previous install
# while avoiding overriding the image that skaffold originally built and set in the
# first install.
helm install kubeai-models ./charts/models -f - <<EOF
catalog:
  gemma2-2b-cpu:
    enabled: true
    minReplicas: 1
  qwen2-500m-cpu:
    enabled: true
  nomic-embed-text-cpu:
    enabled: true
  faster-whisper-medium-en-cpu:
    enabled: true
    minReplicas: 1
EOF


# wait for kubeai pod to be ready after helm upgrade.
sleep 10
wait_for_pod_ready app.kubernetes.io/name=kubeai
wait_for_pod_ready model=gemma2-2b-cpu

curl -s -X GET "http://localhost:8000/openai/v1/models" | jq '. | length == 4'

function test_completion() {
  local url="$1"
  http_code=$(curl -L -sw '%{http_code}' ${url} \
    -H "Content-Type: application/json" \
    -d '{"model": "gemma2-2b-cpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}')
  if [[ "$http_code" != "200" ]]; then
    echo "Failed to get completions from $url"
    echo "HTTP code: $http_code"
    exit 1
  fi
}

test_completion "http://localhost:8000/openai/v1/completions"
# Double slash in the URL should work as well.
test_completion "http://localhost:8000/openai//v1/completions"

# Test the speech to text endpoint
wait_for_pod_ready model=faster-whisper-medium-en-cpu
curl -L -o kubeai.mp4 https://github.com/user-attachments/assets/711d1279-6af9-4c6c-a052-e59e7730b757
result=$(curl http://localhost:8000/openai/v1/audio/transcriptions \
  -F "file=@kubeai.mp4" \
  -F "language=en" \
  -F "model=faster-whisper-medium-en-cpu" | jq '.text | ascii_downcase | contains("kubernetes")')
if [ "$result" = "true" ]; then
  echo "The transcript contains 'kubernetes'."
else
  echo "The text does not contain 'kubernetes'."
  exit 1
fi
