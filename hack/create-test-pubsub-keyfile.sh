#!/bin/bash

set -xe

SA_NAME=${SA_NAME:-lingo-test-pubsub-client}
GCP_PUBSUB_KEYFILE_PATH=${GCP_PUBSUB_KEYFILE_PATH:-/tmp/${SA_NAME}.keyfile.json}

# Create SA if it doesn't exist
if ! gcloud iam service-accounts list | grep -q $SA_NAME; then
    gcloud iam service-accounts create $SA_NAME
fi

# Get the email of the SA
sa_email=$(gcloud iam service-accounts list --format="value(email)" --filter="name:$SA_NAME")

# Add PubSub roles
gcloud projects add-iam-policy-binding $PROJECT_ID --member=serviceAccount:$sa_email --role=roles/pubsub.publisher
gcloud projects add-iam-policy-binding $PROJECT_ID --member=serviceAccount:$sa_email --role=roles/pubsub.subscriber

# Create a keyfile for the SA
if [[ ! -e "$GCP_PUBSUB_KEYFILE_PATH" ]]; then
    gcloud iam service-accounts keys create --iam-account=$sa_email $GCP_PUBSUB_KEYFILE_PATH
fi

