FROM golang:1.23.5

WORKDIR /work

COPY ./go.mod .
COPY ./go.sum .

RUN go mod download

COPY ./main.go .
COPY ./benchmark ./benchmark

RUN mkdir bin
ENV PATH="/work/bin:$PATH"
RUN go build -o bin/bench ./main.go

COPY ./data ./data
COPY ./example ./example

ENTRYPOINT [ "bench" ]