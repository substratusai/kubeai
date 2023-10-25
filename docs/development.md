# Development

```sh
kind create cluster

# Deploy
skaffold run

# In another terminal...
kubectl port-forward svc/proxy-controller 8080:80
# In another terminal...
watch kubectl get pods

# Experiment with different delays...
curl localhost:8080/delay/1 -H 'X-Model: backend'
```
