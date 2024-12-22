# KubeAI: AI Inferencing Operator

Deploy and scale machine learning models in production. Built for LLMs, embeddings, and speech-to-text.

## Key Features

🚀 **LLM Operator** - Manages vLLM and Ollama servers  
🔗 **OpenAI Compatible** - WoWorks withpenAI clclient librarie 
🛠️ **Simple Deployment** - No external depencies required  
⚡️ **Intelligent Scaling** - Scale from zero to meet demand  
⛕ **Smart Routing** - Load balancing algo purpose built for LLMs  
🧩 **Dynamic LoRA** - Hot-swap model adapters with zero downtime  
🖥 **Hardware Flexible** - Runs on CPU, GPU, or TPU  
💾 **Efficient Caching** - Supports EFS, Filestore, and more  
🎙️ **Speech Processing** - Transcribe audio via FasterWhisper  
🔢 **Vector Operations** - Generate embeddings via Infinity  
📨 **Event Streaming** - Native integrations with Kafka and more  

Quotes from the community:

> reusable, well abstracted solution to run LLMs - [Mike Ensor](https://www.linkedin.com/posts/mikeensor_gcp-solutions-public-retail-edge-available-cluster-traits-activity-7237515920259104769-vBs9?utm_source=share&utm_medium=member_desktop)

## Architecture

KubeAI exposes an OpenAI-compatible API and manages ML models through Kubernetes Custom Resources. It is architected as a Model Operator ([learn more](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)) that orchestrates [vLLM](https://github.com/vllm-project/vllm) and [Ollama](https://github.com/ollama/ollama) servers.

<img src="./diagrams/arch.excalidraw.png"></img>

## Adopters

List of known adopters:

| Name | Description | Link |
| ---- | ----------- | ---- |
| Telescope | Telescope uses KubeAI for multi-region large scale batch LLM inference. | [trytelescope.ai](https://trytelescope.ai) |
| Google Cloud Distributed Edge | KubeAI is included as a reference architecture for inferencing at the edge. | [LinkedIn](https://www.linkedin.com/posts/mikeensor_gcp-solutions-public-retail-edge-available-cluster-traits-activity-7237515920259104769-vBs9?utm_source=share&utm_medium=member_desktop), [GitLab](https://gitlab.com/gcp-solutions-public/retail-edge/available-cluster-traits/kubeai-cluster-trait) |
| Lambda | You can try KubeAI on the Lambda AI Developer Cloud. See Lambda's [tutorial](https://docs.lambdalabs.com/education/large-language-models/kubeai-hermes-3/) and [video](https://youtu.be/HEtPO2Wuiac). | [Lambda](https://lambdalabs.com/) |
| Vultr | KubeAI can be deployed on Vultr Managed Kubernetes using the application marketplace. | [Vultr](https://www.vultr.com) |
| Arcee | Arcee uses KubeAI for multi-region, multi-tenant SLM inference. | [Arcee](https://www.arcee.ai/) |

If you are using KubeAI and would like to be listed as an adopter, please make a PR.

## Local Quickstart

Create a local cluster using [kind](https://kind.sigs.k8s.io/) or [minikube](https://minikube.sigs.k8s.io/docs/).

<details>
<summary>TIP: If you are using Podman for kind...</summary>
Make sure your Podman machine can use up to 6G of memory (by default it is capped at 2G):

```bash
# You might need to stop and remove the existing machine:
podman machine stop
podman machine rm

# Init and start a new machine:
podman machine init --memory 6144 --disk-size 120
podman machine start
```
</details>


```bash
kind create cluster # OR: minikube start
```

Add the KubeAI [Helm](https://helm.sh/docs/intro/install/) repository.

```bash
helm repo add kubeai https://www.kubeai.org
helm repo update
```

Install KubeAI and wait for all components to be ready (may take a minute).

```bash
helm install kubeai kubeai/kubeai --wait --timeout 10m
```

Install some predefined models.

```bash
cat <<EOF > kubeai-models.yaml
catalog:
  gemma2-2b-cpu:
    enabled: true
    minReplicas: 1
  qwen2-500m-cpu:
    enabled: true
  nomic-embed-text-cpu:
    enabled: true
EOF

helm install kubeai-models kubeai/models \
    -f ./kubeai-models.yaml
```

Before progressing to the next steps, start a watch on Pods in a standalone terminal to see how KubeAI deploys models. 

```bash
kubectl get pods --watch
```

#### Interact with Gemma2

Because we set `minReplicas: 1` for the Gemma model you should see a model Pod already coming up.

Start a local port-forward to the bundled chat UI.

```bash
kubectl port-forward svc/openwebui 8000:80
```

Now open your browser to [localhost:8000](http://localhost:8000) and select the Gemma model to start chatting with.

#### Scale up Qwen2 from Zero

If you go back to the browser and start a chat with Qwen2, you will notice that it will take a while to respond at first. This is because we set `minReplicas: 0` for this model and KubeAI needs to spin up a new Pod (you can verify with `kubectl get models -oyaml qwen2-500m-cpu`).

## Get Started

Learn more at [kubeai.org](https://www.kubeai.org)!

Join the [Discord channel](https://discord.gg/JeXhcmjZVm) to chat.

Or just reach out on LinkedIn:

* [Nick Stogner](https://www.linkedin.com/in/nstogner/)
* [Sam Stoelinga](https://www.linkedin.com/in/samstoelinga/)
