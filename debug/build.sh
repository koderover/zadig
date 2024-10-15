#!/bin/bash
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64
export GOPROXY=https://goproxy.cn,direct

# 检查是否提供了服务名参数
if [ -z "$1" ]; then
  echo "请提供服务名参数"
  exit 1
fi

SERVICE_NAME=$1

cd "$(dirname "$0")"
cd ..
go mod download
go build -v -gcflags="all=-trimpath=$PWD -N -l" -asmflags "all=-trimpath=$PWD" -o ./debug/$SERVICE_NAME ./cmd/$SERVICE_NAME/main.go
# docker build --platform linux/amd64 -t koderover.tencentcloudcr.com/test/golang-debug:latest . -f ./debug/golang-debug.Dockerfile --push