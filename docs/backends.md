# Backends

Lingo backends are expected to serve models via an OpenAI-compatible API.

## Deployments

Annotations:

| Annotation | Required | Default | Description |
| ---------- | -------- | ------- | ----------- |
| `lingo.substratus.ai/models` | Required | `N/A` | Comma-separated list of models served |
| `lingo.substratus.ai/min-replicas` | Optional | `0` | Minimum number of replicas to autoscale down to |
| `lingo.substratus.ai/max-replicas` | Optional | `0` | Maximum number of replicas to autoscale up to |

## Services

* Lingo will forward traffic to a backend Service with the same name as the relevant Deployment.
* If one port exists, lingo will send traffic to it.
* If more than one port exists, lingo will send traffic to the port named `http`.
