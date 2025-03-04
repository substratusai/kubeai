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
  features: [TextGeneration]
  url: "hf://facebook/opt-125m"
  engine: VLLM
  resourceProfile: "cpu:1"
  minReplicas: 1
  args:
    # This revision does not contain its own chat template.
    - --revision=27dcfa74d334bc871f3234de431e71c6eeba5dd6
  env:
    VLLM_CPU_KVCACHE_SPACE: "1"
  files:
    - path: "/config/chat-template.jinja"
      content: "{% for message in messages %}\n{% if message['role'] == 'user' %}\n{{ 'Question:\n' + message['content'] + '\n\n' }}{% elif message['role'] == 'system' %}\n{{ 'System:\n' + message['content'] + '\n\n' }}{% elif message['role'] == 'assistant' %}{{ 'Answer:\n'  + message['content'] + '\n\n' }}{% endif %}\n{% if loop.last and add_generation_prompt %}\n{{ 'Answer:\n' }}{% endif %}{% endfor %}"
    - path: "/config/prompt.txt"
      content: "prompt content"
EOF

# Wait for the model pod to be ready
echo "Waiting for model pod to be ready..."
kubectl wait --timeout 15m --for=jsonpath='.status.replicas.ready'=1 model/${model_name}

# Get the model pod name
model_pod=$(kubectl get pods -l "model=${model_name}" -o jsonpath='{.items[0].metadata.name}')

# Check that the files are properly mounted in the model pod
echo "Checking files are mounted in the model pod..."
kubectl exec ${model_pod} -- cat /config/prompt.txt | grep "prompt content"

# Check that chat completion works.
curl http://localhost:8000/openai/v1/completions \
    -H "Content-Type: application/json" \
    -d '{
        "model": "files-test-model",
        "max_tokens": 10,
        "messages": [
            {
                "role": "system",
                "content": "You are a helpful assistant."
            },
            {
                "role": "user",
                "content": "Write a haiku that explains the concept of recursion."
            }
        ]
    }'

echo "Model files e2e test passed!"