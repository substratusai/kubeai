#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

kubectl apply -f $REPO_DIR/manifests/models/opt-125m-cpu.yaml

python -m venv $TEST_DIR/venv

cleanup() {
    echo "Running openai-python-client test case cleanup..."
    kubectl delete -f $REPO_DIR/manifests/models/opt-125m-cpu.yaml
    deactivate
}
trap cleanup EXIT

source $TEST_DIR/venv/bin/activate

which pip
pip install -r $TEST_DIR/requirements.txt

# Wait for models to sync.
sleep 3

pytest $TEST_DIR/test.py
