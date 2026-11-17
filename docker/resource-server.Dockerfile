FROM golang:1.25.9-alpine as build

WORKDIR /app

ENV CGO_ENABLED=0 GOOS=linux
# ENV GOPROXY=https://goproxy.cn,direct
ENV GOCACHE=/gocache

COPY go.mod go.sum ./
COPY cmd cmd
COPY pkg pkg

RUN go mod download

RUN --mount=type=cache,id=gobuild,target=/gocache \
    go build -v -o /reaper ./cmd/reaper/main.go
RUN --mount=type=cache,id=gobuild,target=/gocache \
    go build -v -o /jobexecutor ./cmd/jobexecutor/main.go

FROM nginx

WORKDIR /app

ADD resource-server-nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /reaper .
COPY --from=build /jobexecutor .

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

EXPOSE 80
