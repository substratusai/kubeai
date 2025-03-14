#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

echo "Installing test model..."
kubectl apply -f $REPO_DIR/manifests/models/opt-125m-cpu.yaml

echo "Setting up Python environment..."
python -m venv $TEST_DIR/venv
source $TEST_DIR/venv/bin/activate

which pip
pip install -r $TEST_DIR/requirements.txt

echo "Running tests..."
pytest $TEST_DIR/test.py

echo "Test completed successfully!"
