FROM ollama/ollama:latest

# Model to be downloaded.
ARG MODEL_URL

# MODEL_URL is a required argument.
RUN test -n "${MODEL_URL}"

# Set the model to be downloaded.
ENV MODEL_URL=${MODEL_URL}

COPY ./download.sh /download.sh
RUN ./download.sh