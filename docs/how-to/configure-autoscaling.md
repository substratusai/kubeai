# Configure autoscaling

This guide with cover how to configure KubeAI [autoscaling](../concepts/autoscaling.md) parameters.

## System Settings

KubeAI administrators can define system-wide autoscaling settings by setting the following helm values:

Example:

```yaml
# helm-values.yaml
modelAutoscaling:
  interval: 15s
  timeWindow: 10m
# ...
```

## Model Settings

The following settings can be configured on a model-by-model basis.

**NOTE:** Updates to model settings will be applied upon scale up of additional model Pods. You can force-restart model servers by running `kubectl delete pods -l model=<model-name>`.

### Model settings: helm

If you are managing models via Helm, you can use:

```yaml
# helm-values.yaml
models:
  catalog:
    model-a:
      # ...
      minReplicas: 1
      maxReplicas: 9
      targetRequests: 250
      scaleDownDelaySeconds: 45
    model-b:
      # ...
      disableAutoscaling: true
# ...
```

Re-running `helm upgrade` with these additional parameters will update model settings in the cluster.

### Model settings: kubectl

You can also specify the autoscaling profile directly via the Models custom resource in the Kubernetes API:

```yaml
apiVersion: kubeai.org/v1
kind: Model
metadata:
  name: my-model
spec:
  # ...
  minReplicas: 1
  maxReplicas: 9
  targetRequests: 250
  scaleDownDelaySeconds: 45
```

If you are already managing models using Model manifest files, you can make the update to your file and reapply it using `kubectl apply -f <filename>.yaml`.
