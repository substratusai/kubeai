# Results

Under specific conditions:

* Restricted GPU memory
* Low `max_tokens` to be generated
* Chat threads with decently long user messages

Prefix hashing was shown to have `34%` decrease in average time per token.

`712.11ms (LeastLoad) --> 469.34ms (PrefixHash)`

## Steps taken

```bash
gcloud container clusters create-auto cluster-1 \
    --location=us-central1
skaffold run -f ./skaffold.yaml --tail --port-forward --profile kubeai-only-gke --default-repo us-central1-docker.pkg.dev/substratus-dev

cd ./benchmarks/chat
make data
export IMG=us-central1-docker.pkg.dev/substratus-dev/default/kubeai-benchmark-chat:v0.0.2
docker build -t $IMG . && docker push $IMG

kubectl apply -f ./scenarios/least-load-vs-prefix-hash/model.yaml
kubectl apply -f ./scenarios/least-load-vs-prefix-hash/pod.yaml

# Run 2x (to ensure both cases start with a preloaded cache)
kubectl exec -it chat-benchmark -- SCENARIO=least-load-vs-prefix-hash make run

kubectl patch model llama-3.1-8b-instruct-fp8-l4 --type='merge' -p '{"spec": {"loadBalancing": {"strategy": "PrefixHash"}}}'
kubectl exec -it chat-benchmark -- SCENARIO=least-load-vs-prefix-hash make run
```

## Next Steps

* Rerun with increased replicas (i.e. 10 instead of 2)

## Benchmark Output

### LeastLoad

```
         /\      Grafana   /‾‾/  
    /\  /  \     |\  __   /  /   
   /  \/    \    | |/ /  /   ‾‾\ 
  /          \   |   (  |  (‾)  |
 / __________ \  |_|\_\  \_____/ 

     execution: local
        script: ./k6.js
        output: -

     scenarios: (100.00%) 1 scenario, 80 max VUs, 10m30s max duration (incl. graceful stop):
              * chat: 1000 iterations shared among 80 VUs (maxDuration: 10m0s, gracefulStop: 30s)


     ✓ Post status is 200

     checks.........................: 100.00% 7341 out of 7341
     data_received..................: 4.7 MB  7.9 kB/s
     data_sent......................: 25 MB   42 kB/s
     http_req_blocked...............: avg=161.4µs  min=2.83µs   med=5.8µs    max=16.67ms  p(90)=8.06µs   p(95)=10.19µs 
     http_req_connecting............: avg=55.73µs  min=0s       med=0s       max=8.41ms   p(90)=0s       p(95)=0s      
     http_req_duration..............: avg=6.31s    min=165.25ms med=6.66s    max=11.65s   p(90)=8.55s    p(95)=9.07s   
       { expected_response:true }...: avg=6.31s    min=165.25ms med=6.66s    max=11.65s   p(90)=8.55s    p(95)=9.07s   
   ✓ http_req_failed................: 0.00%   0 out of 7341
     http_req_receiving.............: avg=84.64µs  min=29.4µs   med=74.05µs  max=732.69µs p(90)=129.94µs p(95)=154.19µs
     http_req_sending...............: avg=68µs     min=12.1µs   med=32.3µs   max=1.38ms   p(90)=144.04µs p(95)=173.19µs
     http_req_tls_handshaking.......: avg=0s       min=0s       med=0s       max=0s       p(90)=0s       p(95)=0s      
     http_req_waiting...............: avg=6.31s    min=165.04ms med=6.66s    max=11.65s   p(90)=8.55s    p(95)=9.07s   
     http_reqs......................: 7341    12.422953/s
     input_tokens...................: 4990223 8444.803735/s
     iteration_duration.............: avg=46.39s   min=6.73s    med=41.26s   max=4m13s    p(90)=1m8s     p(95)=1m28s   
     iterations.....................: 1000    1.69227/s
     new_tokens.....................: 68062   115.179268/s
     time_per_token.................: avg=712.11ms min=39.56ms  med=703.28ms max=2.69s    p(90)=928.58ms p(95)=1.09s   
     tokens.........................: 5058285 8559.983003/s
     vus............................: 1       min=0            max=80
     vus_max........................: 80      min=21           max=80


running (09m50.9s), 00/80 VUs, 1000 complete and 0 interrupted iterations
chat ✓ [======================================] 80 VUs  09m50.9s/10m0s  1000/1000 shared iters
```

### PrefixHash

```
         /\      Grafana   /‾‾/  
    /\  /  \     |\  __   /  /   
   /  \/    \    | |/ /  /   ‾‾\ 
  /          \   |   (  |  (‾)  |
 / __________ \  |_|\_\  \_____/ 

     execution: local
        script: ./k6.js
        output: -

     scenarios: (100.00%) 1 scenario, 80 max VUs, 10m30s max duration (incl. graceful stop):
              * chat: 1000 iterations shared among 80 VUs (maxDuration: 10m0s, gracefulStop: 30s)


     ✓ Post status is 200

     checks.........................: 100.00% 7341 out of 7341
     data_received..................: 4.7 MB  12 kB/s
     data_sent......................: 25 MB   65 kB/s
     http_req_blocked...............: avg=268.24µs min=2.94µs   med=5.76µs   max=28.19ms  p(90)=8.17µs   p(95)=10.41µs 
     http_req_connecting............: avg=136.33µs min=0s       med=0s       max=17.7ms   p(90)=0s       p(95)=0s      
     http_req_duration..............: avg=4.08s    min=151.9ms  med=2.45s    max=12.32s   p(90)=9.63s    p(95)=10.26s  
       { expected_response:true }...: avg=4.08s    min=151.9ms  med=2.45s    max=12.32s   p(90)=9.63s    p(95)=10.26s  
   ✓ http_req_failed................: 0.00%   0 out of 7341
     http_req_receiving.............: avg=81.81µs  min=28.68µs  med=72.08µs  max=786.09µs p(90)=125.04µs p(95)=148.6µs 
     http_req_sending...............: avg=63.61µs  min=11.85µs  med=31.65µs  max=1.59ms   p(90)=136.85µs p(95)=161.88µs
     http_req_tls_handshaking.......: avg=0s       min=0s       med=0s       max=0s       p(90)=0s       p(95)=0s      
     http_req_waiting...............: avg=4.08s    min=151.81ms med=2.45s    max=12.32s   p(90)=9.63s    p(95)=10.26s  
     http_reqs......................: 7341    19.230625/s
     input_tokens...................: 4990576 13073.409349/s
     iteration_duration.............: avg=29.98s   min=2.37s    med=20.29s   max=2m53s    p(90)=1m1s     p(95)=1m18s   
     iterations.....................: 1000    2.619619/s
     new_tokens.....................: 68218   178.705191/s
     time_per_token.................: avg=469.34ms min=44.2ms   med=257.72ms max=3.86s    p(90)=1s       p(95)=1.1s    
     tokens.........................: 5058794 13252.11454/s
     vus............................: 3       min=0            max=80
     vus_max........................: 80      min=19           max=80


running (06m21.7s), 00/80 VUs, 1000 complete and 0 interrupted iterations
chat ✓ [======================================] 80 VUs  06m21.7s/10m0s  1000/1000 shared iters
```
