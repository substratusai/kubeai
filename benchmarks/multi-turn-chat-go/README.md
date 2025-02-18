# Benchmark

## Docker (Example)

Build the docker image.

```bash
make data
make build-docker-image
make push-docker-image
```

### Example: Ollama (with config flags)

Make sure the Ollama server is running on your machine.

```bash
docker run --network=host -e OPENAI_BASE_URL=http://host.docker.internal:11434/v1 us-central1-docker.pkg.dev/substratus-dev/default/benchmark-multi-turn-chat-go \  --threads ./data/tiny.json \
  --thread-count 4 \
  --request-model qwen2:0.5b \
  --tokenizer-model Qwen/Qwen2.5-VL-7B-Instruct \
  --max-concurrent-threads 2 \
  --max-completion-tokens 10 \
  --request-timeout 30s
```

### Example: OpenAI (with config file)

Make sure you have set `OPENAI_API_KEY`.

```bash
docker run --network=host -e OPENAI_API_KEY=$OPENAI_API_KEY -e OPENAI_BASE_URL=https://api.openai.com/v1 us-central1-docker.pkg.dev/substratus-dev/default/benchmark-multi-turn-chat-go --config ./example/openai-config.json --threads ./data/tiny.json
```

## Dataset

Prepare the data in the `data/` directory.

```bash
make data
```

## KubeAI

Create a cluster.

```bash
gcloud container clusters create cluster-1 \
    --zone us-central1-a \
    --node-locations us-central1-a --num-nodes 2 --machine-type e2-standard-16
```

Install KubeAI. 

```bash
helm repo add kubeai https://www.kubeai.org
helm repo update
helm upgrade --install kubeai kubeai/kubeai --set open-webui.enabled=false --set secrets.huggingface.token=$HUGGING_FACE_HUB_TOKEN
cat <<EOF > kubeai-models.yaml
catalog:
  llama-3.1-8b-instruct-cpu:
    enabled: true
    minReplicas: 2
    maxReplicas: 2
EOF

helm upgrade --install kubeai-models kubeai/models \
    -f ./kubeai-models.yaml
```

```bash
kubectl wait --timeout 10m --for=jsonpath='{.status.replicas.ready}'=2 model/llama-3.1-8b-instruct-cpu
kubectl create -f ./example/pod.yaml
```


## Development

Setup tokenizer python env.

```bash
python -m venv .venv
source .venv/bin/activate
pip install pydantic 'fastapi[standard]' transformers
```

Run the tokenizer api in another terminal.

```bash
TOKENIZER_MODEL="Qwen/Qwen2.5-VL-7B-Instruct" ./.venv/bin/fastapi run tokens.py --port 7000
```

Run the benchmark (against a local ollama instance).

```bash
OPENAI_BASE_URL=http://localhost:11434/v1 go run . --config ./example/ollama-config.json --threads ./data/tiny.json
```