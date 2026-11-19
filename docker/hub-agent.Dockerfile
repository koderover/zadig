FROM golang:1.25.9-alpine as build

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
    go build -v -o /hub-agent ./cmd/hub-agent/main.go

FROM alpine/git:v2.54.0

# https://wiki.alpinelinux.org/wiki/Setting_the_timezone
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo Asia/Shanghai  > /etc/timezone && \
    apk del tzdata

RUN apk update
RUN apk --no-cache add curl curl-dev    

WORKDIR /app

COPY --from=build /hub-agent .

ARG BUILD_VERSION=unknown
ARG BUILD_COMMIT=unknown
ARG BUILD_BRANCH=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.version="${BUILD_VERSION}" \
      org.opencontainers.image.revision="${BUILD_COMMIT}" \
      org.opencontainers.image.ref.name="${BUILD_BRANCH}" \
      org.opencontainers.image.created="${BUILD_DATE}"

ENV BUILD_VERSION=${BUILD_VERSION} \
    BUILD_COMMIT=${BUILD_COMMIT} \
    BUILD_BRANCH=${BUILD_BRANCH} \
    BUILD_DATE=${BUILD_DATE}

ENTRYPOINT ["/app/hub-agent"]
