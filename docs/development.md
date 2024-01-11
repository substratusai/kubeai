# Development

## Testing

```sh
make test-unit
make test-integration
make test-race
```

## Local Deployment

Create a local cluster.

```sh
kind create cluster
```

Install a scaled-to-zero embeddings backend.

```sh
# Install STAPI
helm repo add substratusai https://substratusai.github.io/helm
helm repo update
helm upgrade --install stapi-minilm-l6-v2 substratusai/stapi -f - << EOF
model: all-mpnet-base-v2
replicaCount: 0
deploymentAnnotations:
  lingo.substratus.ai/models: text-embedding-ada-002
EOF
```

Deploy Lingo from source.

```sh
skaffold dev
```

OR

Deploy Lingo from the main branch.

```bash
helm upgrade --install lingo substratusai/lingo \
  --set image.tag=main \
  --set image.pullPolicy=Always
```

Send test requests.

```sh
# In another terminal...
kubectl port-forward svc/lingo 8080:80
# In another terminal...
watch kubectl get pods

# try httpbin with a delay
curl http://localhost:8080/delay/10 \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Your text string goes here",
    "model": "backend"
  }'


# Get embeddings using OpenAI compatible API endpoint.
curl http://localhost:8080/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "input": "Your text string goes here",
    "model": "text-embedding-ada-002"
  }'
```
