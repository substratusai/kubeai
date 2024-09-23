# Configure Embedding Models

KubeAI supports the following engines for text embedding models:

- Infinity
- vLLM
- Ollama

Infinity supports any HuggingFace models listed as text-embedding [models, reranking or clip models on huggingface](https://huggingface.co/models?other=text-embeddings-inference&sort=trending).


## Install the BAAI/bge-small-en-v1.5 model using Infinity

Create a file named `kubeai-models.yaml` with the following content:

```yaml
catalog:
  bge-embed-text-cpu:
    enabled: true
    features: ["TextEmbedding"]
    owner: baai
    url: "hf://BAAI/bge-small-en-v1.5"
    engine: Infinity
    resourceProfile: cpu:1
    minReplicas: 1
```

Apply the kubeai-models helm chart:

```bash
helm install kubeai-models kubeai/models -f ./kubeai-models.yaml
```
