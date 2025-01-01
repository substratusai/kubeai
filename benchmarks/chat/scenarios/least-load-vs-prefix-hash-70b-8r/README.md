# Results

Under specific conditions:

* Restricted GPU memory
* Low `max_tokens` to be generated
* Chat threads with decently long user messages

Prefix hashing was shown to have `34%` decrease in average time per token.

`712.11ms (LeastLoad) --> 469.34ms (PrefixHash)`

## Steps taken

```bash
export SCENARIO=least-load-vs-prefix-hash-70b-8r
export PROJECT_ID=$(gcloud config get-value project)
export IMG=us-central1-docker.pkg.dev/$PROJECT_ID/default/kubeai-benchmark-chat:v0.0.2

cd ./benchmarks/chat
make data
gcloud builds submit . -t $IMG
# docker build -t $IMG . && docker push $IMG

kubectl apply -f ./scenarios/$SCENARIO/model.yaml
envsubst < ./scenarios/$SCENARIO/pod.yaml | kubectl apply -f -

# Had to manually copy the file for some reason
# TODO fix Dockerfile to ensure it gets added
kubectl cp data/message-threads.json chat-benchmark:/work/data/

# Run 2x (to ensure both cases start with a preloaded cache)
# kubectl exec -it chat-benchmark -- SCENARIO=$SCENARIO make run
kubectl exec -it chat-benchmark -- bash -c "SCENARIO=$SCENARIO make run"

kubectl patch model llama-3.1-70b-instruct-fp8-h100 --type='merge' -p '{"spec": {"loadBalancing": {"strategy": "PrefixHash"}}}'
kubectl exec -it chat-benchmark -- SCENARIO=$SCENARIO make run
```


## Benchmark Output

### LeastLoad - single replica

```
     scenarios: (100.00%) 1 scenario, 320 max VUs, 10m30s max duration (incl. graceful stop):
              * chat: 1000 iterations shared among 320 VUs (maxDuration: 10m0s, gracefulStop: 30s)


     ✓ Post status is 200

     checks.........................: 100.00% 6094 out of 6094
     data_received..................: 3.9 MB  6.2 kB/s
     data_sent......................: 20 MB   32 kB/s
     dropped_iterations.............: 23      0.036508/s
     http_req_blocked...............: avg=1.52ms   min=1.72µs   med=4.52µs  max=47.12ms p(90)=7.64µs   p(95)=14.47ms
     http_req_connecting............: avg=79.02µs  min=0s       med=0s      max=13.96ms p(90)=0s       p(95)=119.84µs
     http_req_duration..............: avg=32.48s   min=6.25s    med=37.74s  max=50.64s  p(90)=43.38s   p(95)=45.81s
       { expected_response:true }...: avg=32.48s   min=6.25s    med=37.74s  max=50.64s  p(90)=43.38s   p(95)=45.81s
   ✓ http_req_failed................: 0.00%   0 out of 6094
     http_req_receiving.............: avg=75.82µs  min=19.9µs   med=68.09µs max=2.04ms  p(90)=115.16µs p(95)=134.82µs
     http_req_sending...............: avg=103.99µs min=8.22µs   med=27.04µs max=33.92ms p(90)=126.5µs  p(95)=186.9µs
     http_req_tls_handshaking.......: avg=0s       min=0s       med=0s      max=0s      p(90)=0s       p(95)=0s
     http_req_waiting...............: avg=32.48s   min=6.25s    med=37.73s  max=50.64s  p(90)=43.38s   p(95)=45.81s
     http_reqs......................: 6094    9.672953/s
     input_tokens...................: 3859568 6126.258596/s
     iteration_duration.............: avg=3m49s    min=1m30s    med=3m23s   max=10m17s  p(90)=5m41s    p(95)=6m36s
     iterations.....................: 728     1.155548/s
     new_tokens.....................: 56340   89.42799/s
     time_per_token.................: avg=4.03s    min=625.66ms med=3.87s   max=22.72s  p(90)=5s       p(95)=11.69s
     tokens.........................: 3915908 6215.686586/s
     vus............................: 252     min=0            max=320
     vus_max........................: 320     min=25           max=320


running (10m30.0s), 000/320 VUs, 728 complete and 249 interrupted iterations
chat ✗ [==========================>-----------] 320 VUs  10m30.0s/10m0s  0728/1000 shared iters
```

