# Model Management

## Problem

This proposal is attempting to solve the following problems:

* Slow model loading on cold-start
* Storage of LoRA adapters generated from finetuning jobs
* Dynamic LoRA adapter injection

While working within the following constraints:

* KubeAI should be able to be installed with a single command
* KubeAI should not rely on external dependencies to provide basic functionality
* KubeAI should be able to run on a local machine
* KubeAI should be able to be run on clouds with heterogenous disk features (limited support for ReadOnlyMany)

## Solution

KubeAI should support configurable storage for both models and adapters.

These settings should be configurable at the `kind: Model` level because the optimum caching technique is relative to the size and frequency-of-use of the model. For example: GCP Hyperdisk ML is relatively expensive compared to other options, however it can be highly performant especially when it comes to loading large models.

```yaml
kind: Model
spec:
  storageProfile:
    model: HyperdiskML
    adapters: Bucket
```

Storage profiles should be defined at the KubeAI config.yaml level:

```yaml
# Set on Model objects if .spec.storageProfile is undefined.
defaultStorageProfiles:
  model: None # None is a special storage mode where each server pulls the model from source.
  adapters: Bucket

storageProfiles:
  # Bucket could be a default profile that KubeAI installs with.
  Bucket:
    object:
      url: s3://...
  # Admins can define their own profiles:
  EFS:
    volume:
      storageClass: aws-efs
      mode: SharedFilesystem
  HyperdiskML:
    volume:
      storageClass: gcp-hyperdisk
      mode: StaticCache
```

A storage profile might have a structure like:

```go
type StorageProfile struct {
    Volume *VolumeStorageProfile `json:"volume,omitempty"`
    Object *ObjectStorageProfile `json:"object,omitempty"`
}

type ObjectStorageProfile struct {
    // URL of bucket ("s3://...")
    URL string `json:"url"`
}

type VolumeStorageProfile struct {
    // StorageClassName to use for Kubernetes Volume.
    StorageClassName string `json:"storageClassName"`

    // Mode is either StaticCache or SharedFilesystem.
    Mode VolumeStorageProfileMode `json:"mode"`
}
```

### Object storage

This mode relies on access to a bucket.

To preserve local deployment as an option this bucket could be provided by a Helm-deployed instance of Minio (possible License issue?), OR [SeaweedFS](https://github.com/seaweedfs/seaweedfs) (mature/secure?).

![Bucket Mode](./diagrams/model-mgmt-buckets.excalidraw.png)

### Volume storage

This mode relies on Kubernetes PersistentVolumes for storage.

Two modes could be supported:

* `StaticCache`
  - How it works: mounted as ReadWriteOnce to initialize, then ReadOnlyMany for serving
  - Examples: GCP Hyperdisk ML
  - NOTEs:
    - Only supported for models, NOT adapters!
    - Typically provisioned via the k8s API
* `SharedFilesystem`
  - How it works: always mounted as ReadWriteMany
  - Examples:  - NFS, EFS, GCP Filestore
  - NOTEs:
    - Works for models and adapaters.
    - Typically provisioned outside of the k8s API

![Volume Mode](./diagrams/model-mgmt-volumes.excalidraw.png)

## Relevant Reading

* [KServe Cache Proposal](https://docs.google.com/document/d/1nao8Ws3tonO2zNAzdmXTYa0hECZNoP2SV_z9Zg0PzLA)
