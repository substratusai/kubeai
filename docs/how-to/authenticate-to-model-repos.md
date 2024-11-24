# Authenticate to model repos

KubeAI supports the following private model repositories.

## Alibaba Object Storage Service

Example url: `oss://my-oss-bucket/my-models/llama-3.1-8b-instruct`

Authenication is required when accessing models or adapters from private OSS buckets.

When using Helm to manage your KubeAI installation, you can pass your credentials as follows:

```bash
helm upgrade --install kubeai kubeai/kubeai \
    --set secrets.alibaba.accessKeyID=$OSS_ACCESS_KEY_ID \
    --set secrets.alibaba.accessKeySecret=$OSS_ACCESS_KEY_SECRET \
    ...
```

**NOTE:** KubeAI does not automatically react to updates to credentials. You will need to manually delete and allow KubeAI to recreate any failed Jobs/Pods that required credentials.

## Google Cloud Storage

Example url: `gs://my-gcs-bucket/my-models/llama-3.1-8b-instruct`

Authenication is required when accessing models or adapters from private GCS buckets.

When using Helm to manage your KubeAI installation, you can pass your credentials as follows:

```bash
helm upgrade --install kubeai kubeai/kubeai \
    --set secrets.gcp.jsonKeyfile=$MY_JSON_KEYFILE \
    ...
```

**NOTE:** KubeAI does not automatically react to updates to credentials. You will need to manually delete and allow KubeAI to recreate any failed Jobs/Pods that required credentials.

## HuggingFace Hub

Example model url: `hf://meta-llama/Llama-3.1-8B-Instruct`

Authentication is required when loading models or adapters from HuggingFace Hub when using a private Hub or accessing a public model that requires agreeing to terms of service.

When using Helm to manage your KubeAI installation, you can pass your credentials as follows:

```bash
helm upgrade --install kubeai kubeai/kubeai \
    --set secrets.huggingface.token=$HUGGING_FACE_HUB_TOKEN \
    ...
```

**NOTE:** KubeAI does not automatically react to updates to credentials. You will need to manually delete and allow KubeAI to recreate any failed Jobs/Pods that required credentials.

## S3

Example model url: `s3://my-private-model-bucket/my-models/llama-3.1-8b-instruct`

Authenication is required when accessing models or adapters from private S3 buckets.

When using Helm to manage your KubeAI installation, you can pass your credentials as follows:

```bash
helm upgrade --install kubeai kubeai/kubeai \
    --set secrets.aws.accessKeyId=$AWS_ACCESS_KEY_ID \
    --set secrets.aws.secretAccessKey=$AWS_SECRET_ACCESS_KEY \
    ...
```

**NOTE:** KubeAI does not automatically react to updates to credentials. You will need to manually delete and allow KubeAI to recreate any failed Jobs/Pods that required credentials.