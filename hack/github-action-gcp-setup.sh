#!/usr/bin/env bash

# Configure the permissions for github actions

set -xe

export CI_PROJECT=substratus-integration-tests
gcloud config set project ${CI_PROJECT}


test_prefix=substratus-lingo-test
requests_topic=${test_prefix}-requests
responses_topic=${test_prefix}-responses
requests_subscription=${test_prefix}-requests-sub
responses_subscription=${test_prefix}-responses-sub

# Create pubsub topics
if ! gcloud pubsub topics list | grep -q $requests_topic; then
    gcloud pubsub topics create $requests_topic
fi
if ! gcloud pubsub topics list | grep -q $responses_topic; then
    gcloud pubsub topics create $responses_topic
fi

# Create pubsub subscriptions
if ! gcloud pubsub subscriptions list | grep -q $requests_subscription; then
    gcloud pubsub subscriptions create $requests_subscription --topic=$requests_topic
fi
if ! gcloud pubsub subscriptions list | grep -q $responses_subscription; then
    gcloud pubsub subscriptions create $responses_subscription --topic=$responses_topic
fi


# Configure minimal permissions so GHA can publish and subscribe
export WORKLOAD_IDENTITY_POOL_ID=$(gcloud iam workload-identity-pools list \
    --format="value(name)" --project ${CI_PROJECT} --location=global)
export REPO="substratusai/lingo"
gcloud projects add-iam-policy-binding ${CI_PROJECT} \
    --role=roles/pubsub.publisher \
    --member="principalSet://iam.googleapis.com/${WORKLOAD_IDENTITY_POOL_ID}/attribute.repository/${REPO}"

gcloud projects add-iam-policy-binding ${CI_PROJECT} \
    --role=roles/pubsub.subscriber \
    --member="principalSet://iam.googleapis.com/${WORKLOAD_IDENTITY_POOL_ID}/attribute.repository/${REPO}"

