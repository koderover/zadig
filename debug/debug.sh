#!/bin/bash
# 检查是否提供了服务名参数
if [ -z "$1" ]; then
  echo "请提供服务名参数"
  exit 1
fi

SERVICE_NAME=$1
cd "$(dirname "$0")"
# ./build.sh $SERVICE_NAME
dlv --headless --log --listen :9009 --api-version 2 --accept-multiclient --wd=$(pwd)/.. exec ./$SERVICE_NAME &

# 获取Delve进程的PID
DLV_PID=$!

# 捕获Ctrl-C信号并终止Delve进程
trap "kill -9 $DLV_PID" SIGINT

# 等待Delve进程结束
wait $DLV_PID