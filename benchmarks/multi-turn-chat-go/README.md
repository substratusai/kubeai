# Benchmark

## E2E Run

Build the docker image.

```bash
make data
make build-docker-image
make push-docker-image
```

Run `run.ipynb`.


## Run with Docker

### Example: Ollama (with config flags)

Make sure the Ollama server is running on your machine.

```bash
docker run --network=host -e OPENAI_BASE_URL=http://host.docker.internal:11434/v1 $BENCH_IMAGE \
  --threads ./data/tiny.json \
  --thread-count 4 \
  --request-model qwen2:0.5b \
  --max-concurrent-threads 2 \
  --max-completion-tokens 10 \
  --request-timeout 30s
```

### Example: OpenAI (with config file)

Make sure you have set `OPENAI_API_KEY`.

```bash
docker run --network=host -e OPENAI_API_KEY=$OPENAI_API_KEY -e OPENAI_BASE_URL=https://api.openai.com/v1 $BENCH_IMAGE --config ./hack/openai-config.json --threads ./data/tiny.json
```


## Run with Go

Run the benchmark (against a local ollama instance).

```bash
OPENAI_BASE_URL=http://localhost:11434/v1 go run . --config ./hack/ollama-config.json --threads ./data/tiny.json
```