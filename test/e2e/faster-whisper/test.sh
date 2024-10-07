#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

model=faster-whisper-medium-en-cpu

cleanup() {
    echo "Running faster-whisper test case cleanup..."
    kubectl delete -f $REPO_DIR/manifests/models/$model.yaml
    deactivate
}
trap cleanup EXIT

kubectl apply -f $REPO_DIR/manifests/models/$model.yaml

audio_file=$REPO_DIR/tmp/kubeai.mp4
curl -L -o $audio_file https://github.com/user-attachments/assets/711d1279-6af9-4c6c-a052-e59e7730b757

transcription_file=$REPO_DIR/tmp/transcription.json
curl http://localhost:8000/openai/v1/audio/transcriptions \
  -F "file=@$audio_file" \
  -F "language=en" \
  -F "model=$model" > $transcription_file
  
result_contains_kubernetes=$(cat $transcription_file | jq '.text | ascii_downcase | contains("kubernetes")')
if [ "$result_contains_kubernetes" = "true" ]; then
  echo "The transcript contains 'kubernetes'."
else
  echo "The text does not contain 'kubernetes':"
  cat $transcription_file
  exit 1
fi
