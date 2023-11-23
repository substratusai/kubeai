# Batch inference for OpenAI compatible APIs
This is a python program that will a configurable amount of concurrent requests.

## Usage
```
python main.py --requests-path gs://my-bucket/*.jsonl \
   --output-path gs://my-bucket/ \
   --flush-every 1000 \
   --url http://localhost:8080/v1/completions
```