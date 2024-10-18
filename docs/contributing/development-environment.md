# Development environment

This document provides instructions for setting up an environment for developing KubeAI.

## Optional: Cloud Setup

### GCP PubSub

If you are develop PubSub messaging integration on GCP, setup test topics and subscriptions and uncomment the `.messaging.streams` in `./hack/dev-config.yaml`.

```bash
gcloud auth login --update-adc

gcloud pubsub topics create test-kubeai-requests
gcloud pubsub subscriptions create test-kubeai-requests-sub --topic test-kubeai-requests
gcloud pubsub topics create test-kubeai-responses
gcloud pubsub subscriptions create test-kubeai-responses-sub --topic test-kubeai-responses
```

## Run in Local Cluster

```bash
kind create cluster
# OR
#./hack/create-dev-gke-cluster.yaml

# Generate CRDs from Go code.
make generate && make manifests

# When CRDs are changed reapply using kubectl:
kubectl apply -f ./charts/kubeai/charts/crds/crds

# Model with special address annotations:
kubectl apply -f ./hack/dev-model.yaml

# OPTION A #
# Run KubeAI inside cluster
# Change `-f` based on the cluster environment.
helm upgrade --install kubeai ./charts/kubeai \
    --set openwebui.enabled=true \
    --set image.tag=latest \
    --set image.pullPolicy=Always \
    --set image.repository=us-central1-docker.pkg.dev/substratus-dev/default/kubeai \
    --set secrets.huggingface.token=$HUGGING_FACE_HUB_TOKEN \
    --set replicaCount=1 -f ./hack/dev-gke-helm-values.yaml

# OPTION B #
# For quick local interation (run KubeAI outside of cluster)
kubectl create cm kubeai-autoscaler-state -oyaml --dry-run=client | kubectl apply -f -
CONFIG_PATH=./hack/dev-config.yaml POD_NAMESPACE=default go run ./cmd/main.go

# In another terminal:
while true; do kubectl port-forward service/dev-model 7000:7000; done
############
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