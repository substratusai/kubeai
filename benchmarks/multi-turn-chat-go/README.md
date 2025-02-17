# Benchmark

Prepare the data.

```bash
make data
```

Run the benchmark.

```bash
OPENAI_BASE_URL=localhost:9999/v1 go run . --config ./example/config.json --threads ./data/threads.json
```