## LeastLoad - 8 replicas 1st run

```
     scenarios: (100.00%) 1 scenario, 320 max VUs, 10m30s max duration (incl. graceful stop):
              * chat: 1000 iterations shared among 320 VUs (maxDuration: 10m0s, gracefulStop: 30s)


     ✓ Post status is 200

     checks.........................: 100.00% 7341 out of 7341
     data_received..................: 4.7 MB  47 kB/s
     data_sent......................: 25 MB   250 kB/s
     http_req_blocked...............: avg=280.95µs min=1.57µs   med=4.13µs   max=28.71ms p(90)=6.86µs   p(95)=32.09µs
     http_req_connecting............: avg=55.16µs  min=0s       med=0s       max=19.59ms p(90)=0s       p(95)=0s
     http_req_duration..............: avg=3.67s    min=112.34ms med=3.65s    max=8.58s   p(90)=6.09s    p(95)=6.56s
       { expected_response:true }...: avg=3.67s    min=112.34ms med=3.65s    max=8.58s   p(90)=6.09s    p(95)=6.56s
   ✓ http_req_failed................: 0.00%   0 out of 7341
     http_req_receiving.............: avg=75.3µs   min=18.48µs  med=62.57µs  max=2.87ms  p(90)=118.19µs p(95)=139.71µs
     http_req_sending...............: avg=100.92µs min=8.74µs   med=29.1µs   max=24.35ms p(90)=129.08µs p(95)=164.54µs
     http_req_tls_handshaking.......: avg=0s       min=0s       med=0s       max=0s      p(90)=0s       p(95)=0s
     http_req_waiting...............: avg=3.67s    min=112.2ms  med=3.65s    max=8.58s   p(90)=6.09s    p(95)=6.56s
     http_reqs......................: 7341    73.808399/s
     input_tokens...................: 4990165 50172.468256/s
     iteration_duration.............: avg=26.96s   min=6.17s    med=24.73s   max=1m30s   p(90)=41.36s   p(95)=48.91s
     iterations.....................: 1000    10.05427/s
     new_tokens.....................: 67808   681.759967/s
     time_per_token.................: avg=419.15ms min=34.84ms  med=397.78ms max=2.37s   p(90)=662.6ms  p(95)=781.79ms
     tokens.........................: 5057973 50854.228224/s
     vus............................: 1       min=0            max=320
     vus_max........................: 320     min=22           max=320


running (01m39.5s), 000/320 VUs, 1000 complete and 0 interrupted iterations
chat ✓ [======================================] 320 VUs  01m39.5s/10m0s  1000/1000 shared iters
```

## LeastLoad - 8 replicas 2nd run

