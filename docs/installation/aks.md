# Install on AKS

<details markdown="1">
<summary><b>TIP</b>: Make sure you have the right quota and permissions.</summary>

- Confirm you have enough quota for GPU-enabled node SKUs (like `Standard_NC48ads_A100_v4`).
- Verify that you have the right permissions and that the account you use with Azure CLI can create AKS clusters, node pools, etc.
</details>

## Installing Prerequisites

Before running this setup, ensure you have:

- [Azure CLI](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli)
- [Kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
- [Helm](https://helm.sh/docs/intro/install/)

## 1. Define environment variables

```bash
AZURE_RESOURCE_GROUP="${AZURE_RESOURCE_GROUP:-kubeai-stack}"
AZURE_REGION="${AZURE_REGION:-southcentralus}"
CLUSTER_NAME="${CLUSTER_NAME:-kubeai-stack}"
USER_NAME="${USER_NAME:-azureuser}"
GPU_NODE_POOL_NAME="${GPU_NODE_POOL_NAME:-gpunodes}"
GPU_NODE_COUNT="${GPU_NODE_COUNT:-1}"
GPU_VM_SIZE="${GPU_VM_SIZE:-Standard_NC48ads_A100_v4}"
```

- `AZURE_RESOURCE_GROUP`: The name of the Azure resource group. The default value is `kubeai-stack`.
- `AZURE_REGION`: The Azure location or region where the AKS cluster will be deployed. The default value is `southcentralus`.
- `CLUSTER_NAME`: The name of the AKS cluster. The default value is `kubeai-stack`.
- `USER_NAME`: The username for the AKS cluster. The default value is `azureuser`.
- `GPU_NODE_POOL_NAME`: The name of the GPU node pool. The default value is `gpunodes`.
- `GPU_NODE_COUNT`: The number of GPU nodes in the GPU node pool. The default value is `1`.
- `GPU_VM_SIZE`: The SKU of the GPU VMs in the GPU node pool. The default value is `Standard_NC48ads_A100_v4`. You can find more GPU VM sizes [here](https://learn.microsoft.com/en-us/azure/virtual-machines/sizes/overview#gpu-accelerated).

## 2. Create a Resource Group

```bash
az group create \
  --name "${AZURE_RESOURCE_GROUP}" \
  --location "${AZURE_REGION}"
```

## 3. Create an AKS Cluster (CPU Only)

If you only intend to run CPU-based models, you can create a simple AKS cluster as follows:

```bash
az aks create \
    --resource-group "${AZURE_RESOURCE_GROUP}" \
    --name "${CLUSTER_NAME}" \
    --enable-oidc-issuer \
    --enable-workload-identity \
    --enable-managed-identity \
    --node-count 1 \
    --location "${AZURE_REGION}" \
    --admin-username "${USER_NAME}" \
    --generate-ssh-keys \
    --os-sku Ubuntu
```

## 4. (Optional) Add a GPU Node Pool

If you want to run GPU-backed models, you need a GPU node pool. Below is an example using the `Standard_NC48ads_A100_v4` SKU. Feel free to adjust `--node-vm-size` to match a GPU SKU you have quota for:

```bash
az aks nodepool add \
  --resource-group "${AZURE_RESOURCE_GROUP}" \
  --cluster-name "${CLUSTER_NAME}" \
  --name "${GPU_NODE_POOL_NAME}" \
  --node-count "${GPU_NODE_COUNT}" \
  --node-vm-size "${GPU_VM_SIZE}" \
  --enable-cluster-autoscaler \
  --min-count 1 \
  --max-count 3
```

## 5. Get AKS Credentials

Download and merge the kubeconfig for your AKS cluster:

```bash
az aks get-credentials \
  --resource-group "${AZURE_RESOURCE_GROUP}" \
  --name "${CLUSTER_NAME}" \
  --overwrite-existing
```

## 6. Install the NVIDIA Device Plugin (for GPU Clusters)

If you created a GPU node pool, install the NVIDIA device plugin so Pods can request GPUs:

```bash
kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.17.1/deployments/static/nvidia-device-plugin.yml
```

Confirm the DaemonSet is running:

```bash
kubectl get daemonset nvidia-device-plugin-daemonset -n kube-system
```

## 7. Install KubeAI

Add the KubeAI Helm repository and update:

```bash
helm repo add kubeai https://www.kubeai.org
helm repo update
```

**(Optional)** If you plan to use private or TOS-protected models from Hugging Face, export your token:

```bash
export HUGGING_FACE_HUB_TOKEN=<YOUR_HF_TOKEN>
```

Install KubeAI:

```bash
helm upgrade --install kubeai kubeai/kubeai \
  --set secrets.huggingface.token="${HUGGING_FACE_HUB_TOKEN}" \
  --wait
```

### Resource Profiles

For AKS GPU deployments using the upstream Kubernetes device plugin, you can customize resource requests and limits via KubeAI’s [values-nvidia-k8s-device-plugin.yaml](https://github.com/substratusai/kubeai/blob/main/charts/kubeai/values-nvidia-k8s-device-plugin.yaml).

We recommend referencing [the installation guide](../any/#installation-using-nvidia-gpus) for examples of how to specify these resource profiles within your Helm values. This file contains preconfigured resource profiles you can adjust for your environment.

## 8. Deploying models

Take a look at the following how-to guides to deploy models:

- [Configure Text Generation Models](../how-to/configure-text-generation-models.md)
- [Configure Embedding Models](../how-to/configure-embedding-models.md)
- [Configure Speech to Text Models](../how-to/configure-speech-to-text.md)
