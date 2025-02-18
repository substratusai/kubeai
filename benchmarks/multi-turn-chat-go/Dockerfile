FROM ubuntu:24.04
RUN apt-get update -y && \
    apt-get install -y python3 python3-pip golang
RUN apt-get install -y python3.12-venv

WORKDIR /work
RUN python3 -m venv venv
ENV PATH="/work/venv/bin:$PATH"
RUN pip install pydantic fastapi 'uvicorn[standard]' transformers

COPY ./go.mod .
COPY ./go.sum .

RUN go mod download

COPY ./main.go .
COPY ./benchmark ./benchmark
COPY ./tokenizer ./tokenizer

RUN mkdir bin
ENV PATH="/work/bin:$PATH"
RUN go build -o bin/bench ./main.go

COPY ./data ./data
COPY ./example ./example

ENTRYPOINT [ "bench" ]