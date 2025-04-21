# Authenticate to model repos

KubeAI supports the following private model repositories, and different authentication methods.

## Helm

### Alibaba Object Storage Service

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

### Google Cloud Storage

Example url: `gs://my-gcs-bucket/my-models/llama-3.1-8b-instruct`

Authenication is required when accessing models or adapters from private GCS buckets.

When using Helm to manage your KubeAI installation, you can pass your credentials as follows:

```bash
helm upgrade --install kubeai kubeai/kubeai \
    --set secrets.gcp.jsonKeyfile=$MY_JSON_KEYFILE \
    ...
```

**NOTE:** KubeAI does not automatically react to updates to credentials. You will need to manually delete and allow KubeAI to recreate any failed Jobs/Pods that required credentials.

### HuggingFace Hub

Example model url: `hf://meta-llama/Llama-3.1-8B-Instruct`

Authentication is required when loading models or adapters from HuggingFace Hub when using a private Hub or accessing a public model that requires agreeing to terms of service.

When using Helm to manage your KubeAI installation, you can pass your credentials as follows:

```bash
helm upgrade --install kubeai kubeai/kubeai \
    --set secrets.huggingface.token=$HUGGING_FACE_HUB_TOKEN \
    ...
```

**NOTE:** KubeAI does not automatically react to updates to credentials. You will need to manually delete and allow KubeAI to recreate any failed Jobs/Pods that required credentials.

### S3

Example model url: `s3://my-private-model-bucket/my-models/llama-3.1-8b-instruct`

Authenication is required when accessing models or adapters from private S3 buckets.

When using Helm to manage your KubeAI installation, you can pass your credentials as follows:

```bash
helm upgrade --install kubeai kubeai/kubeai \
    --set secrets.aws.accessKeyID=$AWS_ACCESS_KEY_ID \
    --set secrets.aws.secretAccessKey=$AWS_SECRET_ACCESS_KEY \
    ...
```

**NOTE:** KubeAI does not automatically react to updates to credentials. You will need to manually delete and allow KubeAI to recreate any failed Jobs/Pods that required credentials.

## Model Spec

You can also pass credentials using envFrom in the model spec.
This is an example how to confiure an S3 self managed instance with and envFrom.

```
apiVersion: v1
kind: Secret
metadata:
  name: set1
type: kubernetes.io/basic-auth
stringData:
  AWS_ACCESS_KEY_ID: test
  AWS_SECRET_ACCESS_KEY: testtest
  HF_TOKEN: secret
---
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: llama-test-s3
spec:
  features: [TextGeneration]
  owner: meta-llama
  url: s3://models/Llama-3.2-1B
  adapters: # <--
  - name: llama-adapter
    url: s3://adapters/llama-3.1-8b-ocr-correction
  engine: VLLM
  env:
    AWS_ENDPOINT_URL: http://locals3:9000
  envFrom:
    - secretRef:
        name: set1
```

**NOTE:** If both configuration methods are used, the helm method takes precedence.
