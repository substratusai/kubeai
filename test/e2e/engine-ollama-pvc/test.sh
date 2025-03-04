#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

models_release="kubeai-models"

PV_HOST_PATH=/usr/share/ollama/.ollama/models/

mkdir -p ${PV_HOST_PATH}

# Execute into the kind container - ollama pull
kind_container=$(docker ps --filter "name=kind-control-plane" --format "{{.ID}}")
# pull qwen:0.5b model into /usr/share/ollama/.ollama/models/
docker exec -i $kind_container bash -c "
  mkdir -p ${PV_HOST_PATH}
  curl -L https://ollama.com/download/ollama-linux-amd64.tgz -o ollama-linux-amd64.tgz
  tar -C /usr -xzf ollama-linux-amd64.tgz
  ollama pull qwen:0.5b"

kubectl apply -f $REPO_DIR/test/e2e/engine-ollama-pvc/pv.yaml
kubectl apply -f $REPO_DIR/test/e2e/engine-ollama-pvc/pvc.yaml

helm install $models_release $REPO_DIR/charts/models -f - <<EOF
catalog:
  qwen-500m:
    enabled: true
    url: pvc://model-pvc?model=qwen:0.5b
    minReplicas: 2
    engine: OLlama
EOF

sleep 5

# There are 2 replicas so send 10 requests to ensure that both replicas are used.
for i in {1..10}; do
  echo "Sending request $i"
  curl http://localhost:8000/openai/v1/completions \
    --max-time 600 \
    -H "Content-Type: application/json" \
    -d '{"model": "opt-125m-cpu", "prompt": "Who was the first president of the United States?", "max_tokens": 40}'
done

sleep 300

helm uninstall kubeai-models # cleans up above model helm chart on success
kubectl delete -f $REPO_DIR/test/e2e/engine-ollama-pvc/pv.yaml
kubectl delete -f $REPO_DIR/test/e2e/engine-ollama-pvc/pvc.yaml