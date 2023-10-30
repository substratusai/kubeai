# Development

```sh
kind create cluster

# Install STAPI
helm repo add substratusai https://substratusai.github.io/helm
helm repo update
helm install stapi-minilm-l6-v2 substratusai/stapi \
  --set model=all-MiniLM-L6-v2 \
  --set deploymentAnnotations.lingo-models=text-embedding-ada-002

# Deploy
skaffold run

# In another terminal...
kubectl port-forward svc/proxy-controller 8080:80
# In another terminal...
watch kubectl get pods

# Get embeddings using OpenAI compatible API endpoint
curl http://localhost:8080/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "input": "Your text string goes here",
    "model": "text-embedding-ada-002"
  }'
```
