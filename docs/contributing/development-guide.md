# KubeAI Development Guide

## Go Toolchain
- Go 1.27.2 or later is required.
- CI reads the Go version from `go.mod`. Keep its `go` directive and the
  builder images in both Dockerfiles aligned when updating Go.

## Kubernetes dependencies

- Kubernetes client libraries: `k8s.io/api`, `apimachinery`, `client-go` and
  `apiextensions-apiserver` v0.37.1.
- Controller runtime: v0.25.2. Keep its minor version aligned with the
  [upstream compatibility table](https://github.com/kubernetes-sigs/controller-runtime#compatibility).
- Code and CRD generator: controller-gen v0.23.0.
- Integration tests: envtest Kubernetes 1.37.0, using setup-envtest release-0.25.
- End-to-end tests: kind v0.33.0 with the published Kubernetes 1.37.0 image.

Update these together when changing Kubernetes minor versions. Regenerate the
CRD in `manifests/crds` and its Helm copy, and run the API lifecycle test as
well as the controller and autoscaling suites. See
[Kubernetes compatibility](../reference/kubernetes-compatibility.md).

## OpenAI API
- Types: See `./api/openai/v1/README.md`

## Build and Run Commands
- Build: `make build` (manager binary)
- Docker: `make docker-build`
- Run locally: `make run`
- Generate go code (for `./api/*`): `make generate`
- Generate manifests: `make manifests`

## Testing Commands
- Unit tests: `make test-unit`
  * Single unit test (does not work for integration tests): `go test -v ./path/to/package -run TestNamePattern`
- Integration tests: `make test-integration RUN=SpecificTestToRun`
- API compatibility smoke test: `make test-integration RUN=TestKubernetesModelLifecycle`
- Override envtest: `make test-integration ENVTEST_K8S_VERSION=1.37.0`.
  To use externally installed API server/etcd binaries, set `KUBEBUILDER_ASSETS`
  and invoke `go test ./test/integration` directly.
- If a local service uses port 8080, set `KUBEAI_TEST_METRICS_ADDR=127.0.0.1:18080`
  when running integration tests. Port 8000 must be free for the inference API.
- E2E tests: `make test-e2e-*` (various test suites)
  * Must be run with an active `kind` cluster; see the pinned image in the
    [development environment guide](development-environment.md).

## Code Style
- Format: `make fmt` (standard Go formatting)
- Lint: `make lint` (golangci-lint v2.14.0)
- Vet: `make vet` (standard Go vetting)

## Conventions
- Standard Go project layout (cmd/, internal/, api/, test/)
- Table-driven tests with descriptive names
- Use testify for assertions
- Integration tests use require.EventuallyWithT for async verification
- Follow Kubernetes controller patterns (kubebuilder / controller-runtime)
