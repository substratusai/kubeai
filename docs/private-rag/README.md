# Private RAG with Weaviate and Lingo

This guide will show you how to use Lingo as the LLM and embedding model provider for a private RAG setup with Weaviate within the same K8s cluster. Now you can take full control over your data and models.

## GKE cluster creation (optional)
You can skip this step if you already have a K8s cluster with GPU nodes.

Create a GKE cluster with a CPU and 1 x L4 GPU nodepool:
```
bash <(curl -s https://raw.githubusercontent.com/substratusai/lingo/main/deploy/create-gke-cluster.sh)
```
Make sure you review the script before executing!

## Installation
Add required helm repos:
```
helm repo add weaviate https://weaviate.github.io/weaviate-helm
helm repo add substratusai https://substratusai.github.io/helm
helm repo update
```

Instal Mistral 7b instruct v2:
```
export HF_TOKEN=replaceMe!
envsubst < mistral-v02-values.yaml | helm upgrade --install mistral-7b-instruct-v02 substratusai/vllm -f -
```
Installing Mistral can take a few minutes. It will first try to scale up the GPU
nodepool and then take time to download and load the model.

Check the progress:
```
kubectl get pods -w
kubectl logs -l app.kubernetes.io/instance=mistral-7b-instruct-v02
```

Create a file called `weaviate-values.yaml` with the following content:
[embedmd]:# (weaviate-values.yaml)
```yaml
modules:
  text2vec-openai:
    enabled: true
    apiKey: 'thiswillbeignoredbylingo'

service:
  type: ClusterIP
```


Install Weaviate:
```
helm upgrade --install weaviate weaviate/weaviate -f weaviate-values.yaml
```

Install Lingo ML proxy and autoscaler:
```
helm upgrade --install lingo substratusai/lingo
```

Create a file called `stapi-values.yaml` with the following content:
[embedmd]:# (stapi-values.yaml)
```yaml
deploymentAnnotations:
  lingo.substratus.ai/models: text-embedding-ada-002
  lingo.substratus.ai/min-replicas: "1" # needs to be string
model: all-MiniLM-L6-v2
replicaCount: 0
```

Install embedding model server with OpenAI compatible endpoint:
```
helm upgrade --install stapi-minilm-l6-v2 substratusai/stapi -f stapi-values.yaml
```

Create a file called `verba-values.yaml` with the following content:
[embedmd]:# (verba-values.yaml)
```yaml
env:
- name: OPENAI_MODEL
  value: mistral-7b-instruct-v0.2
- name: OPENAI_API_KEY
  value: ignored-by-lingo
- name: OPENAI_BASE_URL
  value: http://lingo/v1
- name: WEAVIATE_URL_VERBA
  value: http://weaviate:80
```


Instal Weaviate Verba:
```
helm upgrade --install verba substratusai/verba -f verba-values.yaml
```

## Usage
Access verba through port-forward:
```
kubectl port-forward service/verba 8080:80
```

Now go to [localhost:8080](http://localhost:8080) in your browser. Try adding a document and then ask some question about your document.

Download a PDF document about Nasoni Smart Faucet [here](https://github.com/docugami/KG-RAG-datasets/blob/main/nih-clinical-trial-protocols/data/v1/docs/NCT06159946_Prot_000.pdf).

Upload the document inside Verba and ask questions like:
- How did they test the Nasoni Smart Facet?
- What's a Nasoni Smart Facet?