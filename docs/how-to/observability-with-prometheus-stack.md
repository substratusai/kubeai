# Configure Observability with Prometheus Stack

KubeAI provides a vLLM PodMonitor resource to scrape vLLM metrics.

The podMonitor can be enabled by using the following KubeAI helm values:

```yaml
metrics:
  prometheusOperator:
    vLLMPodMonitor:
      # Enable creation of PodMonitor resource that scrapes vLLM metrics endpoint.
      enabled: true
      labels: {}
```

If you want to manually create the PodMonitor please take a look at the KubeAI helm chart [vLLM PodMonitor template](https://github.com/substratusai/kubeai/blob/main/charts/kubeai/templates/vllm-pod-monitor.yaml).


## Deploying Prometheus Operator
The Prometheus Operator is a Kubernetes operator that manages Prometheus and its related components.
The Prometheus Stack includes Grafana and Prometheus.

Add Prometheus Helm repo:
```sh
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
```

Install the Prometheus Stack and ensure PodMonitor without special labels works:
```sh
helm install prometheus prometheus-community/kube-prometheus-stack -f - <<EOF
prometheus:
  prometheusSpec:
    podMonitorSelectorNilUsesHelmValues: false
    ruleSelectorNilUsesHelmValues: false
    serviceMonitorSelectorNilUsesHelmValues: false
    probeSelectorNilUsesHelmValues: false
EOF
```

## Enable KubeAI vLLM PodMonitor

Install the KubeAI provided PodMonitor:

```sh
helm upgrade --reuse-values --install kubeai kubeai/kubeai \
  --set metrics.prometheusOperator.vLLMPodMonitor.enabled=true
```

## Importing the vLLM Grafana Dashboard

Now you can configure a port forward to the Grafana service:
```
kubectl port-forward svc/prometheus-grafana 8081:80
```

Open your browswer at [http://localhost:8081](http://localhost:8081) and log in with the default credentials (admin/prom-operator).

You can get the credential if that doesn't work:
```sh
kubectl get secret prometheus-grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo
```

You can import the example vLLM dashboard in the KubeAI repo at [examples/observability/vllm-grafana-dashboard.json](https://github.com/substratusai/kubeai/blob/main/examples/observability/vllm-grafana-dashboard.json).