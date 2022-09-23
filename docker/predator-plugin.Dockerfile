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

FROM koderover.tencentcloudcr.com/koderover-public/build-base:xenial-amd64

# install docker client
RUN curl -fsSL "http://resources.koderover.com/docker-cli-v19.03.2.tar.gz" -o docker.tgz &&\
    tar -xvzf docker.tgz &&\
    mv docker/* /usr/local/bin &&\
    rm -rf docke*

WORKDIR /app

COPY --from=build /predator-plugin .

ENTRYPOINT ["/app/predator-plugin"]
