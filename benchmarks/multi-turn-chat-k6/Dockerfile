FROM ubuntu:20.04

RUN apt-get update && apt-get install -y build-essential make python3 wget vim

# Install k6 binary.
ENV K6_VERSION=v0.55.0
RUN wget https://github.com/grafana/k6/releases/download/${K6_VERSION}/k6-${K6_VERSION}-linux-amd64.tar.gz && tar -zxvf k6-${K6_VERSION}-linux-amd64.tar.gz && mv k6-${K6_VERSION}-linux-amd64/k6 /usr/local/bin && rm k6-${K6_VERSION}-linux-amd64.tar.gz

WORKDIR /work

COPY ./k6.js .
COPY ./Makefile .
COPY ./data ./data
COPY ./scenarios ./scenarios