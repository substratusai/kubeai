# Development

```sh
kind create cluster

# Install STAPI
helm repo add substratusai https://substratusai.github.io/helm
helm repo update
helm upgrade --install stapi-minilm-l6-v2 substratusai/stapi -f - << EOF
model: all-mpnet-base-v2
replicaCount: 0
deploymentAnnotations:
  lingo.substratus.ai/models: text-embedding-ada-002
EOF


# Deploy
skaffold dev

# In another terminal...
kubectl port-forward svc/proxy-controller 8080:80
# In another terminal...
watch kubectl get pods

# try httpbin with a delay
curl http://localhost:8080/delay/10 \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Your text string goes here",
    "model": "backend"
  }'


# Get embeddings using OpenAI compatible API endpoint
curl http://localhost:8080/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "input": "Your text string goes here",
    "model": "text-embedding-ada-002"
  }'

# Install vLLM with facebook opt 125


```
