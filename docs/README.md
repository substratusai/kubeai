# KubeAI: AI Inferencing Operator

Serve ML models in production on Kubernetes. Supports LLMs, embeddings, and speech-to-text.


⛕  Better performance with Prefix-aware Load Balancing [(see benchmark)](./benchmarks/prefix-aware-load-balancing.md)  
✅️  OpenAI API Compatibility: Drop-in replacement for OpenAI  
⚖️  Autoscaling: Scale from zero, autoscale based on load  
🧠  Serve text generation models with vLLM or Ollama  
🔌  Dynamic LoRA adapter loading  
💬  Speech to Text API with FasterWhisper  
🧮  Embedding/Vector API with Infinity  
🚀  Multi-platform: CPU, GPU, TPU  
💾  Model caching with shared filesystems (EFS, Filestore, etc.)  
🛠️  Zero dependencies (does not depend on Istio, Knative, etc.)  
💬  Chat UI included ([OpenWebUI](https://github.com/open-webui/open-webui))  
✉  Stream/batch inference via messaging integrations (Kafka, PubSub, etc.)  

Quotes from the community:

> reusable, well abstracted solution to run LLMs - [Mike Ensor](https://www.linkedin.com/posts/mikeensor_gcp-solutions-public-retail-edge-available-cluster-traits-activity-7237515920259104769-vBs9?utm_source=share&utm_medium=member_desktop)

## Why KubeAI?

### Better performance at scale
When running multiple replicas of a serving engine such as vLLM, performance under production traffic is heavily influence by the load balancing strategy.

KubeAI supports Least Load and Prefix Hash load balancing. Prefix Hash
load balancing with the KubeAI proxy provides a significant performance boost.

<img src="./benchmarks/prefix-aware-load-balancing-mean-ttft.png" width="80%"/>

See the [full benchmark](./benchmarks/prefix-aware-load-balancing.md) for more details.

### Simplicity and ease of use
KubeAI does not have other dependencies which makes it possible to deploy
and manage in any environment. You can deploy models
using pre-validated models for specific GPU types. This saves you time
because you don't have to tweak engine arguments for hours to get
a model up and running.


## Architecture

KubeAI serves an OpenAI compatible HTTP API. Admins can configure ML models via `kind: Model` Kubernetes Custom Resources. KubeAI can be thought of as a Model Operator (See [Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)) that manages [vLLM](https://github.com/vllm-project/vllm) and [Ollama](https://github.com/ollama/ollama) servers.

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
| Seeweb | Seeweb leverages KubeAI for direct and client-facing GPU inference workloads. KubeAI can be deployed on any GPU server and SKS | [Seeweb](https://www.seeweb.it/en) |

If you are using KubeAI and would like to be listed as an adopter, please make a PR.

## Local Quickstart


<video controls src="https://github.com/user-attachments/assets/711d1279-6af9-4c6c-a052-e59e7730b757" width="800"></video>

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
  deepseek-r1-1.5b-cpu:
    enabled: true
    features: [TextGeneration]
    url: 'ollama://deepseek-r1:1.5b'
    engine: OLlama
    minReplicas: 1
    resourceProfile: 'cpu:1'
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

#### Interact with Deepseek R1 1.5b

Because we set `minReplicas: 1` for the Deepseek model you should see a model Pod already coming up.

Start a local port-forward to the bundled chat UI.

```bash
kubectl port-forward svc/open-webui 8000:80
```

Now open your browser to [localhost:8000](http://localhost:8000) and select the Deepseek model to start chatting with.

#### Scale up Qwen2 from Zero

If you go back to the browser and start a chat with Qwen2, you will notice that it will take a while to respond at first. This is because we set `minReplicas: 0` for this model and KubeAI needs to spin up a new Pod (you can verify with `kubectl get models -oyaml qwen2-500m-cpu`).

## Documentation

Checkout our documentation on [kubeai.org](https://www.kubeai.org) to find info on:

* Installing KubeAI in the cloud
* How to guides (e.g. how to manage models and resource profiles).
* Concepts (how the components of KubeAI work).
* How to contribute

## OpenAI API Compatibility

```bash
# Implemented #
/v1/chat/completions
/v1/completions
/v1/embeddings
/v1/models
/v1/audio/transcriptions

# Planned #
# /v1/assistants/*
# /v1/batches/*
# /v1/fine_tuning/*
# /v1/images/*
# /v1/vector_stores/*
```

## Immediate Roadmap

* Model caching
* LoRA finetuning (compatible with OpenAI finetuning API)
* Image generation (compatible with OpenAI images API)

*NOTE:* KubeAI was born out of a project called Lingo which was a simple Kubernetes LLM proxy with basic autoscaling. We relaunched the project as KubeAI (late August 2024) and expanded the roadmap to what it is today.

🌟 Don't forget to drop us a star on GitHub and follow the repo to stay up to date!

[![KubeAI Star history Chart](https://api.star-history.com/svg?repos=substratusai/kubeai&type=Date)](https://star-history.com/#substratusai/kubeai&Date)

## Contact

Let us know about features you are interested in seeing or reach out with questions. [Visit our Discord channel](https://discord.gg/JeXhcmjZVm) to join the discussion!

Or just reach out on LinkedIn if you want to connect:

* [Nick Stogner](https://www.linkedin.com/in/nstogner/)
* [Sam Stoelinga](https://www.linkedin.com/in/samstoelinga/)
