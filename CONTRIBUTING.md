# Contributing to KubeAI

Open an issue to discuss substantial changes before implementing them. Keep pull
requests focused, explain the behavior they change, and include the checks you ran.
Contact the maintainers privately to report security vulnerabilities.

## Development setup

Use the Go version declared in `go.mod`, Make, Docker, and a Kubernetes test
cluster. See the [development environment](docs/contributing/development-environment.md)
and [development guide](docs/contributing/development-guide.md) for installation
and integration test setup.

## Before opening a pull request

From the repository root, run the checks relevant to your change:

```sh
make fmt vet
make lint
make test-unit
```

The golangci-lint version is pinned in `.golangci-lint-version`, shared by Make
and CI. Fix findings using the existing `.golangci.yml` configuration.
For concurrency changes, also run `go test -race ./internal/... ./api/...`.
Run `make test-integration` for controller and Kubernetes API changes; the target
downloads envtest assets. For end-to-end changes, run the relevant
`make test-e2e TEST_CASE=<case>` against the documented test cluster.

For changes to the independent chat example, run `go test ./...` and
`go build ./...` from `examples/private-deep-chat`.

## Code and API changes

- Preserve compatibility of `api/k8s/v1` CRD fields, defaults, and validation.
  After changing API types or kubebuilder markers, run `make generate manifests`
  and include the generated files.
- Reconciliation must be idempotent and handle missing objects and conflicts.
  Cover scaling from and to zero, and avoid reconcile loops without backoff.
- In proxy and load balancer changes, check cancellation, retries, streamed
  responses, goroutine cleanup, and concurrent access.
- Document new Helm values in `values.yaml` and follow least privilege for RBAC.
  Run `helm dependency build charts/kubeai`, `helm lint charts/kubeai`, and
  `helm template kubeai charts/kubeai` for chart changes.
- Give tests explicit timeouts and cleanup. Prefer waiting for a condition over
  fixed sleeps.

## Workflows and dependency updates

Pin GitHub Actions to full commit SHAs. Grant token
permissions to the jobs that need them, disable persisted checkout credentials
unless later steps push to Git, and pass expression values to shell scripts
through environment variables.

CI runs golangci-lint, govulncheck for both Go modules, and zizmor. To check
workflow changes locally, run `actionlint` and `zizmor .github` after installing
those tools. Security checks also run weekly to catch newly disclosed issues.
Major dependency updates require manual review. See
[CI security and dependency automation](docs/contributing/ci-security.md) for
maintainer setup, required checks, CodeQL, and the Dependabot GitHub App.

CodeRabbit uses `.coderabbit.yaml` for English reviews with a relaxed profile,
path-specific guidance, and generated-file exclusions. Maintainers must install
the CodeRabbit GitHub App on the repository for automatic reviews to run.
