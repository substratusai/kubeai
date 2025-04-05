# Configure Text Generation Models

KubeAI supports the following engines for text generation models (LLMs, VLMs, ..):

- vLLM (Recommended for GPU)
- Ollama (Recommended for CPU)
- Need something else? Please file an issue on [GitHub](https://github.com/substratusai/kubeai).

There are 2 ways to install a text generation model in KubeAI:
- Use Helm with the `kubeai/models` chart.
- Use `kubectl apply -f model.yaml` to install a Model Custom Resource.

KubeAI comes with pre-validated and optimized Model configurations for popular text generation models. These models are available in the
[kubeai/models Helm chart](https://github.com/substratusai/kubeai/tree/main/charts/models)
and are also published as raw manifests in the
[manifests/model directory](https://github.com/substratusai/kubeai/tree/main/manifests/models).

You can also easily define your own models using the Model Custom Resource directly or by using the `kubeai/models` Helm chart.

## Install a Text Generation Model using Helm

You can take a look at all the pre-configured models in the chart's [default values file](https://github.com/substratusai/kubeai/blob/main/charts/models/values.yaml).

You can get the default values for the models chart using the following command:
```bash
helm show values kubeai/models
```

### Install Text Generation Model using L4 GPU

Enable the Llama 3.1 8B model using the Helm chart:

```bash
helm upgrade --install --reuse-values kubeai-models kubeai/models -f - <<EOF
catalog:
  llama-3.1-8b-instruct-fp8-l4:
    enabled: true
    engine: VLLM
    resourceProfile: nvidia-gpu-l4:1
    minReplicas: 1 # by default this is 0
EOF
```

## Install a Text Generation Model using kubectl
You can use the Model Custom Resource directly to install a model using `kubectl apply -f model.yaml`.

### Install Text Generation Model using L4 GPU

Apply the following Model Custom Resource to install the Llama 3.1 8B model using vLLM on L4 GPU:
```yaml
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: llama-3.1-8b-instruct-fp8-l4
spec:
  features: [TextGeneration]
  url: hf://neuralmagic/Meta-Llama-3.1-8B-Instruct-FP8
  engine: VLLM
  args:
    - --max-model-len=16384
    - --max-num-batched-token=16384
    - --gpu-memory-utilization=0.9
    - --disable-log-requests
  resourceProfile: nvidia-gpu-l4:1
```

## Configure a Chat Template
Some models do not ship will chat templates and some engines such as vLLM do not provide a default one. In these cases, you can use `.spec.files` to inject a template at Pod runtime.

```yaml
kind: Model
spec:
  # ...
  engine: VLLM
  args:
  - --chat-template=/config/chat-template.jinja
  files:
  - path: /config/chat-template.jinja
    content: |
      {% for message in messages %}{{'<|im_start|>' + message['role'] + '\n' + message['content']}}{% if (loop.last and add_generation_prompt) or not loop.last %}{{ '<|im_end|>' + '\n'}}{% endif %}{% endfor %}
      {% if add_generation_prompt and messages[-1]['role'] != 'assistant' %}{{ '<|im_start|>assistant\n' }}{% endif %}
```

## Interact with the Text Generation Model
The KubeAI service exposes an OpenAI compatible API that you can use to query the available models and interact with them.

The KubeAI service is available at `http://kubeai/openai/v1` within the Kubernetes cluster.

You can also port-forward the KubeAI service to your local machine to interact with the models:

```bash
kubectl port-forward svc/kubeai 8000:80
```

You can now query the available models using curl:

```bash
curl http://localhost:8000/openai/v1/models
```

### Using curl to interact with the model

Run the following curl command to interact with the model named `llama-3.1-8b-instruct-fp8-l4`:
```bash
curl "http://localhost:8000/openai/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{
        "model": "llama-3.1-8b-instruct-fp8-l4",
        "messages": [
            {
                "role": "system",
                "content": "You are a helpful assistant."
            },
            {
                "role": "user",
                "content": "Write a haiku about recursion in programming."
            }
        ]
    }'
```

### Using the OpenAI Python SDK to interact with the model
Once the pod is ready, you can use the OpenAI Python SDK to interact with the model:
All OpenAI SDKs work with KubeAI since the KubeAI service is OpenAI API compatible.

See the below example code to interact with the model using the OpenAI Python SDK:
```python
import os
from openai import OpenAI
# Assumes port-forward of kubeai service to localhost:8000.
kubeai_endpoint = "http://localhost:8000/openai/v1"
model_name = "llama-3.1-8b-instruct-fp8-l4"

# If you are running in a Kubernetes cluster, you can use the kubeai service endpoint.
if os.getenv("KUBERNETES_SERVICE_HOST"):
    kubeai_endpoint = "http://kubeai/openai/v1"

client = OpenAI(api_key="ignored", base_url=kubeai_endpoint)

chat_completion = client.chat.completions.create(
    messages=[
        {
            "role": "user",
            "content": "Say this is a test",
        }
    ],
    model=model_name,
)
```

## Ollama Insecure Model Pulling

You can set the environment variable `INSECURE=true` to allow pulling from insecure repositories.

**Warning:** Use this only in trusted network environments.

Example Model spec:
```yaml
spec:
  env:
    INSECURE: "true"
```