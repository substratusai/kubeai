#!/usr/bin/env bash

set -xe

OPENAI_REQUEST_COUNT=${OPENAI_REQUEST_COUNT:-60}
OPENAI_EXPECTED_REPLICAS=${OPENAI_EXPECTED_REPLICAS:-1}
OPENAI_LINGO_REPLICAS=${OPENAI_LINGO_REPLICAS:-3}

host=127.0.0.1
port=30080
base_url="http://$host:$port/v1"

script_dir=$(dirname "$0")
venv_dir=$script_dir/.venv
echo "Running test in $script_dir"

kubectl patch deployment lingo --patch "{\"spec\": {\"replicas\": $OPENAI_LINGO_REPLICAS}}"
kubectl wait --for=condition=available --timeout=30s deployment/lingo

replicas=$(kubectl get deployment stapi-minilm-l6-v2 -o jsonpath='{.spec.replicas}')
if [ "$replicas" -ne 0 ]; then
  echo "Expected 0 replica before sending requests, got $replicas"
  exit 1
fi


if [ ! -d "$venv_dir" ]; then
  python3 -m venv "$venv_dir"
fi
source "$venv_dir/bin/activate"
pip3 install openai==1.2.3

# Send 60 requests in parallel to stapi backend using openai python client and threading
python3 $script_dir/test_openai_embedding.py \
  --requests ${OPENAI_REQUEST_COUNT} --timeout 300 --base-url "${base_url}" \
  --model text-embedding-ada-002

# Ensure replicas has been scaled up to 1 after sending 60 requests
replicas=$(kubectl get deployment stapi-minilm-l6-v2 -o jsonpath='{.spec.replicas}')
if [ "$replicas" -ge "${OPENAI_EXPECTED_REPLICAS}" ]; then
  echo "Test passed: Expected ${OPENAI_EXPECTED_REPLICAS} or more replicas and got ${replicas} after sending requests ${OPENAI_REQUEST_COUNT} requests"
  else
  echo "Test failed: Expected ${OPENAI_EXPECTED_REPLICAS} or more replicas after sending requests ${OPENAI_REQUEST_COUNT} requests, got ${replicas}"
  exit 1
fi

# Verify that leader election works by forcing a 120 second apiserver outage
docker exec ${KIND_NODE} iptables -I INPUT -p tcp --dport 6443 -j DROP
sleep 120
docker exec ${KIND_NODE} iptables -D INPUT -p tcp --dport 6443 -j DROP
echo "Waiting for K8s to recover from apiserver outage"
sleep 30
until kubectl get deployment stapi-minilm-l6-v2; do
  sleep 1
done


checks=$((OPENAI_REQUEST_COUNT / 2))
echo "Waiting for deployment to scale down back to 0 within ~2 minutes"
for i in $(seq 1 ${checks}); do
  if [ "${i}" -eq "${checks}" ]; then
    echo "Test failed: Expected 0 replica after not having requests for more than 1 minute, got $replicas"
    kubectl logs -l app=lingo --tail=-1
    exit 1
  fi
  replicas=$(kubectl get deployment stapi-minilm-l6-v2 -o jsonpath='{.spec.replicas}')
  if [ "$replicas" -eq 0 ]; then
    echo "Test passed: Expected 0 replica after not having requests for more than 1 minute"
    break
  fi
  sleep 6
done