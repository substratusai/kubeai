#!/usr/bin/env bash

set -e
set -u
set -x

PROJECT_ID=${PROJECT_ID:=$(gcloud config get project)}
REGION=${REGION:-us-central1}
ZONE=${ZONE:=${REGION}-a}
LOCATION=${LOCATION:-"${REGION}"}
L4_LOCATIONS=$(gcloud compute accelerator-types list | grep L4 | grep ${REGION} | grep -v Workstation | awk '{ print $2 }' | tr '\n' ',' | sed 's/.$//')

# Enable required services.
gcloud services enable container.googleapis.com

export CLUSTER_NAME=${CLUSTER_NAME:-substratus}
if ! gcloud container clusters describe ${CLUSTER_NAME} --location ${LOCATION} -q >/dev/null; then
gcloud container clusters create ${CLUSTER_NAME} --location ${LOCATION} \
  --machine-type e2-standard-2 --num-nodes 1 --min-nodes 1 --max-nodes 5 \
  --autoscaling-profile optimize-utilization --enable-autoscaling \
  --node-locations ${ZONE} --workload-pool ${PROJECT_ID}.svc.id.goog \
  --enable-image-streaming --enable-shielded-nodes --shielded-secure-boot \
  --shielded-integrity-monitoring \
  --addons GcsFuseCsiDriver
fi

if ! gcloud container node-pools describe g2-standard-8 --cluster ${CLUSTER_NAME} --location ${LOCATION} -q >/dev/null; then
nodepool_args=(--spot --enable-autoscaling --enable-image-streaming
  --num-nodes=0 --min-nodes=0 --max-nodes=3 --cluster ${CLUSTER_NAME}
  --node-locations "${L4_LOCATIONS}" --location ${LOCATION} --async)

gcloud container node-pools create g2-standard-8 \
  --accelerator type=nvidia-l4,count=1,gpu-driver-version=latest \
  --machine-type g2-standard-8 \
  "${nodepool_args[@]}"
fi

# Install nvidia driver
kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/container-engine-accelerators/master/nvidia-driver-installer/cos/daemonset-preloaded-latest.yaml

