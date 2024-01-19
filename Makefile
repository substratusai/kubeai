ENVTEST_K8S_VERSION = 1.27.1

.PHONY: test
test: test-unit test-race test-integration test-e2e

.PHONY: test-unit
test-unit:
	go test -timeout=5m -mod=readonly -race ./pkg/...

.PHONY: test-integration
test-integration: envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./tests/integration -v

.PHONY: test-e2e
test-e2e:
	./tests/e2e/test.sh

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)
ENVTEST ?= $(LOCALBIN)/setup-envtest

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

GOLANGCI ?= $(LOCALBIN)/golangci-lint

.PHONY: golangci
golangci: $(GOLANGCI) ## Download golangci-lint locally if necessary.
$(GOLANGCI): $(LOCALBIN)
	test -s $(LOCALBIN)golangci-lint || GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2


.PHONY: lint
lint: golangci
	golangci-lint run ./... --timeout 5m

GOLANGCI ?= $(LOCALBIN)/golangci-lint

.PHONY: golangci
golangci: $(GOLANGCI) ## Download golangci-lint locally if necessary.
$(GOLANGCI): $(LOCALBIN)
	test -s $(LOCALBIN)golangci-lint || GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2

GCI ?= $(LOCALBIN)/gci

.PHONY: formatter
formatter: $(GCI) ## Download gci locally if necessary.
$(GCI): $(LOCALBIN)
	test -s $(LOCALBIN)gci || GOBIN=$(LOCALBIN) go install github.com/daixiang0/gci@v0.12.1

.PHONY: format
format: formatter
	find . -name '*.go' -type f -not -path "./vendor*" -not -path "*.git*"  | xargs gci write --skip-generated -s standard -s default -s "prefix(*.k8s.io)" -s "prefix(github.com/substratusai/lingo)" --custom-order