# Model Storage

With "Large" in the name, caching is a critical part of serving LLMs.

The best caching technique may very depending on your environment:

* What cloud features are available?
* Is your cluster deployed in an air-gapped environment?

## A. Model embedded in container

**Status:** Supported

Building a model into a container image can provide a simple way to take advantage of image-related optimizations built into Kubernetes:

* Relaunching a model server on the same Node that it ran on before will [likely](https://kubernetes.io/docs/concepts/architecture/garbage-collection/#container-image-lifecycle) be able to reuse the previously pulled image.

* [Secondary boot disks on GKE](https://cloud.google.com/kubernetes-engine/docs/how-to/data-container-image-preloading) can be used to avoid needing to pull images.

* [Image streaming on GKE](https://cloud.google.com/blog/products/containers-kubernetes/introducing-container-image-streaming-in-gke) can allow for containers to startup before the entire image is present on the Node.

* Container images can be pre-installed on Nodes in air-gapped environments (example: [k3s airgap installation](https://docs.k3s.io/installation/airgap)).


**Guides:**

* [How to preload model container images](../how-to/preload-model-container-images.md)

## B. Model on shared filesystem (read-write-many)

**Status:** [Planned](https://github.com/substratusai/kubeai/blob/main/proposals/model-storage.md).

Examples: [AWS EFS](https://aws.amazon.com/efs/)

## C. Model on read-only-many disk

**Status:** [Planned](https://github.com/substratusai/kubeai/blob/main/proposals/model-storage.md).

Examples: [GCP Hyperdisk ML](https://cloud.google.com/compute/docs/disks/hyperdisks)
