# Install models

This guide provides instructions on how to configure KubeAI models.

## Installing models with helm

KubeAI provides a [chart](https://github.com/substratusai/kubeai/blob/main/charts/models) that contains preconfigured models.

### Preconfigured models with helm

When you are defining Helm values for the `kubeai/models` chart you can install a preconfigured Model by setting `enabled: true`. You can view a list of all preconfigured models in the chart's [default values file](https://github.com/substratusai/kubeai/blob/main/charts/models/values.yaml). 

```yaml
# helm-values.yaml
catalog:
  llama-3.1-8b-instruct-fp8-l4:
    enabled: true
```

You can optionally override preconfigured settings, for example, `resourceProfile`:

```yaml
# helm-values.yaml
catalog:
  llama-3.1-8b-instruct-fp8-l4:
    enabled: true
    resourceProfile: nvidia-gpu-l4:2 # Require "2 NVIDIA L4 GPUs"
```

### Custom models with helm

If you prefer to add a custom model via the same Helm chart you use for installed KubeAI, you can add your custom model entry into the `.catalog` array of your existing values file for the `kubeai/models` Helm chart:

```yaml
# helm-values.yaml
catalog:
  my-custom-model-name:
    enabled: true
    features: ["TextEmbedding"]
    owner: me
    url: "hf://me/my-custom-model"
    resourceProfile: CPU:1
```

## Installing models with kubectl

You can add your own model by defining a Model yaml file and applying it using `kubectl apply -f model.yaml`.

Take a look at the KubeAI [API docs](../reference/kubernetes-api.md) to view Model schema documentation.

If you have a running cluster with KubeAI installed you can inspect the schema for a Model using `kubectl explain`:

```bash
kubectl explain models
kubectl explain models.spec
kubectl explain models.spec.engine
```

You can view all example manifests on the [GitHub repository](https://github.com/substratusai/kubeai/tree/main/manifests/models).

Below are few examples using various engines and resource profiles.

### Example Gemma 2 2B using Ollama on CPU

```yaml
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: gemma2-2b-cpu
spec:
  features: [TextGeneration]
  url: ollama://gemma2:2b
  engine: OLlama
  resourceProfile: cpu:2
```

### Example Llama 3.1 8B using vLLM on NVIDIA L4 GPU

```yaml
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: llama-3.1-8b-instruct-fp8-l4
spec:
  features: [TextGeneration]
  owner: neuralmagic
  url: hf://neuralmagic/Meta-Llama-3.1-8B-Instruct-FP8
  engine: VLLM
  args:
    - --max-model-len=16384
    - --max-num-batched-token=16384
    - --gpu-memory-utilization=0.9
    - --disable-log-requests
  resourceProfile: nvidia-gpu-l4:1
```

## Programmatically installing models

See the [examples](https://github.com/substratusai/kubeai/tree/main/examples/k8s-api-clients).

## Calling a model

You can inference a model by calling the KubeAI OpenAI compatible API. The model name should match the KubeAI model name.

## Using Pod Priority Classes for Model Preemption

You can use Kubernetes [Pod Priority and Preemption](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/) to configure your models with different priority levels. This is useful when you have limited resources and want to ensure that high-priority models can preempt lower-priority models when necessary.

First, create the necessary PriorityClasses in your cluster:

```yaml
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: high-priority
value: 1000000  # Higher value means higher priority
globalDefault: false
description: "This priority class should be used for critical inference models only."
---
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: medium-priority
value: 100000
globalDefault: false
description: "This priority class should be used for medium priority inference models."
---
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: low-priority
value: 10000
globalDefault: false
description: "This priority class should be used for low priority inference models."
```

Then, assign these priority classes to your models:

```yaml
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: critical-service-model
spec:
  features: [TextGeneration]
  url: hf://neuralmagic/Meta-Llama-3.1-8B-Instruct-FP8
  engine: VLLM
  resourceProfile: nvidia-gpu-l4:1
  priorityClassName: high-priority
---
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: background-research-model
spec:
  features: [TextGeneration]
  url: ollama://gemma2:2b
  engine: OLlama
  resourceProfile: cpu:2
  priorityClassName: low-priority
```

When the cluster is under resource pressure, Kubernetes will evict lower-priority model pods to make room for higher-priority model pods. This ensures that your most critical models remain available even during resource constraints.

## Feedback welcome: A model management UI

We are considering adding a UI for managing models in a running KubeAI instance. Give the [GitHub Issue](https://github.com/substratusai/kubeai/issues/148) a thumbs up if you would be interested in this feature.
