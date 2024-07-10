ENVTEST_K8S_VERSION = 1.27.1

.PHONY: test
test: test-unit test-integration test-e2e

.PHONY: test-unit
test-unit:
	go test -timeout=5m -mod=readonly -race ./pkg/...

.PHONY: test-integration
test-integration: envtest
	go clean -testcache; KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./tests/integration -v

# Make sure to set:
# export PROJECT_ID=$(gcloud config get-value project) 
# export GCP_PUBSUB_KEYFILE_PATH=/tmp/lingo-test-pubsub-client.keyfile.json
.PHONY: test-e2e
test-e2e:
	TEST_CASE=gcp_pubsub ./tests/e2e/test.sh
	TEST_CASE=openai_client ./tests/e2e/test.sh

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)
ENVTEST ?= $(LOCALBIN)/setup-envtest

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

