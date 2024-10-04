#!/bin/bash

set -e
set -u
set -x

this_dir=$(dirname "$0")

kubectl create configmap dev-load --from-file $this_dir/k6.js --dry-run=client -oyaml | kubectl apply -f -

kubectl create -f $this_dir/pod.yaml