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

RUN go build -v -o /podexec ./cmd/podexec/...

FROM n7832lxy.mirror.aliyuncs.com/library/ubuntu:16.04

# 修改镜像源和时区
RUN sed -i -E "s/[a-zA-Z0-9]+.ubuntu.com/mirrors.aliyun.com/g" /etc/apt/sources.list \
    && apt-get clean && apt-get update && apt-get install -y apt-transport-https ca-certificates \
    && apt-get install -y \
    tzdata \
    net-tools \
    dnsutils \
	ca-certificates \
	git \
	curl \
	lsof \
    telnet \
    && ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /podexec /app/podexec

ENTRYPOINT ["/app/podexec"]
