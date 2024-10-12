#!/bin/bash

set -euxo pipefail

huggingface-cli download --local-dir $MODEL_DIR $MODEL_REPO
rm -rf $MODEL_DIR/.cache
