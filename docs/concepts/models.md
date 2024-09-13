# Models

KubeAI serves ML models by launching Pods on Kubernetes. Every model server Pod loads exactly one model on startup.

Model [Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) are used to configure what ML models KubeAI serves.

Example:

```yaml
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: llama-3.1-8b-instruct-fp8-l4
spec:
  features: ["TextGeneration"]
  owner: neuralmagic
  url: hf://neuralmagic/Meta-Llama-3.1-8B-Instruct-FP8
  engine: VLLM
  args:
    - --max-model-len=16384
    - --max-num-batched-token=16384
    - --gpu-memory-utilization=0.9
  resourceProfile: nvidia-gpu-l4:1
```

## Server Settings

In a Model manifest you can define what server to use for inference (`VLLM`, `OLlama`). Any model-specific settings can be passed to the server process via the `args` and `env` fields.

## Autoscaling

KubeAI has built-in support for autoscaling model inference servers. If the model is scaled to zero when a request comes in, the KubeAI server will hold the request until it is able spin up a new server Pod. Once the model server is running, KubeAI server will automatically scrape its metrics endpoint to determine how to autoscale from there.

Autoscaling can be configured via the `minReplicas` and `maxReplicas` spec fields. KubeAI will modify the `.spec.replicas` field as it autoscales and report the observed state in `.status.replicas`.

## OpenAI-API compatibility

Kubernetes Model Custom Resources are the source of truth for models that exist in KubeAI. KubeAI provides a view into installed Models via the OpenAI `/v1/models` endpoint (which KubeAI serves at `/openai/v1/models`).

## Next

Read about [how to manage models](../how-to/manage-models.md).