FROM golang:1.19.1-alpine as build

VOLUME ["/gocache"]

WORKDIR /app

ENV CGO_ENABLED=0 GOOS=linux
ENV GOPROXY=https://goproxy.cn,direct
ENV GOCACHE=/gocache

COPY go.mod go.sum ./
COPY cmd cmd
COPY pkg pkg

RUN go mod download

RUN go build -v -o /predator-plugin ./cmd/predator-plugin/main.go

FROM koderover.tencentcloudcr.com/koderover-public/build-base:focal

WORKDIR /app

COPY --from=build /predator-plugin .

ENTRYPOINT ["/app/predator-plugin"]
