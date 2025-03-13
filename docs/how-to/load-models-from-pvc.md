# Load Models from PVC

You can store your models in a Persistent Volume Claim (PVC) and let KubeAI use them for serving.

## Supported Engines

Both vLLM and Ollama engines support loading models from PVCs.

### vLLM
For vLLM, use the following URL format:
```yaml
url: pvc://$PVC_NAME          # Loads the model from the PVC named $PVC_NAME
url: pvc://$PVC_NAME/$PATH    # Loads from a specific path within the PVC
```


### Ollama
For Ollama, use the following URL formats:
```yaml
url: pvc://$PVC_NAME?model=$MODEL_NAME    # Loads the model named $MODEL_NAME that's loaded on the disk
url: pvc://$PVC_NAME/$PATH?model=$MODEL_NAME
```

For example, if you ran `ollama pull qwen:0.5b` to preload the model on your PVC named `my-pvc`. Then the PVC disk should have the following directories:
```
blobs/
manifests/registry.ollama.ai/library/qwen/0.5b
```

The correct KubeAI Model config would then be:
```yaml
url: pvc://my-pvc?model=qwen:0.5b
```


## PVC Requirements

1. **Access Mode**: The PVC should use either `ReadOnlyMany` or `ReadWriteMany` access mode. This is required to support multiple model replicas.

2. **Storage**: Ensure sufficient storage capacity for your model files. The recommended minimum is 10Gi.

3. **Pre-loading**: You must ensure the model files are already present in the PVC before creating the Model resource.


## Implementation details

1. **Mounting**: KubeAI will automatically mount the PVC at:
    - vLLM: `/model` directory in the model server pod
    - Ollama: `/model` directory in the Ollama server pod

2. **Environment Variables**: 
    - For Ollama, KubeAI automatically sets `OLLAMA_MODELS=/model` to ensure models are loaded from the PVC
    - For vLLM, the model path is automatically configured through command-line arguments

## Best Practices

1. **Storage Class**: Use a storage class that supports your access mode requirements (`ReadOnlyMany` or `ReadWriteMany`).

2. **Model Organization**: When storing multiple models in a single PVC, use a clear directory structure and specify the correct subpaths in your model configurations.

3. **Monitoring**: Monitor your PVC usage to ensure sufficient storage space for your models.
