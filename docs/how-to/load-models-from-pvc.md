# Load Models from PVC

You can store your models in a Persistent Volume Claim (PVC) and let KubeAI use them for serving.
Both vLLM and Ollama engines support loading models from PVCs.

You must ensure the model files are already present in the PVC before creating the Model resource.
Alternatively you can use KubeAI's native caching mechanism which downloads the model for you:

- [Cache Models with GCP Filestore](./cache-models-with-gcp-filestore.md)
- [Cache Models with EFS](./cache-models-with-aws-efs.md)


## vLLM

For vLLM, use the following URL format:
```yaml
url: pvc://$PVC_NAME          # Loads the model from the PVC named $PVC_NAME
url: pvc://$PVC_NAME/$PATH    # Loads from a specific path within the PVC
```

### PVC requirements

vLLM supports both ReadWriteMany and ReadOnlyMany access modes. `Many` is used in order to support more than 1 vLLM replica.


## Ollama

For Ollama, use the following URL formats:
```yaml
url: pvc://$PVC_NAME?model=$MODEL_NAME    # Loads the model named $MODEL_NAME that's loaded on the disk
url: pvc://$PVC_NAME/$PATH?model=$MODEL_NAME
```

### PVC Requirements
Ollama requires using ReadWriteMany access mode because the rename operation `ollama cp` needs to write to the PVC.

### Example: Loading Qwen 0.5b from PVC

1. Create a PVC with ReadWriteMany named `model-pvc`. See [example](https://github.com/substratusai/kubeai/blob/main/examples/ollama-pvc/pvc.yaml).
2. Create a K8s Job to load the model onto `model-pvc. See [example](https://github.com/substratusai/kubeai/blob/main/examples/ollama-pvc/job.yaml)

    The PVC should now have a `blobs/` and `manifests/` directory after the loader completes.


3. Create a Model to load from PVC:
   
   ```yaml
   url: pvc://model-pvc?model=qwen:0.5b
   ```
