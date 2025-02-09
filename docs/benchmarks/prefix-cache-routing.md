
Exploring how Prefix Cache aware routing affects
1. time to first token
2. total token throughput

Variables:
1. Concurrent requests per replica (50, 100, 200)
2.. Model and GPU type
  a. L4 GPU, 8 replicas, llama 3 8b
  b. H100 GPU, 8 replicas, llama 3.3 70b
3. Dataset number of conversations

##  8 replicas l4 GPU llama 3.1 8b

model:
```yaml
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: llama-3.1-8b-instruct-fp8-l4
spec:
  features: [TextGeneration]
  url: hf://neuralmagic/Meta-Llama-3.1-8B-Instruct-FP8
  engine: VLLM
  env:
    # VLLM_ATTENTION_BACKEND: "FLASHINFER"
    VLLM_USE_V1: "1"
  args:
    - --enable-prefix-caching
    - --max-model-len=16384
    - --max-num-batched-token=16384
    - --gpu-memory-utilization=0.95
    - --disable-log-requests
    - --kv-cache-dtype=fp8
  resourceProfile: nvidia-gpu-l4:1
  minReplicas: 8
  maxReplicas: 8
```

### No prefix aware caching
```
python3 benchmark_serving.py --backend openai \
    --base-url http://localhost:8000/openai \
    --dataset-name=sharegpt --dataset-path=sharegpt_16_messages_or_more.json \
    --model llama-3.1-8b-instruct-fp8-l4 \
    --seed 12345 \
    --tokenizer neuralmagic/Meta-Llama-3.1-8B-Instruct-FP8 \
    --request-rate 200 \
    --max-concurrency 400 \
    --num-prompts 2000
```

```
============ Serving Benchmark Result ============
Successful requests:                     2000      
Benchmark duration (s):                  85.40     
Total input tokens:                      1774781   
Total generated tokens:                  163963    
Request throughput (req/s):              23.42     
Output token throughput (tok/s):         1919.97   
Total Token throughput (tok/s):          22702.31  
---------------Time to First Token----------------
Mean TTFT (ms):                          1560.80   
Median TTFT (ms):                        768.12    
P99 TTFT (ms):                           7660.92   
-----Time per Output Token (excl. 1st token)------
Mean TPOT (ms):                          175.87    
Median TPOT (ms):                        156.81    
P99 TPOT (ms):                           637.14    
---------------Inter-token Latency----------------
Mean ITL (ms):                           144.47    
Median ITL (ms):                         69.26     
P99 ITL (ms):                            785.28    
==================================================
```

### With prefix aware caching


```
============ Serving Benchmark Result ============
Successful requests:                     2000      
Benchmark duration (s):                  83.90     
Total input tokens:                      1774781   
Total generated tokens:                  163963    
Request throughput (req/s):              23.84     
Output token throughput (tok/s):         1954.35   
Total Token throughput (tok/s):          23108.74  
---------------Time to First Token----------------
Mean TTFT (ms):                          1644.32   
Median TTFT (ms):                        816.65    
P99 TTFT (ms):                           8097.08   
-----Time per Output Token (excl. 1st token)------
Mean TPOT (ms):                          176.75    
Median TPOT (ms):                        150.60    
P99 TPOT (ms):                           725.76    
---------------Inter-token Latency----------------
Mean ITL (ms):                           138.42    
Median ITL (ms):                         68.05     
P99 ITL (ms):                            867.75    
==================================================
```