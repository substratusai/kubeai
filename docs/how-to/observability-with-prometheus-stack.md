# Configure Observability with Prometheus Stack

The Prometheus Stack includes Grafana and Prometheus.

Add helm repo:
```sh
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
```


Install the Prometheus Stack with PodMonitor for scraping vLLM metrics:
```sh
helm install prometheus prometheus-community/kube-prometheus-stack -f - <<EOF
additionalPodMonitors:
  - name: vllm
    selector:
      matchLabels:
        app.kubernetes.io/name: vllm
    podMetricsEndpoints:
      - port: http
EOF
```

Now you can configure a port forward to the Grafana service:
```
kubectl port-forward svc/prometheus-grafana 8081:80
```

Open your browswer at [http://localhost:8081](http://localhost:8081) and log in with the default credentials (admin/prom-operator).

You can get the credential if that doesn't work:
```sh
kubectl get secret prometheus-grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo
```

You can use the example vLLM dashboard in the KubeAI repo at [examples/vllm-grafana-dashboard.json](https://github.com/substratusai/kubeai/blob/main/examples/vllm-grafana-dashboard.json).