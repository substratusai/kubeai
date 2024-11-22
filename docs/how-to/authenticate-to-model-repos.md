# Authenticate to model repos

KubeAI supports the following private model repositories.

## HuggingFace Hub

Example model url: `hf://meta-llama/Llama-3.1-8B-Instruct`

Authentication is required when loading models or adapters from HuggingFace Hub when using a private Hub or accessing a public model that requires agreeing to terms of service.

When using Helm to manage your KubeAI installation, you can pass your credentials as follows:

```bash
helm upgrade --install kubeai kubeai/kubeai \
    --set secrets.huggingface.token=$HUGGING_FACE_HUB_TOKEN \
    ...
```

## S3

Example model url: `s3://my-private-model-bucket/my-models/llama-3.1-8b-instruct`

Authenication is required when access models or adapters from private S3 buckets.

When using Helm to manage your KubeAI installation, you can pass your credentials as follows:

```bash
helm upgrade --install kubeai kubeai/kubeai \
    --set secrets.aws.accessKeyId=$AWS_ACCESS_KEY_ID \
    --set secrets.aws.secretAccessKey=$AWS_SECRET_ACCESS_KEY \
    ...
```
