#!/bin/bash

set -eu

# Exit if the model URL is not set.
: "$MODEL_URL"

# Check if the model URL is in the correct format - matching
# the format used in .spec.url in the Model Custom Resource.
if [[ $MODEL_URL != ollama://* ]] ;
then
  echo "MODEL_URL must use the \"ollama://<model-name>\" format"
  exit 1
fi

ollama_model_name=${MODEL_URL#ollama://}

# Run Ollama server in the background.
/bin/ollama serve &
pid=$!

# TODO: Wait for the server to start using something more exact.
sleep 5

/bin/ollama pull $ollama_model_name

# Send SIGTERM to the server to allow it to gracefully exit.
kill -SIGTERM "$pid"

# Wait for the server to exit.
wait "$pid"
