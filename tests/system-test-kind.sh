#!/usr/bin/env bash

set -e

kind create cluster --name=substratus-test
trap "kind delete cluster --name=substratus-test" EXIT

skaffold run

kubectl wait --for=condition=available --timeout=30s deployment/proxy-controller

kubectl port-forward svc/proxy-controller 8080:80 &

# need to wait for a bit for the port-forward to be ready
sleep 5

replicas=$(kubectl get deployment backend -o jsonpath='{.spec.replicas}')
if [ "$replicas" -ne 0 ]; then
  echo "Expected 0 replica before sending requests, got $replicas"
  exit 1
fi

echo "Sending 60 requests to model named backend"
for i in {1..60}; do
curl -s -o /dev/null http://localhost:8080/delay/10 \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Your text string goes here",
    "model": "backend"
  }' &
done

sleep 10

replicas=$(kubectl get deployment backend -o jsonpath='{.spec.replicas}')

if [ "$replicas" -ne 1 ]; then
  echo "Expected 1 replica after sending less than 100 requests, got $replicas"
  exit 1
fi
