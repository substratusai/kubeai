
Deploy KubeAI with an embedding model and text generation model:
```yaml
models:
  catalog:
    nomic-embed-text-cpu:
      enabled: true
      features: ["TextEmbedding"]
      owner: nomic
      url: "ollama://nomic-embed-text"
      engine: OLlama
      resourceProfile: cpu:1
    gemma2-2b-cpu:
      enabled: false
      features: ["TextGeneration"]
      owner: google
      url: "ollama://gemma2:2b"
      engine: OLlama
      resourceProfile: cpu:2
```

```bash
helm upgrade --install kubeai kubeai/kubeai \
    -f ./helm-values.yaml --reuse-values
```

Install Weaviate:
```
helm repo add weaviate https://weaviate.github.io/weaviate-helm
helm upgrade --install \
  "weaviate" \
  weaviate/weaviate \
  -f weaviate-values.yaml
```

Setup a local port forwards to the Weaviate services:
```bash
kubectl port-forward svc/weaviate 8080:80
kubectl port-forward svc/weaviate-grpc 50051:50051
```

Create a file named `create-collection.py` with the following content:
```python
import json
import weaviate
import requests
from weaviate.classes.config import Configure

# This works due to port forward in previous step
client = weaviate.connect_to_local(port=8080, grpc_port=50051,
            headers={
        "X-OpenAI-Api-Key": "thisIsIgnored",
        # "X-OpenAI-BaseURL": "http://localhost:8000/openai/v1",
    })

client.collections.create(
    "Question",
    vectorizer_config=Configure.Vectorizer.text2vec_openai(
            model="nomic-embed-text-cpu",
            base_url="http://kubeai/openai",
    ),
    generative_config=Configure.Generative.openai(
        model="gemma2-2b-cpu",
        base_url="http://kubeai/openai",
    ),
)

# import data
resp = requests.get('https://raw.githubusercontent.com/weaviate-tutorials/quickstart/main/data/jeopardy_tiny.json')
data = json.loads(resp.text)  # Load data

question_objs = list()
for i, d in enumerate(data):
    question_objs.append({
        "answer": d["Answer"],
        "question": d["Question"],
        "category": d["Category"],
    })

questions = client.collections.get("Question")
questions.data.insert_many(question_objs)

questions = client.collections.get("Question")

response = questions.query.near_text(
    query="biology",
    limit=2
)

print(response.objects[0].properties)  # Inspect the first object
```

Create a collection that uses KubeAI as the openAI endpoint:
```bash
pip install -U weaviate-client requests
python create-collection.py
```