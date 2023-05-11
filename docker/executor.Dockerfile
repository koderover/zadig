FROM golang:1.19.1-alpine as build

WORKDIR /app

ENV CGO_ENABLED=0 GOOS=linux
ENV GOPROXY=https://goproxy.cn,direct
ENV GOCACHE=/gocache

COPY go.mod go.sum ./
COPY cmd cmd
COPY pkg pkg

RUN go mod download

RUN --mount=type=cache,id=gobuild,target=/gocache \
    go build -v -o /reaper ./cmd/reaper/main.go
RUN --mount=type=cache,id=gobuild,target=/gocache \
    go build -v -o /jobexecutor ./cmd/jobexecutor/main.go

FROM alpine:3.18.0

WORKDIR /app

COPY --from=build /reaper .
COPY --from=build /jobexecutor .
