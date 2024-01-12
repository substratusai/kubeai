# Backends

Lingo backends are expected to serve models via an OpenAI-compatible API.

## Routing

Lingo will select a backend based on the `X-Model` header or the `.model` field in the JSON request body.

## Deployments

Lingo manages the replicas of Kubernetes Deployments.

Annotations:

| Annotation | Required | Default | Description |
| ---------- | -------- | ------- | ----------- |
| `lingo.substratus.ai/models` | Required | `N/A` | Comma-separated list of models served |
| `lingo.substratus.ai/min-replicas` | Optional | `0` | Minimum number of replicas to autoscale down to |
| `lingo.substratus.ai/max-replicas` | Optional | `3` | Maximum number of replicas to autoscale up to |

## Services

Lingo will keep track of Pods associated with Kubernetes Services tied to backend Deployments.

* Lingo will forward traffic to a backend Service with the same name as the relevant Deployment.
* If one port exists, lingo will send traffic to it.
* If more than one port exists, lingo will send traffic to the port named `http`.
