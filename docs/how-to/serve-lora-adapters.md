# Serve LoRA adapters

In this guide you will configure KubeAI to serve LoRA adapters.

## Configuring adapters

LoRA adapters are configured on Model objects. For Example:

```yaml
# model.yaml
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: tinyllama-chat
spec:
  features: [TextGeneration]
  owner: meta-llama
  url: hf://TinyLlama/TinyLlama-1.1B-Chat-v0.3
  adapters: # <--
  - name: colorist
    url: hf://jashing/tinyllama-colorist-lora
  engine: VLLM
  resourceProfile: nvidia-gpu-l4:1
  minReplicas: 1
```

**Limitation:** Currently LoRA adapters are only supported with `engine: VLLM` and `hf://` or `s3://` urls.

You can install this Model using kubectl:

```bash
kubectl apply -f ./model.yaml
```

Or if you are managed models with the KubeAI [models helm chart](https://github.com/substratusai/kubeai/tree/main/charts/models) you can add adapters to a given model via your helm values:

```yaml
# helm-values.yaml
catalog:
  llama-3.1-8b-instruct-fp8-l4:
    enabled: true
    adapters:
    - name: example
      url: hf://some-huggingface-user/some-huggingface-repo
    # ...
```

## Requesting an adapter

When using the OpenAI compatible REST API, model adapters are referenced using the `<base-model>_<adapter>` convention. Once a Model is installed with an adapter, you can request that adapter by name via appending `_<adapter-name>` to the model field. This will work with any OpenAI client library.

If you installed a Model with `name: llama-3.2` and configured `.spec.adapters[]` to contain an adapter with `name: sql`, you could issue a completion request to that adapter using:

```bash
curl http://$KUBEAI_ENDPOINT/openai/v1/completions \
    -H "Content-Type: application/json" \
    -H "X-Label-Selector: tenancy in (org-abc, public)" \
    -d '{"prompt": "Hi", "model": "llama-3.2_sql"}'
```

## Listing adapters

Adapters will be returned by the `/models` endpoint:

```bash
curl http://$KUBEAI_ENDPOINT/openai/v1/models
```

Each adapter will be listed as a separate model object with the adapter name appended to the base Model name.