# LLM Load Balancing at Scale: A Consistent Hashing Approach with Bounded Loads

## Abstract:

In large-scale deployments of large language models (LLMs), efficient load balancing is paramount to reducing latency and maximizing throughput. We present a novel application of the Consistent Hashing with Bounded Loads (CHWBL) algorithm to the domain of LLM serving. Our approach dramatically reduces Time To First Token (TTFT) while improving overall Tokens Per Second (TPS) compared to the conventional strategies. This paper details the motivation, design, and performance benchmarks of our implementation within a Kubernetes-based system using vLLM.

## 1. Introduction

LLM serving engines face unique challenges when they operate under real-world workloads that increasingly involve long contexts. Effective load balancing strategies not only ensure low latency but reduce operational costs by improving overall system throughput. In this paper, we investigate how the CHWBL algorithm can be harnessed to improve performance in LLM serving infrastructures, with a particular focus on vLLM deployments on Kubernetes.

## 2. Background and Motivation

Before an inference engine such as vLLM can begin generating output tokens, it must process the input text during the “prefill phase”. The resulting intermediate representation is stored in a key-value (KV) cache—a technique known as prefix caching—which is especially beneficial for:

**Multi-turn Conversations:** In applications like chatbots (e.g., ChatGPT) or autonomous AI agents, every new turn appends to an evolving shared context. As the conversation grows, effective reuse of cached prefixes becomes critical to reducing latency.

<img src="../diagrams/multi-turn-clients.excalidraw.png" style="max-width:500px" alt="Multi-turn conversation diagram">

**Multi-threaded Requests with Shared Context:**
Scenarios that involve multiple queries against a single long document are particularly sensitive to the efficiency of prefix caching. When requests are executed concurrently, even slight improvements in cache hit rates can lead to substantial reductions in end-to-end latency.

<img src="../diagrams/multi-threaded-shared-context.excalidraw.png" style="max-width:500px" alt="Multi-threaded shared context diagram">
The conventional random routing strategy provided by Kubernetes often results in suboptimal cache utilization, leading to frequent cache evictions and degraded performance. A more consistent routing methodology is required to ensure that related requests consistently hit the same replica.

<img src="../diagrams/random-vs-consistent-hash.excalidraw.png" style="max-width:600px"></img>

## 3. Problem Statement

An effective load balancing strategy for LLM serving should satisfy the following criteria:

**Maximize Cache Utilization:** Route requests with common prefixes to vLLM replicas with hot caches.

**Adaptability:** Maintain consistent routing decisions even as the number of replicas changes.

**Load Distribution:** Prevent any single replica from becoming overloaded.

The Consistent Hashing with Bounded Loads algorithm inherently addresses these challenges, making it a compelling choice for LLM load balancing.

## 4. Proposed Approach: CHWBL for LLM Load Balancing

The CHWBL algorithm extends traditional consistent hashing by incorporating load bounds, ensuring that no individual replica receives more than its fair share of requests. This approach not only preserves cache affinity but also prevents the overloading of any single server, thereby optimizing overall system performance.

<figure> <img src="../diagrams/chwbl.excalidraw.png" style="max-width:600px" alt="CHWBL algorithm diagram"> <figcaption>Figure 1. Depiction of the CHWBL algorithm applied to LLM load balancing.</figcaption> </figure>

## 5. Implementation

We have integrated the CHWBL-based routing strategy into the KubeAI project under the PrefixHash configuration. This strategy functions as follows:

Request Inspection: The incoming request payload is analyzed.
Prefix Extraction: A configurable prefix is extracted from the request (for example, using the first user message in chat completions).
Hashing: The extracted prefix is hashed.
Replica Lookup: The hash value is used to select the appropriate vLLM replica using the CHWBL algorithm.
The configuration is specified on a per-model basis via the following YAML snippet:

```yaml
kind: Model
spec:
  # ...
  loadBalancing:
    strategy: PrefixHash
    # Optional parameters:
    prefixHash:
      meanLoadFactor: 125
      prefixCharLength: 100
      replication: 256
```

## 6. Evaluation

### 6.1. Scenarios

To evaluate the efficacy of the PrefixHash strategy, we conducted experiments using three distinct load balancing scenarios:

1. **Kubernetes Service (Random)**
    * Utilized the default Kubernetes Service, bypassing the KubeAI proxy.
    * Relied on iptables for proxying via kube-proxy.
2. **KubeAI (LeastLoad)**
    * Employed the KubeAI load balancer to route traffic to the replica with the fewest active requests.
3. **KubeAI (PrefixHash)**
    * Applied the CHWBL-driven PrefixHash strategy to route requests.

### 6.2 Setup

A Kubernetes cluster was configured with the following:

* **Hardware:** 8x L4 GPUs
* **Software:** 8x vLLM instances
* **Model:** Llama 3.1 8B
* **Dataset:** Message threads derived from ShareGPT
* **Workload:** A custom load generator simulating parallel chat completion threads, preserving conversation state by appending LLM responses in a loop.

### 6.3 Metrics
Our evaluation focused on three key performance metrics:

* **Time To First Token (TTFT):** The latency before the first token is generated.
* **Inter-Token Latency (ITL):** The delay between successive token generations.
* **Tokens Per Second (TPS):** The overall throughput of token generation.

### 6.4 Results

Results indicate that the PrefixHash strategy offers significant improvements in TTFT, ITL, and TPS compared to both the default Kubernetes Service and the LeastLoad strategy.

## 7. Conclusion

We have demonstrated that applying the CHWBL algorithm to LLM load balancing—via the PrefixHash strategy in KubeAI—can substantially enhance performance in real-world deployment scenarios. The improved cache hit rate and balanced load distribution led to reductions in latency and increases in throughput. Future work will focus on conducting broader experiments across diverse workloads along with benchmarking emerging strategies to ensure KubeAI always implements the most effective strategy for every type of workload.

## References

* [vLLM Documentation on Automatic Prefix Caching](https://docs.vllm.ai/en/latest/features/automatic_prefix_caching.html)
* [Google Research: Consistent Hashing with Bounded Loads.](https://research.google/blog/consistent-hashing-with-bounded-loads/)
* [KubeAI Load Balancing Concepts](https://www.kubeai.org/concepts/load-balancing/)