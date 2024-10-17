# Cache models with GCP Filestore

KubeAI can manage model caches. GCP Filestore is supported as a pluggable backend store.

<br>
<img src="/diagrams/caching-shared-filesystem.excalidraw.png" width="90%"></img>

Follow the [GKE install guide](../installation/gke.md).

Ensure that the Filestore API is enabled.

```bash
gcloud services enable file.googleapis.com
```

Apply a Model with the cache profile set to `standard-filestore` (defined in the reference [GKE Helm values file](https://github.com/substratusai/kubeai/blob/main/charts/kubeai/values-gke.yaml)).

NOTE: If you already installed the models chart, you will need to edit you values file and run `helm upgrade`.

```bash
helm install kubeai-models $REPO_DIR/charts/models -f - <<EOF
catalog:
  opt-125m-cpu:
    enabled: true
    cacheProfile: standard-filestore
EOF
```

Wait for the Model to be fully cached.

```bash
kubectl wait --timeout 10m --for=jsonpath='{.status.cache.loaded}'=true model/opt-125m-cpu
```

This model will now be loaded from Filestore when it is served.

## Troubleshooting

### Filestore CSI Driver

Ensure that the Filestore CSI driver is enabled by checking for the existance of Kubernetes storage classes. If they are not found, follow the [GCP guide](https://cloud.google.com/filestore/docs/csi-driver#existing) for enabling the CSI driver.

```bash
kubectl get storageclass standard-rwx premium-rwx
```

### PersistentVolumeClaim

Check the PersistentVolumeClaim (that should be created by KubeAI).

```bash
kubectl describe pvc shared-model-cache-
```

### Model Loading Job

Check to see if there is an ongoing model loader Job.

```bash
kubectl get jobs
```