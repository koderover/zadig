FROM golang:1.24.1-alpine as build

WORKDIR /app

ENV CGO_ENABLED=0 GOOS=linux
# ENV GOPROXY=https://goproxy.cn,direct
ENV GOCACHE=/gocache

COPY go.mod go.sum ./
COPY cmd cmd
COPY pkg pkg

RUN apk add --no-cache git

RUN go mod download

RUN --mount=type=cache,id=gobuild,target=/gocache \
    go build -v -o /aslan ./cmd/aslan/main.go

FROM alpine:3.20

ENV VERSION=1.4.0

# https://wiki.alpinelinux.org/wiki/Setting_the_timezone
RUN set -eux; \
    sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories; \
    apk add --no-cache ca-certificates; \
    apk add --no-cache --virtual .fetch-deps curl tar tzdata; \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime; \
    echo Asia/Shanghai > /etc/timezone; \
    curl -fsSL "https://resources.koderover.com/helm-acr_0.8.2_linux_amd64.tar.gz" -o /tmp/helm-acr.tar.gz; \
    mkdir -p /app/.helm/helmplugin/helm-acr; \
    tar -xzf /tmp/helm-acr.tar.gz -C /app/.helm/helmplugin/helm-acr; \
    rm -f /tmp/helm-acr.tar.gz; \
    apk del .fetch-deps

WORKDIR /app

COPY --from=build /aslan .

ENTRYPOINT ["/app/aslan"]
