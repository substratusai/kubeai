# LLM Load Balancing at Scale

**TLDR:** Applying the Consistent Hashing with Bounded Loads (CHWBL) algorithm to LLM Load Balancing results in dramatic performance improvements over the default random strategy built into Kubernetes.

## Introduction

Before an inference engine such as vLLM can start producing output tokens, it needs to first process the input text (this is called the "prefill phase"). The result of this phase is stored in a KV cache for future reference (this is called "prefix caching").

The impact of prefix caching can be significant, especially in multi-turn use-cases such as chat, whether the client is a human or an "agent". This is because multi-turn use-cases operate in a generate-append-generate loop, where the last response ends up being incorporated into the prefix for the next request.

<img src="../diagrams/multi-turn-clients.excalidraw.png" style="max-width:500px"></img>

When operating at scale (multiple replicas of vLLM) and under load (at the threshold of KV cache-space), choosing a load balancing strategy that can maximize cache-hits and minimize cache-evictions becomes critical.

The default random strategy built into Kubernetes leaves a lot of performance on the table. Some sort of consistent routing strategy would be better to keep a relevant cache on each replica.

<img src="../diagrams/random-vs-consistent-hash.excalidraw.png" style="max-width:600px"></img>

## Finding the right algorithm

The ideal load balancing algorithm should be able to:

1. Route requests with the same prefix to the same vLLM replica.
2. Keep routing as consistent as possible when the number of replicas changes.
3. Avoid overloading any single vLLM replica.

This is exactly the problem that the Consistent Hashing with Bounded Loads (CHWBL) algorithm was designed to tackle.

![CHWBL](../diagrams/chwbl.excalidraw.png)

## KubeAI implementation

KubeAI now provides a `PrefixHash` load balancing strategy that can be configured on a per-model basis.

```yaml
kind: Model
spec:
  # ...
  loadBalancing:
    strategy: PrefixHash
```

When using this strategy, KubeAI will:

1. Inspect the incoming request body.
2. Extract a prefix of up to a configured length. 
    * NOTE: For chat completion requests - the first user message is used.
3. Hash the extracted prefix.
4. Lookup the vLLM replica that has a hash value closest to the hash value of the request (unidirectional).
    * NOTE: The next closest replica is considered if the current replica is serving too many requests.

## Performance results

Three load balancing scenarios were tested:

1. Kubernetes Service (Random)
    * Implemented via a standard Kubernetes Service that bypassed KubeAI (avoided proxying overhead).
    * The kube-proxy was configured to use `iptables` proxying.
2. KubeAI (LeastLoad)
    * Proxied through KubeAI load balancer.
    * Routes traffic to the replica with the least number of in-flight requests.
3. KubeAI (PrefixHash)
    * Proxied through KubeAI load balancer.
    * Routes traffic to replicas according the CHWBL strategy described above.

Several key performance numbers were considered:

1. TTFT - Time To First Token - How long the user waits for the model to start generating output.
2. ITL - Inter-Token Latency - How long the user waits for each subsequent token to be generated.
3. TPS - Tokens Per Second (throughput) - The total number of tokens generated each second by the system.

Overview of the results:

Improvements to ITL and TPS were seen across the board when using the PrefixHash strategy, with the most dramatic improvement showing up in TTFT.

Summary of Time To First Token (TTFT):

When concurrency was increased **10x** (`800` -> `8000`), mean TTFT (Time To First Token) with a standard Kubernetes Service (Random) increased over **36x** (`1300 ms` -> `48500 ms`) while mean TTFT remained relatively **constant** for the PrefixHash strategy (`2XX ms` range).

Summary of Inter-Token Latency (ITL):

TODO

Summary of Tokens Per Second (TPS):

TODO

## Conclusion

TODO

## References

* https://docs.vllm.ai/en/latest/features/automatic_prefix_caching.html
* https://research.google/blog/consistent-hashing-with-bounded-loads/
* https://www.kubeai.org/concepts/load-balancing/