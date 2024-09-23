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

## OpenAI Client libaries
You can use the official OpenAI client libraries by setting the
`base_url` to the KubeAI endpoint.

For example, you can use the Python client like this:
```python
from openai import OpenAI
client = OpenAI(api_key="ignored",
                base_url="http://kubeai/openai/v1")
response = client.chat.completions.create(
  model="gemma2-2b-cpu",
  messages=[
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Who won the world series in 2020?"},
    {"role": "assistant", "content": "The Los Angeles Dodgers won the World Series in 2020."},
    {"role": "user", "content": "Where was it played?"}
  ]
)
```
