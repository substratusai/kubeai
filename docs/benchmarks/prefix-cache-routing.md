
Exploring how Prefix Cache aware routing affects
1. time to first token
2. total token throughput

Variables:
1. Concurrent requests per replica (50, 100, 200)
2.. Model and GPU type
  a. L4 GPU, 8 replicas, llama 3 8b
3. Dataset number of conversations

## model used

8 replicas of llama 3 8b and each uses a single L4 GPU


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

## 800 concurrent requests

```yaml
      containers:
        - name: benchmark-serving
          image: substratusai/benchmark_serving:v0.0.1
          args:
            - --base-url=http://kubeai/openai
            - --dataset-name=sharegpt
            - --dataset-path=/app/sharegpt_16_messages_or_more.json
            - --model=llama-3.1-8b-instruct-fp8-l4
            - --seed=12345
            - --tokenizer=neuralmagic/Meta-Llama-3.1-8B-Instruct-FP8
            - --request-rate=800
            - --max-concurrency=800
            - --num-prompts=8000
            - --max-conversations=800
      restartPolicy: Never
```

### without

```
============ Serving Benchmark Result ============
Successful requests:                     8000      
Benchmark duration (s):                  158.39    
Total input tokens:                      6656338   
Total generated tokens:                  608447    
Request throughput (req/s):              50.51     
Output token throughput (tok/s):         3841.42   
Total Token throughput (tok/s):          45866.16  
---------------Time to First Token----------------
Mean TTFT (ms):                          817.18    
Median TTFT (ms):                        494.28    
P99 TTFT (ms):                           5551.81   
-----Time per Output Token (excl. 1st token)------
Mean TPOT (ms):                          191.44    
Median TPOT (ms):                        183.18    
P99 TPOT (ms):                           520.48    
---------------Inter-token Latency----------------
Mean ITL (ms):                           176.03    
Median ITL (ms):                         124.55    
P99 ITL (ms):                            691.97    
==================================================
```

### with

```
============ Serving Benchmark Result ============
Successful requests:                     8000      
Benchmark duration (s):                  104.67    
Total input tokens:                      6656338   
Total generated tokens:                  608447    
Request throughput (req/s):              76.43     
Output token throughput (tok/s):         5813.11   
Total Token throughput (tok/s):          69407.79  
---------------Time to First Token----------------
Mean TTFT (ms):                          280.20    
Median TTFT (ms):                        239.80    
P99 TTFT (ms):                           1260.53   
-----Time per Output Token (excl. 1st token)------
Mean TPOT (ms):                          86.55     
Median TPOT (ms):                        91.13     
P99 TPOT (ms):                           139.47    
---------------Inter-token Latency----------------
Mean ITL (ms):                           85.78     
Median ITL (ms):                         77.35     
P99 ITL (ms):                            272.04    
==================================================
```



## 1600 concurrent requests

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: benchmark-serving
spec:
  template:
    spec:
      containers:
        - name: benchmark-serving
          image: substratusai/benchmark_serving:v0.0.1
          args:
            - --base-url=http://kubeai/openai
            - --dataset-name=sharegpt
            - --dataset-path=/app/sharegpt_16_messages_or_more.json
            - --model=llama-3.1-8b-instruct-fp8-l4
            - --seed=12345
            - --tokenizer=neuralmagic/Meta-Llama-3.1-8B-Instruct-FP8
            - --request-rate=200
            - --max-concurrency=1600
            - --num-prompts=8000
            - --max-conversations=800
      restartPolicy: Never
```


### without prefix aware lb
```
============ Serving Benchmark Result ============
Successful requests:                     8000      
Benchmark duration (s):                  153.02    
Total input tokens:                      6656338   
Total generated tokens:                  608447    
Request throughput (req/s):              52.28     
Output token throughput (tok/s):         3976.28   
Total Token throughput (tok/s):          47476.29  
---------------Time to First Token----------------
Mean TTFT (ms):                          10579.01  
Median TTFT (ms):                        11501.96  
P99 TTFT (ms):                           15514.10  
-----Time per Output Token (excl. 1st token)------
Mean TPOT (ms):                          212.39    
Median TPOT (ms):                        202.98    
P99 TPOT (ms):                           613.06    
---------------Inter-token Latency----------------
Mean ITL (ms):                           193.34    
Median ITL (ms):                         92.65     
P99 ITL (ms):                            747.65    
==================================================
```

### with prefix aware LB
```
============ Serving Benchmark Result ============
Successful requests:                     8000      
Benchmark duration (s):                  110.00    
Total input tokens:                      6656338   
Total generated tokens:                  608447    
Request throughput (req/s):              72.73     
Output token throughput (tok/s):         5531.31   
Total Token throughput (tok/s):          66043.15  
---------------Time to First Token----------------
Mean TTFT (ms):                          196.13    
Median TTFT (ms):                        184.29    
P99 TTFT (ms):                           492.33    
-----Time per Output Token (excl. 1st token)------
Mean TPOT (ms):                          78.51     
Median TPOT (ms):                        81.50     
P99 TPOT (ms):                           117.36    
---------------Inter-token Latency----------------
Mean ITL (ms):                           79.20     
Median ITL (ms):                         70.36     
P99 ITL (ms):                            249.71    
==================================================
```

## 3200 concurrent requests

job:
```yaml
        - name: benchmark-serving
          image: substratusai/benchmark_serving:v0.0.1
          args:
            - --base-url=http://kubeai/openai
            - --dataset-name=sharegpt
            - --dataset-path=/app/sharegpt_16_messages_or_more.json
            - --model=llama-3.1-8b-instruct-fp8-l4
            - --seed=12345
            - --tokenizer=neuralmagic/Meta-Llama-3.1-8B-Instruct-FP8
            - --request-rate=200
            - --max-concurrency=3200
            - --num-prompts=8000
            - --max-conversations=800
