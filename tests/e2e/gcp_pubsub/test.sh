#!/usr/bin/env bash

set -xe

# NOTE: The keyfile created by hack/create-test-pubsub-keyfile.sh defaults to:
# GOOGLE_APPLICATION_CREDENTIALS=/tmp/lingo-test-pubsub-client.keyfile.json
if [ -z "$GOOGLE_APPLICATION_CREDENTIALS" ]; then
    echo "GOOGLE_APPLICATION_CREDENTIALS not set. Exiting."
    exit 1
fi

if [ -z "$PROJECT_ID" ]; then
    echo "PROJECT_ID not set. Exiting."
    exit 1
fi

random_id=$(openssl rand -hex 2)
test_prefix=lingo-test-${random_id}
requests_topic=${test_prefix}-requests
responses_topic=${test_prefix}-responses
requests_subscription=${test_prefix}-requests-sub
responses_subscription=${test_prefix}-responses-sub

function cleanup() {
    kubectl logs -l app=lingo
    if [ "$CLEANUP_PUBSUB" != "false" ]; then
        if gcloud pubsub subscriptions list | grep -q $requests_subscription; then
            gcloud pubsub subscriptions delete $requests_subscription --quiet
        fi
        if gcloud pubsub subscriptions list | grep -q $responses_subscription; then
            gcloud pubsub subscriptions delete $responses_subscription --quiet
        fi
        if gcloud pubsub topics list | grep -q $requests_topic; then
            gcloud pubsub topics delete $requests_topic --quiet
        fi
        if gcloud pubsub topics list | grep -q $responses_topic; then
            gcloud pubsub topics delete $responses_topic --quiet
        fi
    fi
}

trap cleanup EXIT

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

# Configure Lingo to use the pubsub topics and subscriptions.
messenger_urls="gcppubsub://projects/$PROJECT_ID/subscriptions/$requests_subscription|gcppubsub://projects/$PROJECT_ID/topics/$responses_topic|10"
configmap_patch=$(cat <<EOF
{"data": {"GOOGLE_APPLICATION_CREDENTIALS": "/secrets/gcp-keyfile.json", "MESSENGER_URLS": "$messenger_urls"}}
EOF
)

kubectl patch configmap lingo-env --patch "$configmap_patch"
kubectl create secret generic lingo-secrets --from-file=gcp-keyfile.json=$GOOGLE_APPLICATION_CREDENTIALS -oyaml --dry-run=client | kubectl apply -f -
kubectl delete pods -l app=lingo
# Wait for pods to become ready again.
kubectl wait --for=condition=ready --timeout=60s pod -l app=lingo

function test_message() {
    # Send a request to the requests topic and expect a response on the responses topic.
    meta_msg_id=$(date +%s)
    msg=$(cat <<EOF
{"path":"/v1/embeddings", "metadata":{"test-id":"$meta_msg_id"}, "body": {"model": "text-embedding-ada-002", "input": "Lingo rocks!"}}
EOF
)
published_msg_id=$(gcloud pubsub topics publish $requests_topic \
    --message="$msg" \
    --format=json | jq -r '.messageIds[0]'
)
expected_msg=$(cat <<EOF
"metadata":{"test-id":"$meta_msg_id"},"status_code":200
EOF
)

    # Wait for the response message to be published to the responses topic.
    msg_path="/tmp/substratus-test-pubsub-message-$meta_msg_id.txt"
    set +e
    for i in {1..120}; do
        gcloud pubsub subscriptions pull $responses_subscription --auto-ack --filter="message.attributes.request_message_id = \"$published_msg_id\"" > $msg_path
        if [ -s $msg_path ]; then
            echo "Received response message"
            break
        fi
        sleep 10
    done
    set -e
    cat $msg_path | grep -q "$expected_msg"
    rm $msg_path
}

# Test that Lingo processes a message.

test_message

# Test that Lingo restarts the subscription if an error occurs without Lingo itself restarting
# and goes on to process a message.

gcloud pubsub subscriptions delete $requests_subscription
sleep 60 
kubectl logs -l app=lingo --tail=-1 | grep -q 'recreating requests subscription'
gcloud pubsub subscriptions create $requests_subscription --topic=$requests_topic

test_message

# Make sure no restarts occurred.
lingo_restart_count=$(kubectl get pods -l app=lingo -o jsonpath='{.items[*].status.containerStatuses[*].restartCount}')
if [ "$lingo_restart_count" -ne 0 ]; then
    echo "Expected 0 restarts, got $lingo_restart_count"
    exit 1
fi