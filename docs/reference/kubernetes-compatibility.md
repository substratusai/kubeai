# Kubernetes compatibility

This source revision targets Kubernetes **1.37**, with Kubernetes Go clients
at **v0.37.1** and controller-runtime **v0.25.2**. The latest stable Kubernetes
patch checked for this update is **1.37.1** (2026-10-20).

## Versions and verification

| Component | Version | Purpose |
| --- | --- | --- |
| `k8s.io/api`, `apimachinery`, `client-go`, `apiextensions-apiserver` | v0.37.1 | One consistent Kubernetes client dependency set |
| controller-runtime | v0.25.2 | Runtime series tested upstream with Kubernetes clients v0.37 |
| controller-gen | v0.23.0 | DeepCopy and CRD generation |
| setup-envtest | release-0.25 | Local integration control plane |
| envtest API server | 1.37.0 | Published test assets used by CI |
| kind | v0.33.0 | End-to-end cluster creation |
| kind node image | 1.37.0, pinned by digest | Published image used by CI |

The 1.37.1 client patch does not imply that a `kindest/node:v1.37.1` image or
an envtest 1.37.1 archive exists. At the time of this update neither is published.
CI therefore uses the official 1.37.0 artifacts explicitly, rather than relying
on a changing default image. An externally installed 1.37.1 API server can also
be tested using `KUBEBUILDER_ASSETS`.

The integration suite installs the generated Model CRD into a real API server.
`TestKubernetesModelLifecycle` checks server-side apply and field ownership,
the `scale` and `status` subresources, and CEL rejection of invalid replica
bounds. The remaining suites cover reconciliation, scaling, caching and proxy
behavior. Envtest does not run kubelets, real GPU workloads or cloud CSI drivers;
the engine end-to-end tests exercise those deployment paths separately.

## Updating an existing cluster

1. Follow your Kubernetes distribution's supported upgrade sequence, including
   intermediate minor versions and version-skew requirements.
2. Check the versions of your GPU device plugin, container runtime, CSI drivers
   and cluster autoscaler against the selected Kubernetes version. For EKS,
   use the [updated EKS guide](../installation/eks.md), including AL2023 AMIs
   and a compatible Karpenter release.
3. Upgrade KubeAI and apply the Model CRD from the same release. The CRD remains
   `apiextensions.k8s.io/v1`, with stored Models served as `kubeai.org/v1`;
   this update does not change the Model schema or require data conversion.
   Helm does not automatically upgrade resources in a chart's `crds/` directory;
   this chart manages its CRD under `templates/crds` when `crds.enabled=true`.
   If you manage CRDs separately, apply `manifests/crds/kubeai.org_models.yaml`.
4. Verify a Model can become ready, scale from zero and serve inference before
   rolling out the rest of your workloads.

This dependency update does not add a Helm version constraint rejecting older
clusters. Compatibility with another Kubernetes minor must be checked with
that cluster's API server and infrastructure components.

## Upstream references

- [Kubernetes releases](https://kubernetes.io/releases/)
- [Kubernetes version-skew policy](https://kubernetes.io/releases/version-skew-policy/)
- [controller-runtime compatibility](https://github.com/kubernetes-sigs/controller-runtime#compatibility)
- [kind v0.33.0 published node images](https://github.com/kubernetes-sigs/kind/releases/tag/v0.33.0)
