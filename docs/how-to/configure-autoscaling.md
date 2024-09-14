# Configure Autoscaling

This guide with cover how to configure KubeAI [autoscaling](../concepts/autoscaling.md) parameters.

## Helm

KubeAI administrators can define system-wide autoscaling profiles by modifying the Helm chart.

Example:

```yaml
# helm-values.yaml
autoscalingProfiles:
  default:
    minReplicas: 0
    maxReplicas: 3
    targetRequests: 100
  online:
    minReplicas: 1
    maxReplicas: 1000
    targetRequests: 100
    scaleDownDelay: 10m
  budget:
    minReplicas: 0
    maxReplicas: 3
    targetRequests: 200
    scaleDownDelay: 30s
  off:
    disabled: true
```

Models can be configured with given autoscaling Profiles either via Helm:

```yaml
# helm-values.yaml
models:
  catalog:
    gemma2-2b-cpu:
      enabled: true
      autoscalingProfile: "budget"
```

You can also specify the autoscaling profile directly via the Models custom resource in the Kubernetes API:

```yaml
# model.yaml
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: my-model
spec:
  # ...
  autoscalingProfile: "budget"
```

## Specifying autoscaling details in-Model

Models can optionally define autoscaling details inline instead of selecting a system-wide autoscaling profile.

```yaml
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: my-model
spec:
  # ...
  autoscaling:
    minReplicas: 1
    maxReplicas: 9
    targetRequests: 250
    scaleDownDelay: 30s
```