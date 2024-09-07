# Configure Speech To Text

KubeAI provides a Speech to Text endpoint that can be used to transcribe audio files. This guide will walk you through the steps to enable this feature.

## Enable Speech to Text model
You can create new models by creating a Model CRD object or by enabling a model from the model catalog.

### Enable from model catalog
KubeAI provides predefined models in the model catalog. To enable the Speech to Text model, you can set the `enabled` flag to `true` in the `helm-values.yaml` file.

```yaml
models:
  catalog:
    faster-whisper-medium-en-cpu:
      enabled: true
      minReplicas: 1
```

### Enable by creating Model CRD
You can also create a Model CRD object to enable the Speech to Text model. Here is an example of a Model CRD object for the Speech to Text model:

```yaml
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: faster-whisper-medium-en-cpu
spec:
  features: [SpeechToText]
  owner: Systran
  url: hf://Systran/faster-whisper-medium.en
  engine: FasterWhisper
  minReplicas: 0
  maxReplicas: 3
  resourceProfile: cpu:1
```

## Usage
The Speech to Text endpoint is available at `/openai/v1/transcriptions`.

Example usage using curl:

```bash
curl -L -o kubeai.mp4 https://github.com/user-attachments/assets/711d1279-6af9-4c6c-a052-e59e7730b757
curl http://localhost:8000/openai/v1/audio/transcriptions \
  -F "file=@kubeai.mp4" \
  -F "language=en" \
  -F "model=faster-whisper-medium-en-cpu"
```
