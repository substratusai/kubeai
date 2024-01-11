# Lingo - K8s LLM Proxy + Scaler

Lingo is an OpenAI compatible LLM proxy and autoscaler for K8s

![lingo demo](lingo.gif)

🚀  Serve popular OSS LLM models in minutes on CPUs or GPUs  
🧮  Serve Embedding Model servers  
⚖️  Automatically scale up and down, all the way to 0  
🪄  Built-in proxy that batches requests while scaling magic happens  
🛠️  Easy to install, No complex dependencies such as Istio or Knative  
☁️  Provide a unified API across clouds for serving LLMs

Support the project by adding a star! ❤️

Join us on Discord:  
<a href="https://discord.gg/JeXhcmjZVm">
<img alt="discord-invite" src="https://dcbadge.vercel.app/api/server/JeXhcmjZVm?style=flat">
</a>

## Quickstart (Any K8s, Kind, GKE, EKS etc)
Add the Helm repo:
```bash
helm repo add substratusai https://substratusai.github.io/helm
helm repo update
```

Install the Lingo controller and proxy:
```bash
helm install lingo substratusai/lingo
```

Deploy an embedding model:
```bash
helm upgrade --install stapi-minilm-l6-v2 substratusai/stapi -f - << EOF
model: all-MiniLM-L6-v2
replicaCount: 0
deploymentAnnotations:
  lingo.substratus.ai/models: text-embedding-ada-002
EOF
```

Deploy a LLM (mistral-7b-instruct) using vLLM:
```bash
helm upgrade --install mistral-7b-instruct substratusai/vllm -f - << EOF
model: mistralai/Mistral-7B-Instruct-v0.1
replicaCount: 0
env:
- name: SERVED_MODEL_NAME
  value: mistral-7b-instruct-v0.1 # needs to be same as lingo model name
deploymentAnnotations:
  lingo.substratus.ai/models: mistral-7b-instruct-v0.1
  lingo.substratus.ai/min-replicas: "0" # needs to be string
  lingo.substratus.ai/max-replicas: "3" # needs to be string
EOF
```
Notice how the deployment has 0 replicas. That's fine because Lingo
will automatically scale the embedding model server from 0 to 1
once there is an incoming HTTP request.

By default, the proxy is only accessible within the Kubernetes cluster. To access it from your local machine, set up a port forward:
```bash
kubectl port-forward svc/lingo 8080:80
```

In a separate terminal watch the pods:
```bash
watch kubectl get pods
```

Get embeddings by using the OpenAI compatible HTTP API:
```bash
curl http://localhost:8080/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "input": "Lingo rocks!",
    "model": "text-embedding-ada-002"
  }'
```
You should see a stapi pod being created on the fly that
will serve the request. The beautiful thing about Lingo
is that it holds  your request in the proxy while the
stapi pod is being created, once it's ready to serve, Lingo
send the request to the stapi pod. The end-user does not
see any errors and gets the response to their request.

Similarly, send a request to the mistral-7b-instruct model that
was deployed:
```bash
curl http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "mistral-7b-instruct-v0.1", "prompt": "<s>[INST]Who was the first president of the United States?[/INST]", "max_tokens": 40}'
```
The first request to an LLM takes longer because
those models require a GPU and require additional time
to download the model.

What else would you like to see? Join our Discord and ask directly.

## Creators

Feel free to contact any of us:

* [Nick Stogner](https://www.linkedin.com/in/nstogner/)
* [Sam Stoelinga](https://www.linkedin.com/in/samstoelinga/)
