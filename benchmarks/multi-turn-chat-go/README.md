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
kind create cluster
```

Install KubeAI. 

```bash
helm repo add kubeai https://www.kubeai.org
helm repo update
helm install kubeai kubeai/kubeai --set open-webui.enabled=false
cat <<EOF > kubeai-models.yaml
catalog:
  deepseek-r1-1.5b-cpu:
    enabled: true
    features: [TextGeneration]
    url: 'ollama://deepseek-r1:1.5b'
    engine: OLlama
    minReplicas: 1
    resourceProfile: 'cpu:4'
  qwen2-500m-cpu:
    enabled: true
  nomic-embed-text-cpu:
    enabled: true
EOF

helm install kubeai-models kubeai/models \
    -f ./kubeai-models.yaml
```

```bash
kubectl create secret generic bench-config --from-file=config.yaml=./example/kubeai-config.json
kubectl wait --timeout 10m --for=jsonpath='{.status.replicas.ready}'=1 model/deepseek-r1-1.5b-cpu
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