```

### without
```
============ Serving Benchmark Result ============
Successful requests:                     8000      
Benchmark duration (s):                  152.43    
Total input tokens:                      6656338   
Total generated tokens:                  608447    
Request throughput (req/s):              52.48     
Output token throughput (tok/s):         3991.56   
Total Token throughput (tok/s):          47658.74  
---------------Time to First Token----------------
Mean TTFT (ms):                          24147.86  
Median TTFT (ms):                        25580.61  
P99 TTFT (ms):                           46021.48  
-----Time per Output Token (excl. 1st token)------
Mean TPOT (ms):                          211.98    
Median TPOT (ms):                        201.97    
P99 TPOT (ms):                           598.14    
---------------Inter-token Latency----------------
Mean ITL (ms):                           192.94    
Median ITL (ms):                         93.29     
P99 ITL (ms):                            721.71    
==================================================
```

### with
```
============ Serving Benchmark Result ============
Successful requests:                     8000      
Benchmark duration (s):                  111.37    
Total input tokens:                      6656338   
Total generated tokens:                  608447    
Request throughput (req/s):              71.84     
Output token throughput (tok/s):         5463.50   
Total Token throughput (tok/s):          65233.60  
---------------Time to First Token----------------
Mean TTFT (ms):                          213.92    
Median TTFT (ms):                        188.53    
P99 TTFT (ms):                           838.35    
-----Time per Output Token (excl. 1st token)------
Mean TPOT (ms):                          78.73     
Median TPOT (ms):                        82.17     
P99 TPOT (ms):                           122.60    
---------------Inter-token Latency----------------
Mean ITL (ms):                           78.49     
Median ITL (ms):                         70.32     
P99 ITL (ms):                            242.44    
==================================================
```

## max concurrency 8k
```
        - name: benchmark-serving
          image: substratusai/benchmark_serving:v0.0.1
          args:
            - --base-url=http://kubeai/openai
            - --dataset-name=sharegpt
            - --dataset-path=/app/sharegpt_16_messages_or_more.json
            - --model=llama-3.1-8b-instruct-fp8-l4
            - --seed=12345
            - --tokenizer=neuralmagic/Meta-Llama-3.1-8B-Instruct-FP8
            - --request-rate=800
            - --max-concurrency=8000
            - --num-prompts=8000
            - --max-conversations=800
```

### without
```
============ Serving Benchmark Result ============
Successful requests:                     8000      
Benchmark duration (s):                  152.59    
Total input tokens:                      6656338   
Total generated tokens:                  608447    
Request throughput (req/s):              52.43     
Output token throughput (tok/s):         3987.46   
Total Token throughput (tok/s):          47609.83  
---------------Time to First Token----------------
Mean TTFT (ms):                          39163.80  
Median TTFT (ms):                        40140.70  
P99 TTFT (ms):                           78489.26  
-----Time per Output Token (excl. 1st token)------
Mean TPOT (ms):                          214.09    
Median TPOT (ms):                        205.62    
P99 TPOT (ms):                           623.61    
---------------Inter-token Latency----------------
Mean ITL (ms):                           194.44    
Median ITL (ms):                         90.36     
P99 ITL (ms):                            725.95    
==================================================
```

### with
```
============ Serving Benchmark Result ============
Successful requests:                     8000      
Benchmark duration (s):                  107.89    
Total input tokens:                      6656338   
Total generated tokens:                  608447    
Request throughput (req/s):              74.15     
Output token throughput (tok/s):         5639.40   
Total Token throughput (tok/s):          67333.71  
---------------Time to First Token----------------
Mean TTFT (ms):                          237.06    
Median TTFT (ms):                        219.27    
P99 TTFT (ms):                           619.65    
-----Time per Output Token (excl. 1st token)------
Mean TPOT (ms):                          79.99     
Median TPOT (ms):                        81.76     
P99 TPOT (ms):                           124.28    
---------------Inter-token Latency----------------
Mean ITL (ms):                           79.90     
Median ITL (ms):                         71.31     
P99 ITL (ms):                            303.14    
==================================================
```