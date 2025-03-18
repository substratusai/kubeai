# KubeAI Development Guide

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
- E2E tests: `make test-e2e-*` (various test suites)
  * Must be run with an active `kind` cluster (Run `kind create cluster` if `kubectl config current-context` does not report a cluster as existing).

## Code Style
- Format: `make fmt` (standard Go formatting)
- Lint: `make lint` (golangci-lint v1.59.1)
- Vet: `make vet` (standard Go vetting)

## Conventions
- Standard Go project layout (cmd/, internal/, api/, test/)
- Table-driven tests with descriptive names
- Use testify for assertions
- Integration tests use require.EventuallyWithT for async verification
- Follow Kubernetes controller patterns (kubebuilder / controller-runtime)