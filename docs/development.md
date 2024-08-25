# Development

## Cloud Setup

```bash
gcloud pubsub topics create test-kubeai-requests
gcloud pubsub subscriptions create test-kubeai-requests-sub --topic test-kubeai-requests
gcloud pubsub topics create test-kubeai-responses
gcloud pubsub subscriptions create test-kubeai-responses-sub --topic test-kubeai-responses
```

## Local Cluster

```bash
kind create cluster
# OR
#./hack/create-dev-gke-cluster.yaml

# When CRDs are changed reapply using kubectl:
kubectl apply -f ./charts/kubeai/charts/crds/crds

# Model with special address annotations:
kubectl apply -f ./hack/dev-model.yaml

# For developing in-cluster features:
helm upgrade --install kubeai ./charts/kubeai \
    --set openwebui.enabled=true \
    --set image.tag=latest \
    --set image.pullPolicy=Always \
    --set image.repository=us-central1-docker.pkg.dev/substratus-dev/default/kubeai \
    --set replicaCount=1 # 0 if running out-of-cluster (using "go run")

# -f ./helm-values.yaml \

# Run in development mode.
CONFIG_PATH=./hack/dev-config.yaml POD_NAMESPACE=default go run ./cmd/main.go --allow-pod-address-override

# In another terminal:
while true; do kubectl port-forward service/dev-model 7000:7000; done
```

## Running

### Completions API

```bash
# If you are running kubeai in-cluster:
# kubectl port-forward svc/kubeai 8000:80

curl http://localhost:8000/openai/v1/completions -H "Content-Type: application/json" -d '{"prompt": "Hi", "model": "dev"}' -v
```

### Messaging Integration

```bash
gcloud pubsub topics publish test-kubeai-requests \                  
  --message='{"path":"/v1/completions", "metadata":{"a":"b"}, "body": {"model": "dev", "prompt": "hi"}}'

gcloud pubsub subscriptions pull test-kubeai-responses-sub --auto-ack
```