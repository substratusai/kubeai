# Llama 3.2 11B Vision Instruct vLLM Benchmarks


Single L4 GPU vLLM 0.6.2
```
python3 benchmark_serving.py --backend openai \
    --base-url http://localhost:8000/openai \
    --dataset-name=sharegpt --dataset-path=ShareGPT_V3_unfiltered_cleaned_split.json \
    --model meta-llama-3.2-11b-vision-instruct \
    --seed 12345 --tokenizer neuralmagic/Llama-3.2-11B-Vision-Instruct-FP8-dynamic
============ Serving Benchmark Result ============
Successful requests:                     1000
Benchmark duration (s):                  681.93
Total input tokens:                      230969
Total generated tokens:                  194523
Request throughput (req/s):              1.47
Output token throughput (tok/s):         285.25
Total Token throughput (tok/s):          623.95
---------------Time to First Token----------------
Mean TTFT (ms):                          319146.12
Median TTFT (ms):                        322707.98
P99 TTFT (ms):                           642512.79
-----Time per Output Token (excl. 1st token)------
Mean TPOT (ms):                          54.84
Median TPOT (ms):                        53.66
P99 TPOT (ms):                           83.75
---------------Inter-token Latency----------------
Mean ITL (ms):                           54.09
Median ITL (ms):                         47.44
P99 ITL (ms):                            216.77
==================================================
```