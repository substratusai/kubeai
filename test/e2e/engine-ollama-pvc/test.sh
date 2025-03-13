#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

models_release="kubeai-models"

# Create PV_HOST_PATH inside the kind container
kind_container=$(docker ps --filter "name=kind-control-plane" --format "{{.ID}}")
export PV_HOST_PATH="/mnt/models"
docker exec -i $kind_container mkdir -p $PV_HOST_PATH
echo "PV_HOST_PATH: $PV_HOST_PATH"


envsubst < $REPO_DIR/test/e2e/engine-ollama-pvc/pv.yaml | kubectl apply -f -
kubectl apply -f $REPO_DIR/test/e2e/engine-ollama-pvc/pvc.yaml

# Apply the Ollama hydrate job
kubectl apply -f $REPO_DIR/test/e2e/engine-ollama-pvc/ollama-hydrate-job.yaml

# Wait for job completion with timeout
echo "Waiting for Ollama hydrate job to complete..."
if ! kubectl wait --for=condition=complete --timeout=600s job/ollama-pvc-hydrate; then
    echo "Ollama hydrate job failed or timed out"
    kubectl logs job/ollama-pvc-hydrate
    exit 1
fi


helm install $models_release $REPO_DIR/charts/models -f - <<EOF
catalog:
  qwen-500m-cpu:
    enabled: true
    url: pvc://model-pvc?model=qwen:0.5b
    minReplicas: 2
    engine: OLlama
    resourceProfile: "cpu:1" 
    features: [TextGeneration]
EOF

sleep 15

# There are 2 replicas so send 10 requests to ensure that both replicas are used.
for i in {1..10}; do
  echo "Sending request $i"
  curl http://localhost:8000/openai/v1/completions \
    --max-time 600 \
    -H "Content-Type: application/json" \
    -d '{"model": "qwen-500m-cpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'
done

kubectl delete -f $REPO_DIR/test/e2e/engine-ollama-pvc/ollama-hydrate-job.yaml
kubectl delete -f $REPO_DIR/test/e2e/engine-ollama-pvc/pvc.yaml
kubectl delete -f $REPO_DIR/test/e2e/engine-ollama-pvc/pv.yaml