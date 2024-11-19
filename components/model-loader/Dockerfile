FROM alpine:3.20

# Python
ENV PYTHONUNBUFFERED=1
RUN apk add --no-cache python3 py3-pip pipx bash && rm -rf ~/.cache/* /usr/local/share/man /tmp/*     
# Location where pipx installs executables:
ENV PATH="/root/.local/bin:$PATH"

# Hugging Face ("hf://")
RUN pipx install huggingface_hub
RUN huggingface-cli version

# S3 ("s3://")
RUN pipx install awscli
RUN aws --version