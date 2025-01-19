# Install on any Kubernetes Cluster

KubeAI can be installed on any Kubernetes cluster and doesn't require GPUs.
If you do have GPUs, then KubeAI can take advantage of them.

Please follow the Installation using GPUs section if you have GPUs available.


## Prerequisites

1. Add the KubeAI helm repository.

```bash
helm repo add kubeai https://www.kubeai.org
helm repo update
```

2. (Optional) Set the Hugging Face token as an environment variable. This is only required if you plan to use HuggingFace models that require authentication.

```bash
export HF_TOKEN=<your-hugging-face-token>
```

## Installation using only CPUs

All engines supported in KubeAI also support running only on CPU resources.

Install KubeAI using the pre-defined values file which defines CPU resourceProfiles:

```bash
helm install kubeai kubeai/kubeai --wait \
  --set secrets.huggingface.token=$HF_TOKEN
```

Optionally, inspect the values file to see the default resourceProfiles:

```bash
helm show values kubeai/kubeai > values.yaml
```

## Installation using NVIDIA GPUs

This section assumes you have a Kubernetes cluster with GPU resources available and
installed the NVIDIA device plugin that adds GPU information labels to the nodes.

This time we need to use a custom resource profiles that define the nodeSelectors
for different GPU types.

Download the values file for the NVIDIA GPU operator:

```bash
curl -L -O https://raw.githubusercontent.com/substratusai/kubeai/refs/heads/main/charts/kubeai/values-nvidia-k8s-device-plugin.yaml
```

You likely will not need to modify the `values-nvidia-k8s-device-plugin.yaml` file.
However, do inspect the file to ensure the GPU resourceProfile nodeSelectors match
the node labels on your nodes.


Install KubeAI using the custom resourceProfiles:
```bash
helm upgrade --install kubeai kubeai/kubeai \
    -f values-nvidia-k8s-device-plugin.yaml \
    --set secrets.huggingface.token=$HF_TOKEN \
    --wait
```

## Installation using AMD GPUs

```
helm upgrade --install kubeai ./charts/kubeai \
    -f charts/kubeai/values-amd-gpu-device-plugin.yaml \
    --set secrets.huggingface.token=$HF_TOKEN \
    --wait
```


## Deploying models

Take a look at the following how-to guides to deploy models:

* [Configure Text Generation Models](../how-to/configure-text-generation-models.md)
* [Configure Embedding Models](../how-to/configure-embedding-models.md)
* [Configure Speech to Text Models](../how-to/configure-speech-to-text.md)

