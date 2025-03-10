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


# Verify that the Model URL can be updated without requests failing.
deepseek_pod=$(kubectl get pod -l model=deepseek-r1-1.5b-cpu -o jsonpath='{.items[0].metadata.name}')
old_model_url="ollama://deepseek-r1:1.5b"
new_model_url="ollama://qwen2.5:0.5b"
old_model_name=${old_model_url#ollama://}
new_model_name=${new_model_url#ollama://}

kubectl patch model deepseek-r1-1.5b-cpu --type=merge -p "{\"spec\": {\"url\": \"$new_model_url\"}}"

# Create a temporary file to store pod names we've seen
seen_pods_file=$(mktemp)
echo "$deepseek_pod" > "$seen_pods_file"

# Function to check pod status and count
check_pod_status() {
  local model_name="$1"
  local old_pod_name="$2"
  # Get current pod count
  pod_count=$(kubectl get pods -l model=$model_name --no-headers | wc -l)

  # During transition we might briefly see 2 pods, but eventually should see exactly 1
  if [ "$pod_count" -gt 2 ]; then
    echo "❌ Too many pods found: $pod_count (expected 1 or 2 during transition)"
    return 1
  fi

  # Check for new pod creation
  current_pods=$(kubectl get pods -l model=$model_name -o name)
  new_pod_count=0

  while IFS= read -r pod; do
    if [ -n "$pod" ]; then
      # Strip the "pod/" prefix from the pod name
      pod_name=${pod#pod/}
      if ! grep -q "^$pod_name$" "$seen_pods_file"; then
        echo "$pod_name" >> "$seen_pods_file"
        new_pod_count=$((new_pod_count + 1))
        if [ "$new_pod_count" -gt 1 ]; then
          echo "❌ Multiple new pods detected! This indicates the rollout created more pods than expected."
          echo "Original pod: $old_pod_name"
          echo "All pods seen:"
          cat "$seen_pods_file"
          return 1
        fi
      fi
    fi
  done <<< "$current_pods"

  # Check if the old pod still exists
  if kubectl get pod $old_pod_name &>/dev/null; then
    old_pod_exists=1
  else
    old_pod_exists=0
  fi

  # Get all running pods
  running_pods=$(kubectl get pods -l model=$model_name --no-headers | grep "Running" | wc -l)

  # Success condition: old pod gone and exactly one running pod
  if [ "$old_pod_exists" -eq 0 ] && [ "$running_pods" -eq 1 ] && [ "$new_pod_count" -le 1 ]; then
    return 0
  fi

  return 1
}

# Make a request and verify pod transition
make_request() {
  local model_name="$1"
  local old_pod_name="$2"
  curl http://localhost:8000/openai/v1/completions \
    --max-time 900 \
    -H "Content-Type: application/json" \
    -d "{\"model\": \"$model_name\", \"prompt\": \"Who was the first president of the United States?\", \"max_tokens\": 40}"

  check_pod_status "$model_name" "$old_pod_name"
}

echo "Waiting for pod transition..."
retry 120 make_request "deepseek-r1-1.5b-cpu" "$deepseek_pod"

# Final verification of pod count and creation events
final_pod_count=$(kubectl get pods -l model=deepseek-r1-1.5b-cpu --no-headers | wc -l)
total_pods_seen=$(wc -l < "$seen_pods_file")

if [ "$final_pod_count" -ne 1 ]; then
  echo "❌ Final pod count incorrect: $final_pod_count (expected exactly 1)"
  echo "Current pods:"
  kubectl get pods -l model=deepseek-r1-1.5b-cpu
  rm "$seen_pods_file"
  exit 1
fi

if [ "$total_pods_seen" -ne 2 ]; then  # Should be exactly original pod + 1 new pod
  echo "❌ Incorrect number of total pods seen: $total_pods_seen (expected exactly 2)"
  echo "Pod history:"
  cat "$seen_pods_file"
  rm "$seen_pods_file"
  exit 1
fi

echo "✅ Pod transition successful - exactly one new pod was created and is running"
rm "$seen_pods_file"

# Verify that the rollout was successful
echo "Verifying successful rollout..."

# List the new pods for the model
echo "Current pods for the model:"
kubectl get pods -l model=deepseek-r1-1.5b-cpu

# For Ollama models, the model URL is in the startup probe command, not in container args
new_pod=$(kubectl get pod -l model=deepseek-r1-1.5b-cpu -o jsonpath='{.items[0].metadata.name}')
startup_probe_cmd=$(kubectl get pod $new_pod -o jsonpath='{.spec.containers[0].startupProbe.exec.command[2]}')
echo "Startup probe command for the new pod:"
echo "$startup_probe_cmd"

# Verify that the new model URL is in the startup probe command
if ! echo "$startup_probe_cmd" | grep -q "$new_model_name"; then
  echo "❌ Rollout verification failed: New model name '$new_model_name' not found in startup probe command"
  exit 1
fi

# Check that the old URL is no longer in use
if echo "$startup_probe_cmd" | grep -q "$old_model_name"; then
  echo "❌ Rollout verification failed: Old model name '$old_model_name' still found in startup probe command"
  exit 1
fi


# Also check that the model is actually available by making a request
echo "Making a request to verify the model is available..."
curl http://localhost:8000/openai/v1/completions \
  --max-time 900 \
  -H "Content-Type: application/json" \
  -d '{"model": "deepseek-r1-1.5b-cpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'


helm uninstall $models_release

# Wait for model pods to be deleted
kubectl wait --for=delete pod -l model=deepseek-r1-1.5b-cpu --timeout=120s

# Verify rollout of a vLLM model works as expected
helm install $models_release $REPO_DIR/charts/models -f - <<EOF
catalog:
  opt-125m-cpu:
    enabled: true
    minReplicas: 1
EOF

sleep 5

curl http://localhost:8000/openai/v1/completions \
  --max-time 900 \
  -H "Content-Type: application/json" \
  -d "{\"model\": \"opt-125m-cpu\", \"prompt\": \"Who was the first president of the United States?\", \"max_tokens\": 40}"

old_vllm_pod=$(kubectl get pod -l model=opt-125m-cpu -o jsonpath='{.items[0].metadata.name}')

seen_pods_file=$(mktemp)
echo "$old_vllm_pod" > "$seen_pods_file"

# Patch the model args to test that rollout is successful and only 1 new pod is created
kubectl patch model opt-125m-cpu --type=merge -p "{\"spec\": {\"args\": [\"--disable-log-stats\"]}}"

retry 120 make_request "opt-125m-cpu" "$old_vllm_pod"

# Final verification of pod count and creation events for vLLM model
final_pod_count=$(kubectl get pods -l model=opt-125m-cpu --no-headers | wc -l)
total_pods_seen=$(wc -l < "$seen_pods_file")

if [ "$final_pod_count" -ne 1 ]; then
  echo "❌ Final pod count incorrect: $final_pod_count (expected exactly 1)"
  echo "Current pods:"
  kubectl get pods -l model=opt-125m-cpu
  rm "$seen_pods_file"
  exit 1
fi

if [ "$total_pods_seen" -ne 2 ]; then  # Should be exactly original pod + 1 new pod
  echo "❌ Incorrect number of total pods seen: $total_pods_seen (expected exactly 2)"
  echo "Pod history:"
  cat "$seen_pods_file"
  rm "$seen_pods_file"
  exit 1
fi

echo "✅ Pod transition successful for vLLM model - exactly one new pod was created and is running"
rm "$seen_pods_file"

# Clean up
helm uninstall $models_release