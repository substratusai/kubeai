# Batch inference
This directory contains a simple python script that can be used to perform batch inference.

The input is expected to be a newline delimited JSON file with the following format.
The JSON object can contain any fields. The JSON object will be passed as-is to the inference server.

The script will send a POST request to the inference server with the JSON object as the request body.

The concurrent requests are limited by the `--concurrency` flag.

## Examples
Batch inference to OpenAI compatible API. The request bodies are stored in a GCS bucket and the output is saved to a GCS bucket:
```sh
python main.py --requests-path gs://my-bucket/requests.jsonl \
   --output-path gs://my-bucket/output.jsonl \
   --flush-every 1000 \
   --concurrency 100 \
   --url http://localhost:8080/v1/completions
```

## Flags
```sh
usage: main.py [-h] [--url URL] [--requests-path REQUESTS_PATH] [--output-path OUTPUT_PATH]
               [--flush-every FLUSH_EVERY] [--concurrency CONCURRENCY]

Test Lingo using Python OpenAI API

options:
  -h, --help            show this help message and exit
  --url URL
  --requests-path REQUESTS_PATH
                        Path to read requests file
  --output-path OUTPUT_PATH
                        Path to write output files
  --flush-every FLUSH_EVERY
                        Number of requests to flush to disk
  --concurrency CONCURRENCY
                        Number of concurrent requests. Defaults to 100
```