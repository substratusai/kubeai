# Auth

The KubeAI system is configured to trust a configured header.

```yaml
auth:
  http:
    trustedHeader: X-Auth-Groups
  # Possibly in future: configure Model roles.
  # modelRoles:
  #   user: ["list", "describe", "infer"]
```

The groups associated with a request are passed in a trusted header.

```bash
curl http://localhost:8000/openai/v1/completions \
    -H "X-Auth-Groups: group-a, group-b"
```

The groups that are allowed to access a given model are configured as labels on the Model.

```yaml
kind: Model
metadata:
  name: llama-3.2
  labels:
    auth.kubeai.org/group-a: user
    auth.kubeai.org/group-b: user
```