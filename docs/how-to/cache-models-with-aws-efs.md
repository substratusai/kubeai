# Cache models with AWS EFS

KubeAI can manage model caches. AWS EFS is supported as a pluggable backend store.

<br>
<img src="/diagrams/caching-shared-filesystem.excalidraw.png" width="90%"></img>

Follow the [EKS install guide](../installation/eks.md).

## 1. Create an EFS File System

Set environment variables to match your environment.

```bash
export CLUSTER_NAME="cluster-with-karpenter"
```

Create an EFS file system in the same VPC as your EKS cluster.

```bash
vpc_id=$(aws eks describe-cluster \
    --name $CLUSTER_NAME \
    --query "cluster.resourcesVpcConfig.vpcId" \
    --output text)

cidr_range=$(aws ec2 describe-vpcs \
    --vpc-ids $vpc_id \
    --query "Vpcs[].CidrBlock" \
    --output text \
    --region ${AWS_DEFAULT_REGION})

security_group_id=$(aws ec2 create-security-group \
    --group-name MyEfsSecurityGroup \
    --description "My EFS security group" \
    --vpc-id $vpc_id \
    --output text)

aws ec2 authorize-security-group-ingress \
    --group-id $security_group_id \
    --protocol tcp \
    --port 2049 \
    --cidr $cidr_range

file_system_id=$(aws efs create-file-system \
    --region ${AWS_DEFAULT_REGION} \
    --performance-mode generalPurpose \
    --query 'FileSystemId' \
    --output text)
```

Expose the EFS file system to the subnets used by your EKS cluster.
```bash
SUBNETS=$(eksctl get cluster --region us-west-2 ${CLUSTER_NAME} -o json | jq -r '.[0].ResourcesVpcConfig.SubnetIds[]')

while IFS= read -r subnet; do
    echo "Creating EFS mount target in $subnet"
    aws efs create-mount-target --file-system-id $file_system_id \
      --subnet-id $subnet --security-groups $security_group_id
done <<< "$SUBNETS"
```

## 2. Install the EFS CSI driver

```bash
export role_name=AmazonEKS_EFS_CSI_DriverRole
eksctl create iamserviceaccount \
    --name efs-csi-controller-sa \
    --namespace kube-system \
    --cluster ${CLUSTER_NAME} \
    --role-name $role_name \
    --role-only \
    --attach-policy-arn arn:aws:iam::aws:policy/service-role/AmazonEFSCSIDriverPolicy \
    --approve

TRUST_POLICY=$(aws iam get-role --role-name $role_name \
    --query 'Role.AssumeRolePolicyDocument' --output json | \
    sed -e 's/efs-csi-controller-sa/efs-csi-*/' -e 's/StringEquals/StringLike/')

aws iam update-assume-role-policy --role-name $role_name --policy-document "$TRUST_POLICY"

# Get the role ARN
EFS_ROLE_ARN=$(aws iam get-role --role-name AmazonEKS_EFS_CSI_DriverRole \
  --query 'Role.Arn' --output text)

aws eks create-addon --cluster-name $CLUSTER_NAME --addon-name aws-efs-csi-driver \
  --service-account-role-arn $EFS_ROLE_ARN
```

Wait for EKS Addon to active.
```bash
aws eks wait addon-active --cluster-name $CLUSTER_NAME \
  --addon-name aws-efs-csi-driver
```
Verify that the EFS CSI driver is running.

```bash
kubectl get daemonset efs-csi-node -n kube-system
```

Create a storage class for using EFS dynamic mode.

```bash
kubectl apply -f - <<EOF
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: efs-sc
provisioner: efs.csi.aws.com
parameters:
  provisioningMode: efs-ap
  fileSystemId: "${file_system_id}"
  directoryPerms: "700"
EOF
```

Make sure to set `file_system_id` match the EFS file system ID created in the first step.

## 3. Configure KubeAI with the EFS cache profile

You can skip this step if you've already installed KubeAI using the [EKS Helm values file: values-eks.yaml](https://github.com/substratusai/kubeai/blob/main/charts/kubeai/values-eks.yaml) file.

Configure KubeAI with the `efs-dynamic` cache profile.
```bash
helm upgrade --install kubeai kubeai/kubeai \
  --reuse-values -f - <<EOF
cacheProfiles:
  efs-dynamic:
    sharedFilesystem:
      storageClassName: "efs-sc"
  efs-static:
    sharedFilesystem:
      persistentVolumeName: "efs-pv"
EOF
```

## 4. Configure a model to use the EFS cache

Apply a Model with `cacheProfile` set to `efs-dynamic`.

NOTE: If you already installed the models chart, you will need to edit you values file and run `helm upgrade`.

```bash
helm install kubeai-models kubeai/models -f - <<EOF
catalog:
  llama-3.1-8b-instruct-fp8-l4:
    enabled: true
    cacheProfile: efs-dynamic
EOF
```

Wait for the Model to be fully cached.

```bash
kubectl wait --timeout 10m --for=jsonpath='{.status.cache.loaded}'=true model/llama-3.1-8b-instruct-fp8-l4
```

This model will now be loaded from Filestore when it is served.

## Troubleshooting

### Model Loading Job

Check to see if there is an ongoing model loader Job.

```bash
kubectl get jobs
```