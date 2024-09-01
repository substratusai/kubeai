# How to manage models

This guide provides instructions on how to perform CRUD operations on KubeAI [Models](../concepts/models.md).

## Listing models

You can view all installed models through the Kubernetes API using `kubectl get models` (use the `-o yaml` flag for more details).

You can also list all models via the OpenAI-compatible `/v1/models` endpoint:

```bash
curl http://your-deployed-kubeai-endpoint/openai/v1/models
```

## Installing models with helm

### Preconfigured models with helm

When you are defining KubeAI Helm values, you can install a preconfigured Model by setting `enabled: true`. You can view a list of all preconfigured models [here](https://github.com/substratusai/kubeai/blob/main/charts/kubeai/charts/models/values.yaml). NOTE: When you are installing the KubeAI chart, the catalog is accessed under `.models.catalog.<model-name>`:

```yaml
# helm-values.yaml
models:
  catalog:
    llama-3.1-8b-instruct-fp8-l4:
      enabled: true
```

You can optionally override preconfigured settings, for example, `resourceProfile`:

```yaml
# helm-values.yaml
models:
  catalog:
    llama-3.1-8b-instruct-fp8-l4:
      enabled: true
      resourceProfile: nvidia-gpu-l4:2 # Require "2 NVIDIA L4 GPUs"
```

### Custom models with helm

If you prefer to add a custom model via the same Helm chart you use for installed KubeAI, you can add your custom model entry into the `.models.catalog` array of your existing Helm values file:

```yaml
# helm-values.yaml
models:
  catalog:
    my-custom-model-name:
      enabled: true
      features: ["TextEmbedding"]
      owner: me
      url: "hf://me/my-custom-model"
      resourceProfile: CPU:1
```

### Installing models with kubectl

You can add your own model by defining a Model yaml file and applying it using `kubectl apply -f model.yaml`.

If you have a running cluster with KubeAI installed you can inspect the schema for a Model using `kubectl explain`:

```bash
kubectl explain models
kubectl explain models.spec
kubectl explain models.spec.engine
```

## Feedback welcome: A model management UI

We are considering adding a UI for managing models in a running KubeAI instance. Give the [GitHub Issue](https://github.com/substratusai/kubeai/issues/148) a thumbs up if you would be interested in this feature.
