#!/bin/bash

source $REPO_ROOT/test/e2e/common.sh

kubectl apply -f $REPO_ROOT/manifests/models/opt-125m-cpu.yaml

python -m venv $TEST_CASE/venv

cleanup() {
    echo "Running test case cleanup..."
    deactivate
}
trap cleanup EXIT

source $TEST_CASE/venv/bin/activate

which pip
pip install -r $TEST_CASE/requirements.txt

pytest $TEST_CASE/test.py
