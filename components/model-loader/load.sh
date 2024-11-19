#!/bin/bash

set -euxo pipefail

url=$1
dir=$2

if [[ $url == "hf://"* ]]; then
    repo=${url#hf://}
    huggingface-cli download --local-dir $dir $repo
    rm -rf $dir/.cache
elif [[ $url == "s3://"* ]]; then
    aws s3 sync $url $dir
else
    echo "Unsupported model url: $url"
    exit 1
fi
