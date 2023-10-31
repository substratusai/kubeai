# Lingo - K8s LLM Proxy + Scaler

Lingo is an OpenAI compatible LLM proxy and autoscaler for K8s

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

## Quickstart
Add the Helm repo:
```bash
helm repo add substratusai https://substratusai.github.io/helm
help repo update
```

Install the Lingo controller and proxy:
```bash
helm install lingo substratusai/lingo
```

Deploy an embedding model:
```bash
helm install stapi-minilm-l6-v2 substratusai/stapi \
  --set model=all-MiniLM-L6-v2 \
  --set deploymentAnnotations.lingo-models=text-embedding-ada-002 \
  --set replicas=0
```

Deploy a LLM (mistral-7b-instruct) using vLLM:
```bash
helm install mistral-7b-instruct substratusai/vllm \
  --set model=mistralai/Mistral-7B-Instruct-v0.1 \
  --set deploymentAnnotations.lingo-models=mistral-7b-instruct-v0.1 \
  --set replicas=0
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
  -d '{"model": "mistralai/Mistral-7B-Inst", "prompt": "<s>[INST]Who was the first president of the United States?[/INST]", "max_tokens": 40}'
```
The first request to an LLM takes longer because
those models require a GPU and require additional time
to download the model.

What else would you like to see? Join our Discord and ask directly.

## Roadmap

* HA for the proxy controller
* Response Request Caching
* Model caching to speed up auto scaling for LLMs
* Authentication
* Multi cluster serving

## Creators
Feel free to contact any of us:
* [Nick Stogner](https://www.linkedin.com/in/nstogner/)
* [Sam Stoelinga](https://www.linkedin.com/in/samstoelinga/)
