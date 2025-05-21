#!/bin/bash

source $REPO_DIR/test/e2e/common.sh

model=bge-embed-text-cpu

apply_model $model

# Test embedding generation
response_file=$TMP_DIR/embedding.json
curl http://localhost:8000/openai/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "input": "Hello world",
    "model": "'$model'"
  }' > $response_file

# Verify response structure and content
embedding_length=$(cat $response_file | jq '.data[0].embedding | length')
if [ "$embedding_length" -ne 384 ]; then
  echo "Unexpected embedding dimension: got $embedding_length, expected 384"
  cat $response_file
  exit 1
fi

echo "Successfully generated embedding with $embedding_length dimensions"
