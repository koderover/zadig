#!/bin/bash
# 检查是否提供了服务名参数
if [ -z "$1" ]; then
  echo "请提供服务名参数"
  exit 1
fi

SERVICE_NAME=$1
cd "$(dirname "$0")"
# ./build.sh $SERVICE_NAME
./$SERVICE_NAME