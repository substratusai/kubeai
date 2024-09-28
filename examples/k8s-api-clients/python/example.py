from kubernetes import config, dynamic
from kubernetes.client import api_client

k8s_client = dynamic.DynamicClient(
    api_client.ApiClient(configuration=config.load_kube_config())
)

models_client = k8s_client.resources.get(api_version="kubeai.org/v1", kind="Model")

model = {
    "apiVersion": "kubeai.org/v1",
    "kind": "Model",
    "metadata": {
        "name": "facebook-opt-125m",
        "namespace": "default",
    },
    "spec": {
        "features": ["TextGeneration"],
        "owner": "facebook",
        "url": "hf://facebook/opt-125m",
        "engine": "VLLM",
        "resourceProfile": "cpu:1",
    },
}


models_client.create(body=model)

# Alternative: Use "server-side apply" (i.e. kubectl apply) to upsert the Model.
# models_client.patch(
#    body=model,
#    content_type="application/apply-patch+yaml",
#    field_manager="my-example-app",  # Set a field manager to track ownership of fields.
# )

created_model = models_client.get(name="facebook-opt-125m", namespace="default")
print(created_model)

# Optionally delete the Model.
# models_client.delete(name="facebook-opt-125m", namespace="default")
