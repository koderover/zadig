FROM golang:1.15 as build
RUN sed -i -E "s/[a-zA-Z0-9]+.debian.org/mirrors.aliyun.com/g" /etc/apt/sources.list \
    && apt-get update \
    && apt-get install libsasl2-dev

WORKDIR /app

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
ENV GOPROXY=https://goproxy.cn,direct

COPY third_party third_party
COPY go.mod go.sum ./

RUN go mod download

COPY cmd cmd
COPY lib lib

