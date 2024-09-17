# OpenAI API Compatibility

KubeAI provides an OpenAI API compatiblity layer.

## General:

### Models

```
GET /v1/models
```

* Lists all `kind: Model` object installed in teh Kubernetes API Server.


## Inference

### Text Generation

```
POST /v1/chat/completions
POST /v1/completions
```

* Supported for Models with `.spec.features: ["TextGeneration"]`.

### Embeddings

```
POST /v1/embeddings
```

* Supported for  Models with `.spec.features: ["TextEmbedding"]`.

### Speech-to-Text

```
POST /v1/audio/transcriptions
```

* Supported for Models with `.spec.features: ["SpeechToText"]`.