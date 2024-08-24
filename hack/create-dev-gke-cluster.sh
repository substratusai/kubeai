#!/bin/bash

cluster_name="kubeai-dev-1"

gcloud container clusters create $cluster_name \
	--zone us-central1-a \
	--node-locations us-central1-a --num-nodes 1 --machine-type e2-medium

gcloud container node-pools create n2s4 \
	--cluster=$cluster_name \
	--zone us-central1-a \
	--machine-type=n2-standard-4 \
	--enable-autoscaling \
	--num-nodes=0 \
	--min-nodes=0 \
	--max-nodes=3

gcloud container node-pools create n2s8 \
	--cluster=$cluster_name \
	--zone us-central1-a \
	--machine-type=n2-standard-8 \
	--enable-autoscaling \
	--num-nodes=0 \
	--min-nodes=0 \
	--max-nodes=3

gcloud container node-pools create n2s16 \
	--cluster=$cluster_name \
	--zone us-central1-a \
	--machine-type=n2-standard-16 \
	--enable-autoscaling \
	--num-nodes=0 \
	--min-nodes=0 \
	--max-nodes=3

gcloud container node-pools create g2s8 \
	--cluster=$cluster_name \
	--zone us-central1-a \
	--accelerator=type=nvidia-l4,count=1,gpu-driver-version=default \
	--machine-type=g2-standard-8 \
	--enable-autoscaling \
	--num-nodes=0 \
	--min-nodes=0 \
	--max-nodes=3

