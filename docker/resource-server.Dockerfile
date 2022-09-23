FROM golang:1.19.1-alpine as build

WORKDIR /app

ENV CGO_ENABLED=0 GOOS=linux
ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
COPY cmd cmd
COPY pkg pkg

RUN go mod download

RUN go build -v -o /reaper ./cmd/reaper/main.go
RUN go build -v -o /jobexecutor ./cmd/jobexecutor/main.go

FROM nginx

WORKDIR /app

ADD resource-server-nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build docker/dist/reaper .
COPY --from=build docker/dist/jobexecutor .

EXPOSE 80
