FROM golang:1.24.1-alpine as build

WORKDIR /app

ENV CGO_ENABLED=0 GOOS=linux
# ENV GOPROXY=https://goproxy.cn,direct
ENV GOCACHE=/gocache

COPY go.mod go.sum ./
COPY cmd cmd
COPY pkg pkg

RUN go mod download

RUN --mount=type=cache,id=gobuild,target=/gocache \
    go build -v -o /warpdrive ./cmd/warpdrive/main.go

FROM alpine:3.13.5

# https://wiki.alpinelinux.org/wiki/Setting_the_timezone
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo Asia/Shanghai  > /etc/timezone && \
    apk del tzdata

WORKDIR /app

COPY --from=build /warpdrive .

ENTRYPOINT ["/app/warpdrive"]
