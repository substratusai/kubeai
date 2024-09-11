# Build embedded model container images

This guide assumes you are in the `kubeai.git` repo. In this guide we will preload a LLM into a custom built Ollama serving image. You can follow the same steps for other models and other serving engines.

Define some values
```bash
export MODEL_URL=ollama://qwen2:0.5b

# Customize with your own image repo.
export IMAGE=us-central1-docker.pkg.dev/substratus-dev/default/ollama-embedded-qwen2-05b:latest
```

Build and push image. Note: building (downloading base image & model) and pushing (uploading image & model) can take a while depending on the size of the model.

```bash
# Within kubeai.git...
cd ./images/ollama-embedded

docker build --build-arg MODEL_URL=$MODEL_URL -t $IMAGE .
docker push $IMAGE
```

Create a model manifest & apply into a cluster with KubeAI installed. NOTE: The only difference between an embedded model image and otherwise is the addition of the `image:` field.

```bash
cat <<EOF | envsubst > model.yaml
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: embedded-model-example
spec:
  features: ["TextGeneration"]
  owner: alibaba
  image: $IMAGE # <-- The embedded model image here
  url: "$MODEL_URL"
  engine: OLlama
  resourceProfile: cpu:1
  minReplicas: 1
  maxReplicas: 3
EOF

kubectl apply -f ./model.yaml
```
