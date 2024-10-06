#!/bin/bash

set -euxo pipefail

# run.sh <testcase> [skaffold flags]
testcase=$1
# All additional arguments are passed to skaffold.
shift
skaffold_flags=$@

# Function to handle errors
error_handler() {
    echo "FAIL"
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
}
trap cleanup EXIT

export REPO_ROOT=$(git rev-parse --show-toplevel)
export TEST_CASE=$REPO_ROOT/test/e2e/$testcase
export PATH=$REPO_ROOT/bin:$PATH
export DOCKER_BUILDKIT=1

mkdir -p $REPO_ROOT/tmp

# Assert that PATH is configured correctly.
if [ "$(which skaffold)" != "$REPO_ROOT/bin/skaffold" ]; then
    echo "Tools are not set up correctly. Probably need to run via Makefile."
    exit 1
fi

skaffold run -f $REPO_ROOT/skaffold.yaml --tail --port-forward $skaffold_flags &
skaffold_pid=$!

# Wait for port-forward to be ready.
source $REPO_ROOT/test/e2e/common.sh
retry 600 curl http://localhost:8000/openai/v1/models

$REPO_ROOT/test/e2e/$testcase/test.sh

echo "PASS"