```
     scenarios: (100.00%) 1 scenario, 320 max VUs, 10m30s max duration (incl. graceful stop):
              * chat: 1000 iterations shared among 320 VUs (maxDuration: 10m0s, gracefulStop: 30s)


     ✓ Post status is 200

     checks.........................: 100.00% 7341 out of 7341
     data_received..................: 4.7 MB  49 kB/s
     data_sent......................: 25 MB   259 kB/s
     http_req_blocked...............: avg=856.57µs min=1.6µs    med=4.23µs   max=33.05ms p(90)=7.16µs   p(95)=32.24µs
     http_req_connecting............: avg=107.71µs min=0s       med=0s       max=28.11ms p(90)=0s       p(95)=0s
     http_req_duration..............: avg=3.54s    min=131.17ms med=3.53s    max=9.66s   p(90)=5.95s    p(95)=6.53s
       { expected_response:true }...: avg=3.54s    min=131.17ms med=3.53s    max=9.66s   p(90)=5.95s    p(95)=6.53s
   ✓ http_req_failed................: 0.00%   0 out of 7341
     http_req_receiving.............: avg=76.78µs  min=20.42µs  med=63.93µs  max=3.16ms  p(90)=119.07µs p(95)=138.94µs
     http_req_sending...............: avg=153.18µs min=8.93µs   med=29.5µs   max=14.71ms p(90)=129.95µs p(95)=173.11µs
     http_req_tls_handshaking.......: avg=0s       min=0s       med=0s       max=0s      p(90)=0s       p(95)=0s
     http_req_waiting...............: avg=3.54s    min=130.82ms med=3.53s    max=9.66s   p(90)=5.95s    p(95)=6.53s
     http_reqs......................: 7341    76.270469/s
     input_tokens...................: 4990249 51846.973437/s
     iteration_duration.............: avg=26.06s   min=3.61s    med=24.15s   max=1m25s   p(90)=39.9s    p(95)=48.14s
     iterations.....................: 1000    10.389657/s
     new_tokens.....................: 67790   704.314821/s
     time_per_token.................: avg=405.39ms min=34.22ms  med=384.49ms max=2.2s    p(90)=650.92ms p(95)=749.72ms
     tokens.........................: 5058039 52551.288258/s
     vus............................: 1       min=0            max=320
     vus_max........................: 320     min=19           max=320


running (01m36.2s), 000/320 VUs, 1000 complete and 0 interrupted iterations
chat ✓ [======================================] 320 VUs  01m36.2s/10m0s  1000/1000 shared iters
```

### PrefixHash - 3rd run

```
     scenarios: (100.00%) 1 scenario, 320 max VUs, 10m30s max duration (incl. graceful stop):
              * chat: 1000 iterations shared among 320 VUs (maxDuration: 10m0s, gracefulStop: 30s)


     ✓ Post status is 200

     checks.........................: 100.00% 7341 out of 7341
     data_received..................: 4.7 MB  55 kB/s
     data_sent......................: 25 MB   288 kB/s
     http_req_blocked...............: avg=833.58µs min=1.61µs  med=4.34µs   max=41.24ms p(90)=10.84µs  p(95)=35.22µs
     http_req_connecting............: avg=243.25µs min=0s      med=0s       max=23.94ms p(90)=0s       p(95)=0s
     http_req_duration..............: avg=3.13s    min=83.91ms med=2.22s    max=10.71s  p(90)=6.67s    p(95)=7.33s
       { expected_response:true }...: avg=3.13s    min=83.91ms med=2.22s    max=10.71s  p(90)=6.67s    p(95)=7.33s
   ✓ http_req_failed................: 0.00%   0 out of 7341
     http_req_receiving.............: avg=75.62µs  min=19.77µs med=71.23µs  max=1.99ms  p(90)=118.68µs p(95)=138.44µs
     http_req_sending...............: avg=135.04µs min=7.79µs  med=30.48µs  max=15.02ms p(90)=137.44µs p(95)=181.62µs
     http_req_tls_handshaking.......: avg=0s       min=0s      med=0s       max=0s      p(90)=0s       p(95)=0s
     http_req_waiting...............: avg=3.13s    min=83.79ms med=2.22s    max=10.71s  p(90)=6.67s    p(95)=7.33s
     http_reqs......................: 7341    85.023164/s
     input_tokens...................: 4989621 57789.588176/s
     iteration_duration.............: avg=23.03s   min=1.71s   med=22.05s   max=1m20s   p(90)=41.36s   p(95)=49.67s
     iterations.....................: 1000    11.581959/s
     new_tokens.....................: 67718   784.307131/s
     time_per_token.................: avg=361.07ms min=35.86ms med=235.35ms max=2.78s   p(90)=723.57ms p(95)=827ms
     tokens.........................: 5057339 58573.895307/s
     vus............................: 1       min=0            max=320
     vus_max........................: 320     min=21           max=320


running (01m26.3s), 000/320 VUs, 1000 complete and 0 interrupted iterations
chat ✓ [======================================] 320 VUs  01m26.3s/10m0s  1000/1000 shared iters
```

