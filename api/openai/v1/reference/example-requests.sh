#!/bin/bash

set -x

if [ -z "$OPENAI_API_BASE" ]; then
    echo "Error: OPENAI_API_BASE environment variable is not set"
    exit 1
fi

echo "Configured models:"
[ ! -z "$COMPLETIONS_MODEL" ] && echo "- Completions: $COMPLETIONS_MODEL"
[ ! -z "$CHAT_MODEL" ] && echo "- Chat: $CHAT_MODEL"
[ ! -z "$EMBEDDINGS_MODEL" ] && echo "- Embeddings: $EMBEDDINGS_MODEL"

if [ -z "$COMPLETIONS_MODEL" ] && [ -z "$CHAT_MODEL" ] && [ -z "$EMBEDDINGS_MODEL" ]; then
    echo "Error: No models configured (COMPLETIONS_MODEL, CHAT_MODEL, or EMBEDDINGS_MODEL)"
    exit 1
fi

echo -e "\nMaking example API requests..."

if [ ! -z "$COMPLETIONS_MODEL" ]; then
    # Basic completion request
    echo -e "\n1. Basic completion request:"
    curl -s "${OPENAI_API_BASE}/v1/completions" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $OPENAI_API_KEY" \
      -d '{
        "model": "'$COMPLETIONS_MODEL'",
        "prompt": "Say this is a test",
        "max_tokens": 7,
        "temperature": 0
      }' | jq .

    # Streaming completion request
    echo -e "\n\n2. Streaming completion request:"
    curl -s "${OPENAI_API_BASE}/v1/completions" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $OPENAI_API_KEY" \
      -d '{
        "model": "'$COMPLETIONS_MODEL'",
        "prompt": "Say this is a test",
        "max_tokens": 7,
        "temperature": 0,
        "stream": true
      }'
fi

if [ ! -z "$CHAT_MODEL" ]; then
    # Basic chat completion request
    echo -e "\n\n3. Basic chat completion request:"
    curl -s "${OPENAI_API_BASE}/v1/chat/completions" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $OPENAI_API_KEY" \
      -d '{
        "model": "'$CHAT_MODEL'",
        "messages": [
          {
            "role": "system",
            "content": "You are a helpful assistant."
          },
          {
            "role": "user",
            "content": "Hello!"
          }
        ]
      }' | jq .

    # Streaming chat completion request
    echo -e "\n\n4. Streaming chat completion request:"
    curl -s "${OPENAI_API_BASE}/v1/chat/completions" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $OPENAI_API_KEY" \
      -d '{
        "model": "'$CHAT_MODEL'",
        "messages": [
          {
            "role": "system",
            "content": "You are a helpful assistant."
          },
          {
            "role": "user",
            "content": "Hello!"
          }
        ],
        "stream": true
      }'
fi

if [ ! -z "$EMBEDDINGS_MODEL" ]; then
    # Embeddings request
    echo -e "\n\n5. Embeddings request:"
    curl -s "${OPENAI_API_BASE}/v1/embeddings" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $OPENAI_API_KEY" \
      -d '{
        "input": "The food was delicious and the waiter...",
        "model": "'$EMBEDDINGS_MODEL'",
        "encoding_format": "float"
      }' | jq .
fi
