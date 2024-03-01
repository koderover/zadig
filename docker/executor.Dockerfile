FROM golang:1.21.1-alpine as build

WORKDIR /app

ENV CGO_ENABLED=0 GOOS=linux
ENV GOPROXY=https://goproxy.cn,direct
ENV GOCACHE=/gocache

COPY v2/go.mod v2/go.sum ./
COPY v2/cmd cmd
COPY v2/pkg pkg

RUN go mod download

RUN --mount=type=cache,id=gobuild,target=/gocache \
    go build -v -o /reaper ./cmd/reaper/main.go
RUN --mount=type=cache,id=gobuild,target=/gocache \
    go build -v -o /jobexecutor ./cmd/jobexecutor/main.go

FROM alpine:3.18.0

WORKDIR /app

COPY --from=build /reaper .
COPY --from=build /jobexecutor .
