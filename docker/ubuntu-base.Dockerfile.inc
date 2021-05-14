FROM n7832lxy.mirror.aliyuncs.com/library/ubuntu:16.04

# 修改镜像源和时区
RUN sed -i -E "s/[a-zA-Z0-9]+.ubuntu.com/mirrors.aliyun.com/g" /etc/apt/sources.list \
    && apt-get clean && apt-get update && apt-get install -y apt-transport-https ca-certificates \
    && apt-get install -y \
    tzdata \
    net-tools \
    dnsutils \
	ca-certificates \
	git \
	curl \
	lsof \
    telnet \
    && ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && rm -rf /var/lib/apt/lists/*

