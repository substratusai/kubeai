# Benchmark

## Docker (Example)

Build the docker image.

```bash
make build-docker-image
```

### Ollama

Make sure the Ollama server is running on your machine.

```bash
docker run --network=host -e OPENAI_BASE_URL=http://host.docker.internal:11434/v1 us-central1-docker.pkg.dev/substratus-dev/default/benchmark-multi-turn-chat-go --config ./example/ollama-config.json --threads ./data/tiny.json
```

### OpenAI

Make sure the Ollama server is running on your machine.

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
helm install kubeai kubeai/kubeai --wait --timeout 10m
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