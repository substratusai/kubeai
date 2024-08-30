# Model Management

KubeAI uses Model [Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) to configure what ML models are available in the system.

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
  minReplicas: 0
  maxReplicas: 3
  resourceProfile: L4:1
```

### Listing Models

You can view all installed models through the Kubernetes API using `kubectl get models` (use the `-o yaml` flag for more details).

You can also list all models via the OpenAI-compatible `/v1/models` endpoint:

```bash
curl http://your-deployed-kubeai-endpoint/openai/v1/models
```

### Installing a predefined Model using Helm

When you are defining your Helm values, you can install a predefined Model by setting `enabled: true`:

```yaml
models:
  catalog:
    llama-3.1-8b-instruct-fp8-l4:
      enabled: true
```

You can also optionally override settings for a given model:

```yaml
models:
  catalog:
    llama-3.1-8b-instruct-fp8-l4:
      enabled: true
      env:
        MY_CUSTOM_ENV_VAR: "some-value"
```

### Adding Custom Models

You can add your own model by defining a Model yaml file and applying it using `kubectl apply -f model.yaml`.

If you have a running cluster with KubeAI installed you can inspect the schema for a Model using `kubectl explain`:

```bash
kubectl explain models
kubectl explain models.spec
kubectl explain models.spec.engine
```

### Model Management UI

We are considering adding a UI for managing models in a running KubeAI instance. Give the [GitHub Issue](https://github.com/substratusai/kubeai/issues/148) a thumbs up if you would be interested in this feature.