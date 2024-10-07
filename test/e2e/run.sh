#!/bin/bash

set -euxo pipefail

# run.sh <testcase> [skaffold flags]

testcase=$1
shift # All additional arguments are passed to skaffold.
skaffold_flags=$@

export REPO_DIR=$(git rev-parse --show-toplevel)
export TEST_DIR=$REPO_DIR/test/e2e/$testcase
export PATH=$REPO_DIR/bin:$PATH
export DOCKER_BUILDKIT=1

mkdir -p $REPO_DIR/tmp

# Function to handle errors
error_handler() {
    echo "--- FAIL ---"
    exit 1
}
trap 'error_handler' ERR

skaffold_pid=9999999999999999
cleanup() {
    echo "Running test framework cleanup..."
    if [ $skaffold_pid != 9999999999999999 ]; then
        kill $skaffold_pid
        wait $skaffold_pid
    fi
    skaffold delete -f $REPO_DIR/skaffold.yaml
}
trap cleanup EXIT

# Assert that PATH is configured correctly.
if [ "$(which skaffold)" != "$REPO_DIR/bin/skaffold" ]; then
    echo "Tools are not set up correctly. Probably need to run via Makefile."
    exit 1
fi

skaffold run -f $REPO_DIR/skaffold.yaml --tail --port-forward $skaffold_flags &
skaffold_pid=$!

# Wait for port-forward to be ready.
source $REPO_DIR/test/e2e/common.sh
retry 600 curl http://localhost:8000/openai/v1/models

$REPO_DIR/test/e2e/$testcase/test.sh

echo "--- PASS ---"