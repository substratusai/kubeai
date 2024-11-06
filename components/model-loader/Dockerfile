FROM python:3.10-slim

# Set environment variables
ENV PYTHONUNBUFFERED=1

# Set the working directory in the container
WORKDIR /loaders

# Install Hugging Face CLI tool and other necessary dependencies
RUN pip install --no-cache-dir huggingface_hub

RUN apt-get update && apt-get install -y git && rm -rf /var/lib/apt/lists/*

COPY huggingface.sh /loaders/huggingface.sh
RUN chmod +x /loaders/huggingface.sh

ENV PATH="/loaders:${PATH}"
