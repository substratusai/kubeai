# Weaviate with local autoscaling embedding and generative models

Weaviate is a vector search engine that can integrate seamlessly with KubeAI's embedding and generative models. This tutorial demonstrates how to deploy both KubeAI and Weaviate in a Kubernetes cluster, using KubeAI as the OpenAI endpoint for Weaviate.

This tutorial uses CPU only models, so it should work even on your laptop.

As you go go through this tutorial, you will learn how to:
- Deploy KubeAI with embedding and generative models
- Install Weaviate and connect it to KubeAI
- Import data into Weaviate
- Perform semantic search using the embedding model
- Perform generative search using the generative model

## Prerequisites
A Kubernetes cluster. You can use [kind](https://kind.sigs.k8s.io/) or [minikube](https://minikube.sigs.k8s.io/docs/start/).

```bash
kind create cluster
```

## KubeAI Configuration
Let's start by deploying KubeAI with the models we want to use.
Nomic embedding model is used instead of text-embedding-ada-002.
Gemma 2 2B is used instead of gpt-3.5-turbo.
You could choose to use bigger models depending on your available hardware.

Create a file named `kubeai-values.yaml` with the following content:
```yaml
models:
  catalog:
    text-embedding-ada-002:
      enabled: true
      minReplicas: 1
      features: ["TextEmbedding"]
      owner: nomic
      url: "ollama://nomic-embed-text"
      engine: OLlama
      resourceProfile: cpu:1
    gpt-3.5-turbo:
      enabled: true
      minReplicas: 1
      features: ["TextGeneration"]
      owner: google
      url: "ollama://gemma2:2b"
      engine: OLlama
      resourceProfile: cpu:2
```

Note: It's important that you name the models as `text-embedding-ada-002` and `gpt-3.5-turbo` as Weaviate expects these names.

Run the following command to deploy KubeAI:
```bash
helm upgrade --install kubeai kubeai/kubeai \
    -f ./kubeai-values.yaml --reuse-values
```

## Weaviate Installation
For this tutorial, we will use the Weaviate Helm chart to deploy Weaviate.

Let's enable the text2vec-openai and generative-openai modules in Weaviate.
We will also set the default vectorizer module to text2vec-openai.

The `apiKey` is ignored in this case as we are using KubeAI as the OpenAI endpoint.

Create a file named `weaviate-values.yaml` with the following content:
```yaml
modules:
  text2vec-openai:
    enabled: true
    apiKey: thisIsIgnored
  generative-openai:
    enabled: true
    apiKey: thisIsIgnored
  default_vectorizer_module: text2vec-openai
service:
  # To prevent Weaviate being exposed publicly
  type: ClusterIP
```


Install Weaviate by running the following command:
```bash
helm repo add weaviate https://weaviate.github.io/weaviate-helm
helm upgrade --install \
  "weaviate" \
  weaviate/weaviate \
  -f weaviate-values.yaml
```

## Usage
We will be using Python to interact with Weaviate.
The 2 use cases we will cover are:
- Semantic search using the embedding model
- Generative search using the generative model

### Connectivity
The remaining steps require connectivity to the Weaviate service.
However, Weaviate is not exposed publicly in this setup.
So we setup a local port forwards to access the Weaviate services.

Setup a local port forwards to the Weaviate services by running:
```bash
kubectl port-forward svc/weaviate 8080:80
kubectl port-forward svc/weaviate-grpc 50051:50051
```

### Weaviate client Python Setup
Create a virtual environment and install the Weaviate client:
```bash
python -m venv .venv
source .venv/bin/activate
pip install -U weaviate-client requests
```

### Collection and Data Import
Create a file named `create-collection.py` with the following content:
```python
import json
import weaviate
import requests
from weaviate.classes.config import Configure

# This works due to port forward in previous step
with weaviate.connect_to_local(port=8080, grpc_port=50051) as client:

    client.collections.create(
        "Question",
        vectorizer_config=Configure.Vectorizer.text2vec_openai(
                model="text-embedding-ada-002",
                base_url="http://kubeai/openai",
        ),
        generative_config=Configure.Generative.openai(
            model="gpt-3.5-turbo",
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
    print("Data imported successfully")
```

Create a collection that uses KubeAI as the openAI endpoint:
```bash
python create-collection.py
```
You should see a message `Data imported successfully`.

The collection is now created and data is imported. The vectors are generated by KubeAI and stored in Weaviate.

### Semantic Search

Now let's do semantic search, which uses the embeddings. Create a file named `search.py` with the following content:
```python
import weaviate
from weaviate.classes.config import Configure

# This works due to port forward in previous step
with weaviate.connect_to_local(port=8080, grpc_port=50051) as client:
    questions = client.collections.get("Question")
    response = questions.query.near_text(
        query="biology",
        limit=2
    )
    print(response.objects[0].properties)  # Inspect the first object
```

Execute the python script:
```bash
python search.py
```

You should see the following output:
```json
{
  "answer": "DNA",
  "question": "In 1953 Watson & Crick built a model of the molecular structure of this, the gene-carrying substance",
  "category": "SCIENCE"
}
```

### Generative Search (RAG)
Now let's do generative search, which uses the generative model (Text generation LLM).
The generative model is run locally and managed by KubeAI.

Create a file named `generate.py` with the following content:
```python
import weaviate
from weaviate.classes.config import Configure

# This works due to port forward in previous step
with weaviate.connect_to_local(port=8080, grpc_port=50051) as client:
    questions = client.collections.get("Question")

    response = questions.generate.near_text(
        query="biology",
        limit=2,
        grouped_task="Write a tweet with emojis about these facts."
    )

    print(response.generated)  # Inspect the generated text
```

Run the python script:
```bash
python generate.py
```

You should see something similar to this:

> 🧬 **Watson & Crick** cracked the code in 1953!  🤯 They built a model of DNA, the blueprint of life. 🧬  
> 🧠 **Liver power!** 💪 This organ keeps your blood sugar balanced by storing glucose as glycogen. 🩸 #ScienceFacts #Biology

## Conclusion
You've now successfully set up KubeAI with Weaviate for both embedding-based semantic search and generative tasks. You've also learned how to import data, perform searches, and generate content using KubeAI-managed models.
