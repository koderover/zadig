FROM golang:1.24.1-alpine as build

WORKDIR /app

ENV CGO_ENABLED=0 GOOS=linux
ENV GOPROXY=https://goproxy.cn,direct
ENV GOCACHE=/gocache

COPY go.mod go.sum ./
COPY cmd cmd
COPY pkg pkg

RUN go mod download

RUN --mount=type=cache,id=gobuild,target=/gocache \
    go build -v -o /predator-plugin ./cmd/predator-plugin/main.go

FROM koderover.tencentcloudcr.com/koderover-public/build-base:focal

WORKDIR /app

COPY --from=build /predator-plugin .

ENTRYPOINT ["/app/predator-plugin"]
