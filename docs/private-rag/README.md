# Fully Private RAG

Add required helm repos:
```
helm repo add weaviate https://weaviate.github.io/weaviate-helm
helm repo add substratusai https://substratusai.github.io/helm
helm repo update
```

Install Weaviate:
```
helm upgrade --install weaviate weaviate/weaviate -f weaviate-values.yaml
```

Install Lingo ML proxy and autoscaler:
```
helm upgrade --install lingo substratusai/lingo
```

Install embedding model server with OpenAI compatible endpoint:
```
helm upgrade --install stapi-minilm-l6-v2 substratusai/stapi -f stapi-values.yaml
```

Instal Mistral 7b instruct v2:
```
export HF_TOKEN=replaceMe!
envsubst < mistral-v02-values.yaml | helm upgrade --install mistral-7b-instruct-v02 substratusai/vllm -f -
```

Instal Weaviate Verba:
```
helm upgrade --install verba substratusai/verba -f verba-values.yaml
```

Access verba through port-forward:
```
kubectl port-forward service/verba 8080:80
```

Now go to localhost:8080 in your browser. Try adding a document and then ask some question about your document.
