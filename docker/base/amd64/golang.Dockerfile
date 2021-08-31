FROM golang:1.16.5 as build
RUN sed -i -E "s/[a-zA-Z0-9]+.debian.org/mirrors.aliyun.com/g" /etc/apt/sources.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends libsasl2-dev=2.1.27+dfsg-1+deb10u1

WORKDIR /app

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./

RUN go mod download

COPY cmd cmd
COPY pkg pkg
COPY version version
