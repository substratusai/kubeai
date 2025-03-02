#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

model_name="files-test-model"

# Create a model with files
cat <<EOF | kubectl apply -f -
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: ${model_name}
spec:
  url: "hf://huggingface/opt-125m"
  engine: VLLM
  minReplicas: 1
  features:
    - TextGeneration
  resourceProfile: "cpu:1"
  files:
    - path: "/config/test-file.txt"
      content: "test content"
    - path: "/config/prompt.txt"
      content: "prompt content"
EOF

# Wait for the model pod to be ready
echo "Waiting for model pod to be ready..."
kubectl wait --timeout=120s --for=condition=Ready "pods/$(kubectl get pods -l "model=${model_name}" -o jsonpath='{.items[0].metadata.name}')"

# Get the model pod name
model_pod=$(kubectl get pods -l "model=${model_name}" -o jsonpath='{.items[0].metadata.name}')

# Check that the files are properly mounted in the model pod
echo "Checking files are mounted in the model pod..."
kubectl exec ${model_pod} -- cat /config/test-file.txt | grep "test content"
kubectl exec ${model_pod} -- cat /config/prompt.txt | grep "prompt content"

echo "Model files e2e test passed!"