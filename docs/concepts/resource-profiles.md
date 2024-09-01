# Resource Profiles

A resource profile maps a type of compute resource (i.e. NVIDIA L4 GPU) to a collection of Kubernetes settings that are configured on inference server Pods. These profiles are defined in the KubeAI `config.yaml` file (via a ConfigMap). Each model specifies the resource profile that it requires.

Kubernetes Model resources specify a resource profile and the count of that resource that they require:

```yaml
# model.yaml
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: llama-3.1-8b-instruct-fp8-l4
spec:
  engine: VLLM
  resourceProfile: nvidia-gpu-l4:1 # Specified as <profile>:<count>
  # ...
```

A given profile might need to contain slightly different settings based on the cluster/cloud that KubeAI is deployed in.

Example: A resource profile named `nvidia-gpu-l4` might contain the following settings on a GKE Kubernetes cluster:

```yaml
# KubeAI config.yaml
resourceProfiles:
  nvidia-gpu-l4:
    limits:
      # Typical across most Kubernetes clusters:
      nvidia.com/gpu: "1"
    requests:
      nvidia.com/gpu: "1"
    nodeSelector:
      # Specific to GKE:
      cloud.google.com/gke-accelerator: "nvidia-l4"
      cloud.google.com/gke-spot: "true"
    imageName: "nvidia-gpu"
```

In addition to node selectors and resource requirements, a resource profile may optionally specify an image name. This name maps to the container image that will be selected when serving a model on that resource:

```yaml
# KubeAI config.yaml
modelServers:
  VLLM:
    images:
      default: "vllm/vllm-openai:v0.5.5"
      nvidia-gpu: "vllm/vllm-openai:v0.5.5" # <--
      cpu: "vllm/vllm-openai-cpu:v0.5.5"
  OLlama:
    images:
      # ...
```

## Next

Read about [how to manage resource profiles](../guides/how-to-manage-resource-profiles.md).