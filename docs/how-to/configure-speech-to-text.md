# Configure speech-to-text

KubeAI provides a Speech to Text endpoint that can be used to transcribe audio files. This guide will walk you through the steps to enable this feature.

## Enable Speech to Text model
You can create new models by creating a Model CRD object or by enabling a model from the model catalog.

### Enable from model catalog
KubeAI provides predefined models in the `kubeai/models` Helm chart. To enable the Speech to Text model, you can set the `enabled` flag to `true` in your values file.

```yaml
# models-helm-values.yaml
catalog:
  faster-whisper-medium-en-cpu:
    enabled: true
    minReplicas: 1
```

### Enable by creating Model
You can also create a Model object to enable the Speech to Text model. For example:

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
