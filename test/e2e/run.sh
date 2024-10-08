#!/bin/bash

set -euxo pipefail

# run.sh <testcase> [skaffold flags]

testcase=$1
shift # All additional arguments are passed to skaffold.
skaffold_flags=$@

export REPO_DIR=$(git rev-parse --show-toplevel)
export TEST_DIR=$REPO_DIR/test/e2e/$testcase
export TMP_DIR=$REPO_DIR/tmp
export PATH=$REPO_DIR/bin:$PATH
export DOCKER_BUILDKIT=1

mkdir -p $REPO_DIR/tmp

source $REPO_DIR/test/e2e/common.sh

skaffold_log_file=$TMP_DIR/skaffold.log

# Avoid using an unintended kubectl context.
expected_kubectl_context=${TEST_KUBECTL_CONTEXT:-kind-kind}
current_kubectl_context=$(kubectl config current-context)
if [ "$current_kubectl_context" != "$expected_kubectl_context" ]; then
    output "Current kubectl context is $current_kubectl_context, expected $expected_kubectl_context"
    output "Set TEST_KUBECTL_CONTEXT to override the expected context."
    exit 1
fi

# Function to handle errors
error_handler() {
    output "Tests failed. Printing logs..."
    if [ -f $skaffold_log_file ]; then
        output "--- Skaffold Logs ---"
        cat $skaffold_log_file
    fi
    output "--- Nodes ---"
    kubectl get nodes -owide
    output "--- Pods ---"
    kubectl get pods -owide
    output "--- Events ---"
    kubectl get events
    output "--- Models ---"
    kubectl get crds models.kubeai.org && kubectl get models -oyaml
    output "!!! FAIL !!!"
    exit 1
}
trap 'error_handler' ERR

skaffold_pid=9999999999999999
cleanup() {
    output "Running test framework cleanup..."
    if [ $skaffold_pid != 9999999999999999 ]; then
        kill $skaffold_pid
        wait $skaffold_pid
    fi
    skaffold delete -f $REPO_DIR/skaffold.yaml
}
trap cleanup EXIT

# Assert that PATH is configured correctly.
if [ "$(which skaffold)" != "$REPO_DIR/bin/skaffold" ]; then
    output "Tools are not set up correctly. Probably need to run via Makefile."
    exit 1
fi

skaffold run -f $REPO_DIR/skaffold.yaml --tail --port-forward > $skaffold_flags &
skaffold_pid=$!

output "Waiting for port-forward to be ready..."
retry 6 curl http://localhost:8000/openai/v1/models

$REPO_DIR/test/e2e/$testcase/test.sh

output "!!! PASS !!!"