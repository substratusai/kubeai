# Using LangChain with KubeAI

LangChain makes it easy to build applications powered by LLMs.
[KubeAI](https://github.com/substratusai/kubeai) makes
it easy to deploy and manage LLMs at scale. Together, they make it easy to
build and deploy private and secure AI applications.

In this tutorial, we'll show you how to use LangChain with KubeAI's OpenAI
compatible API. The beauty of KubeAI's OpenAI compatibility is that you can use
KubeAI with any framework that supports OpenAI.

## Prerequisites
A K8s cluster. You can use a local cluster like [kind](https://kind.sigs.k8s.io/).

## Installing KubeAI with Gemma 2B

Run the following command to install KubeAI with Gemma 2B:

```bash
helm repo add kubeai https://www.kubeai.org
cat <<EOF > helm-values.yaml
models:
  catalog:
    gemma2-2b-cpu:
      enabled: true
      minReplicas: 1
EOF

helm upgrade --install kubeai kubeai/kubeai \
    -f ./helm-values.yaml \
    --wait --timeout 10m
```

## Using LangChain
Install the required Python packages:
```bash
pip install langchain_openai
```

Let's access the KubeAI OpenAI compatible API locally to make it easier.

Run the following command to port-forward to the KubeAI service:
```bash
kubectl port-forward svc/kubeai 8000:80
```
Now the KubeAI OpenAI compatible API is available at `http://localhost:8000/openai`
from your local machine.

Let's create a simple Python script that uses LangChain and is connected to KubeAI.

Create a file named `test-langchain.py` with the following content:
```python
from langchain_openai import ChatOpenAI

llm = ChatOpenAI(
    model="gemma2-2b-cpu",
    temperature=0,
    max_tokens=None,
    timeout=None,
    max_retries=2,
    api_key="thisIsIgnored",
    base_url="http://localhost:8000/openai/v1",
)

messages = [
    (
        "system",
        "You are a helpful assistant that translates English to French. Translate the user sentence.",
    ),
    ("human", "I love programming."),
]
ai_msg = llm.invoke(messages)
print(ai_msg.content)
```

Run the Python script:
```bash
python test-langchain.py
```

Notice that we set base_url to `http://localhost:8000/openai/v1`. This tells
LangChain to use our local KubeAI OpenAI compatible AP instead of the default
OpenAI public API.

If you run langchain within the K8s cluster, you can use the following base_url instead:
`http://kubeai/openai/v1`. So the code would look like this:
```python
llm = ChatOpenAI(
    ...
    base_url="http://kubeai/openai/v1",
)
```

That's it! You've successfully used LangChain with KubeAI. Now you can build
and deploy private and secure AI applications with ease.
