# Install on EKS

<details markdown="1">
<summary>TIP: Make sure you have enough GPU quota in your AWS account.</summary>

The default quotas for GPU instances are often 0. You will need to request a quota increase for the GPU instances you want to use.

The following quotas may require an increase if you wish to use GPUs in your EKS cluster:
- All G and VT Spot Instance Requests
- All P5 Spot Instance Requests
- All P4, P3 and P2 Spot Instance Requests
- Running Dedicated p4d Hosts

</details>

## 1. Create EKS cluster with Karpenter

Set the environment variables used throughout this guide:

```bash
export CLUSTER_NAME="cluster-with-karpenter"
export AWS_DEFAULT_REGION="us-west-2"
export K8S_VERSION="1.30"
export GPU_AMI_ID="$(aws ssm get-parameter --name /aws/service/eks/optimized-ami/${K8S_VERSION}/amazon-linux-2-gpu/recommended/image_id --query Parameter.Value --output text)"
```

Create the EKS cluster using eksctl:
```bash
eksctl create cluster -f - <<EOF
---
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: "${CLUSTER_NAME}"
  region: "${AWS_DEFAULT_REGION}"
  version: "${K8S_VERSION}"
  tags:
    karpenter.sh/discovery: "${CLUSTER_NAME}" # here, it is set to the cluster name

iam:
  withOIDC: true # required

karpenter:
  version: '1.0.6' # Exact version must be specified

managedNodeGroups:
- instanceType: m5.large
  amiFamily: AmazonLinux2
  name: "${CLUSTER_NAME}-m5-ng"
  desiredCapacity: 2
  minSize: 1
  maxSize: 10
EOF
```

## 2. Configure a Karpenter GPU NodePool

Create the NodePool and EC2NodeClass objects:

```bash
kubectl apply -f - <<EOF
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: gpu
spec:
  template:
    spec:
      requirements:
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["spot", "on-demand"]
        - key: karpenter.k8s.aws/instance-category
          operator: In
          values: ["g", "p"]
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: gpu
      expireAfter: 720h # 30 * 24h = 720h
      taints:
      - key: nvidia.com/gpu
        value: "true"
        effect: NoSchedule
  limits:
    cpu: 1000
  disruption:
    consolidationPolicy: WhenEmptyOrUnderutilized
    consolidateAfter: 1m
---
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: gpu
spec:
  amiFamily: AL2 # Amazon Linux 2
  role: "eksctl-KarpenterNodeRole-${CLUSTER_NAME}"
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}" # replace with your cluster name
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}" # replace with your cluster name
  amiSelectorTerms:
    - id: "${GPU_AMI_ID}" # <- GPU Optimized AMD AMI 
  blockDeviceMappings:
    - deviceName: /dev/xvda
      ebs:
        volumeSize: 300Gi
        volumeType: gp3
        encrypted: true
EOF
```

Install the NVIDIA device plugin (needed for Karpenter nodes):

```bash
kubectl create -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.16.1/deployments/static/nvidia-device-plugin.yml
```

## 3. Install KubeAI

Add KubeAI Helm repository.

```bash
helm repo add kubeai https://www.kubeai.org
helm repo update
```

**Make sure** you have a HuggingFace Hub token set in your environment (`HUGGING_FACE_HUB_TOKEN`).

```bash
export HF_TOKEN="replace-with-your-huggingface-token"
```

Install KubeAI with [Helm](https://helm.sh/docs/intro/install/).

```bash
curl -L -O https://raw.githubusercontent.com/substratusai/kubeai/refs/heads/main/charts/kubeai/values-eks.yaml
# Please review the values-eks.yaml file and edit the nodeSelectors if needed.
cat values-eks.yaml
helm upgrade --install kubeai kubeai/kubeai \
    -f values-eks.yaml \
    --set secrets.huggingface.token=$HF_TOKEN \
    --wait
```

## 4. Deploying models

Take a look at the following how-to guides to deploy models:
* [Configure Text Generation Models](../how-to/configure-text-generation-models.md)
* [Configure Embedding Models](../how-to/configure-embedding-models.md)
* [Configure Speech to Text Models](../how-to/configure-speech-to-text.md)
