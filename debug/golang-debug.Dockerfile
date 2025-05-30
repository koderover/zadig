FROM golang:1.24.1-alpine

WORKDIR /app

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
ENV GOPROXY=https://goproxy.cn,direct
ENV GOCACHE=/gocache
ENV VERSION=1.4.0

# COPY go.mod go.sum ./
# COPY cmd cmd
# COPY pkg pkg
# COPY debug debug

RUN apk update
RUN apk --no-cache add bash git curl make
RUN go install github.com/go-delve/delve/cmd/dlv@latest

# https://wiki.alpinelinux.org/wiki/Setting_the_timezone
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo Asia/Shanghai  > /etc/timezone && \
    apk del tzdata

# install ali-acr plugin
RUN curl -fsSL "https://resources.koderover.com/helm-acr_0.8.2_linux_amd64.tar.gz" -o helm-acr.tar.gz &&\
    mkdir -p /app/.helm/helmplugin/helm-acr &&\
    tar -xvzf helm-acr.tar.gz -C /app/.helm/helmplugin/helm-acr &&\
    rm -rf helm-acr*

WORKDIR /app

ENTRYPOINT ["/app/aslan"]
