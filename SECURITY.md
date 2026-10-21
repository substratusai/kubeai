# Security Policy

The KubeAI maintainers take the security of KubeAI, and of the clusters it runs in, seriously. Thank you for helping keep the project and its users safe.

This policy covers the [kubeai-project/kubeai](https://github.com/kubeai-project/kubeai) repository and the artifacts built from it. It explains which versions receive security fixes, how to report a vulnerability privately, what happens after you report, and what we do and do not consider a vulnerability.

> [!IMPORTANT]
> Please **do not** report security vulnerabilities through public GitHub issues, discussions, pull requests, or the KubeAI Slack channel. Use one of the private channels described below.

## Disclaimer

KubeAI is an open-source project, distributed under the [Apache License 2.0](https://github.com/kubeai-project/kubeai/blob/main/LICENSE) and maintained on a best-effort basis. It is provided on an "AS IS" basis, **without warranties or conditions of any kind**, either express or implied, as stated in the Disclaimer of Warranty and Limitation of Liability sections of the license.

This policy describes how the maintainers intend to handle security reports. It is not a contract, a service-level agreement, or a support commitment: the response times, fix targets, and supported versions described below are goals, not guarantees. In particular, there is no guarantee that KubeAI is free of vulnerabilities, that every reported issue will be fixed, or that fixes will be released within a given timeframe. You are responsible for evaluating whether KubeAI is suitable for your use and for securing your own deployments.

## Supported Versions

KubeAI is pre-1.0 and evolves quickly, so only the **latest release** is maintained. Security fixes are shipped in a new release, together with updated container images and Helm charts, and are not backported to earlier versions.

| Version                                                                    | Supported        |
| -------------------------------------------------------------------------- | ---------------- |
| [Latest release](https://github.com/kubeai-project/kubeai/releases/latest) | ✅                |
| Any earlier release                                                        | ❌ Not supported |

To receive security fixes, upgrade to the latest release.

## Reporting a Vulnerability

### Preferred: GitHub Private Vulnerability Reporting

1. Open the [Security tab](https://github.com/kubeai-project/kubeai/security) of this repository.
2. Click **Report a vulnerability**, or go directly to <https://github.com/kubeai-project/kubeai/security/advisories/new>.
3. Fill in the form with the information listed in [What to include](#what-to-include).

Your report is visible only to you and the repository maintainers. This channel lets us discuss the issue with you, work on a fix in a temporary private fork, request a CVE, and publish an advisory, all in one place.

### Alternative channel

If you cannot use GitHub, send a direct message to one of the [active maintainers](https://github.com/kubeai-project/kubeai#get-plugged-in) on the KubeAI Slack workspace and ask for a private channel. **Do not include any vulnerability details in that first message.**

### What to include

The more of the following you can provide, the faster we can triage your report:

- **Affected versions**: KubeAI release tag or commit SHA, and Helm chart versions.
- **Affected component**: model proxy (OpenAI-compatible API), model operator and `Model` handling, model loader and caching, messaging integration, Helm charts, container images, CI/release workflows, or documentation.
- **Environment**: Kubernetes version and distribution, inference engine (vLLM, Ollama, SGLang, llama.cpp, FasterWhisper, Infinity), model source scheme (`hf://`, `s3://`, `gs://`, `oss://`, `pvc://`, `ollama://`, `oci://`), and any non-default Helm values relevant to the issue (with secrets redacted).
- **Attacker preconditions**: what an attacker needs, for example network access to the KubeAI API, permission to create or update `Model` resources in a namespace, or control over a model repository.
- **Steps to reproduce**, ideally with a minimal proof of concept (manifests, HTTP requests, or a script).
- **Impact**: what an attacker can achieve, and your severity estimate (a CVSS vector is welcome but not required).
- **Suggested fix or mitigation**, if you have one.
- Whether you plan to publish or present your findings, and how you would like to be credited.

Please do not include real credentials, tokens, or personal data in your report.

## What to Expect

KubeAI is maintained by a small team. We aim to meet the following targets on a best-effort basis:

| Stage                                                                          | Target                                  |
| ------------------------------------------------------------------------------ | --------------------------------------- |
| Acknowledgement and initial assessment (validity, severity, affected versions) | Within 7 days of your report            |
| Release of a fix, for any severity                                             | Within 7 days of the initial assessment |

After we receive a report:

1. **Triage**: we confirm the issue, determine the affected versions, and assign a severity using CVSS. We may ask you for more details.
2. **Fix**: we develop the fix in a temporary private fork linked to the advisory. You are welcome to review or contribute to it.
3. **Release**: we publish a new release that includes the fix.
4. **Disclosure**: we publish a GitHub Security Advisory, request a CVE ID through GitHub, and reference the advisory in the release notes and in the [CHANGELOG](https://github.com/kubeai-project/kubeai/blob/main/CHANGELOG.md).
5. **Credit**: we credit reporters in the advisory, unless they prefer to remain anonymous.

We will keep you informed throughout the process. If we conclude that a report is not a security vulnerability, we will explain why; it may still be worth filing as a regular issue or hardening request.

### Coordinated disclosure

Please keep the details of a vulnerability private until a fix has been released or 90 days have passed since your report, whichever comes first. We will agree on the publication date with you. For complex issues we may ask for a short extension; if a vulnerability is being actively exploited, we will move faster and may disclose sooner.

## Scope

### In scope

Vulnerabilities in the code, configuration, and artifacts produced by this repository:

- **KubeAI manager**: the model proxy (OpenAI-compatible API, prefix-aware load balancing, request queueing and retries, autoscaling) and the model operator (reconciliation of `Model` resources into Pods, cache-loading Jobs, volumes, and LoRA adapters).
- **`Model` custom resource** (`models.kubeai.org`): validation, defaulting, and how its fields are translated into workloads.
- **Model loader** and model caching.
- **Messaging integration** (e.g., Kafka, Pub/Sub).
- **Helm charts** (`kubeai`, `models`), including default values, RBAC, and the model catalog, as published at <https://www.kubeai.org> and as OCI artifacts.
- **Container images and release artifacts** built from this repository, and the CI/CD workflows that produce them.
- **Documentation and examples** that, when followed as written, result in an insecure deployment.

Examples of issues we treat as vulnerabilities:

- A `Model` field being interpreted beyond its documented meaning, such as command, argument, or path injection through the model `url`, adapter definitions, or other `Model` fields (see [GHSA-324q-cwx9-7crr](https://github.com/kubeai-project/kubeai/security/advisories/GHSA-324q-cwx9-7crr) / CVE-2026-34940).
- KubeAI acting as a confused deputy: causing the controller to read, mount, or expose Secrets or other resources, or to act in namespaces, that the requesting principal cannot access directly.
- Leaks of credentials (e.g., Hugging Face tokens or object-storage credentials) into logs, events, metrics, `status` fields, or Pods that should not receive them.
- Cross-request data leakage in the proxy, such as a client receiving another client's response.
- Request smuggling, header injection, or server-side request forgery (SSRF) through the proxy or the model loader.
- Crashes or resource exhaustion of the KubeAI manager triggered by a single crafted request or resource.
- Insecure defaults in the Helm charts, such as unnecessarily broad RBAC permissions or container privileges.

### Out of scope

The following are generally **not** considered vulnerabilities in KubeAI:

- **Missing authentication on the OpenAI-compatible API.** This is by design: KubeAI does not validate API keys and expects access control to be enforced in front of it (see [Security Model and Hardening](#security-model-and-hardening)). The consequences of exposing the API to untrusted networks without such controls, such as unauthorized inference, scale-up of costly GPU workloads, or volumetric denial of service, are deployment issues.
- **Documented features used by authorized principals.** Anyone allowed to create or update `Model` resources can choose the container image, arguments, environment variables, and model source of the Pods that KubeAI creates. Doing so as documented is not a vulnerability, even though it allows running arbitrary code in model Pods. Making a field do something it is *not* documented to do is in scope (see above).
- **Model behavior**: prompt injection, jailbreaks, harmful, biased, or incorrect outputs, and training data extraction. These are properties of the models you deploy.
- **Untrusted model artifacts you choose to load**, such as malicious weights or tokenizers, or code executed through engine options like `--trust-remote-code`.
- **Vulnerabilities in third-party software**: inference engines (vLLM, Ollama, SGLang, llama.cpp, FasterWhisper, Infinity), Open WebUI and other subcharts, Kubernetes, and base images. Please report these upstream. If a KubeAI default, such as an image version pinned in the charts or model catalog, exposes users to an upstream vulnerability that is already public, a regular issue or pull request is fine.
- **Publicly known CVEs in dependencies** without demonstrated impact on KubeAI, and unverified output of automated scanners. Regular issues or pull requests are welcome for these.
- Attacks that require **cluster-admin, node-level access, or an already compromised cluster**.
- **Third-party services built on KubeAI**, such as managed or marketplace offerings. Please report those to the provider.
- Issues on the documentation website that do not affect the integrity of published charts or artifacts (e.g., missing security headers).

## Security Model and Hardening

The KubeAI controller holds permissions to create and manage workloads in the namespaces it watches, and the models it serves can be expensive to run and may process sensitive data. Securing a deployment is a shared responsibility. Operators should:

**Protect the API**

- Keep the `kubeai` Service reachable only from inside the cluster, and put an authenticating gateway or ingress, with rate limits and quotas, in front of it before exposing it to users. KubeAI ignores the API keys sent by OpenAI clients.
- Remember that KubeAI scales models from zero on demand: anyone who can reach the API can cause model Pods, and potentially GPU nodes, to start. Set `maxReplicas` on every `Model`.
- Use NetworkPolicies so that only intended clients can reach the KubeAI API, and only KubeAI can reach model server Pods. Inference engines usually expose their own unauthenticated HTTP APIs.
- If you enable the messaging integration, anyone who can publish to the request topics can submit inference requests. Secure your broker and topics accordingly.
- The `kubeai` chart can deploy Open WebUI as a convenience chat UI (used in the quickstart). Review its authentication settings before exposing it, or disable it if you do not need it.

**Control who can define models**

- Treat `create`, `update`, and `patch` on `models.kubeai.org` as equivalent to creating Pods in that namespace.
- In shared clusters, use admission policies (e.g., ValidatingAdmissionPolicy, Kyverno, or Gatekeeper) to restrict the images, model sources, and engine arguments allowed in `Model` resources.
- Keep the `watchNamespaces` setting (v0.24.0+) as narrow as possible. Watching all namespaces (`["*"]`) grants the controller cluster-wide permissions. Model names are also resolved across all watched namespaces: on a name collision the proxy prefers the operator's own namespace and then alphabetical order, so anyone who can create a `Model` in a watched namespace can influence where requests for that model name are routed. Only watch namespaces whose `Model` authors you trust equally.

**Trust your model sources**

- Loading a model means running code and data from its source. Use only repositories and registries you trust, pin revisions or digests where the source supports it, and enable `--trust-remote-code` (or equivalent options) only for code you have reviewed.

**Scope your credentials**

- Credentials for model sources (Hugging Face tokens, S3/GCS/OSS credentials) are made available to the Pods that download or serve models. Use read-only credentials limited to what KubeAI needs, and rotate them regularly.

**Stay up to date**

- Run the latest KubeAI release and keep inference engine images current. Watch this repository's releases (**Watch → Custom → Releases**) and review the [published security advisories](https://github.com/kubeai-project/kubeai/security/advisories).

## Good-Faith Research

We welcome security research conducted in good faith. When investigating KubeAI, please:

- test only against clusters and deployments that you own or are explicitly authorized to test;
- avoid privacy violations, data destruction, and service disruption;
- access no more data than needed to demonstrate the issue, and do not retain it;
- do not test against third-party services that run KubeAI without their permission.

Thank you for helping keep KubeAI and its users secure.
