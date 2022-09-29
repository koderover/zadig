FROM ubuntu:bionic

RUN sed -i -E "s/[a-zA-Z0-9]+.ubuntu.com/mirrors.aliyun.com/g" /etc/apt/sources.list
RUN apt-get clean && apt-get update
RUN DEBIAN_FRONTEND=noninteractive apt install -y tzdata
RUN apt-get install -y \
  curl \
  git \
  netcat-openbsd \
  wget \
  build-essential \
  libfontconfig \
  libsasl2-dev \
  libfreetype6-dev \
  libpcre3-dev \
  pkg-config \
  cmake \
  python \
  librrd-dev \
  sudo \
  apt-transport-https \
  ca-certificates

# timezone modification
RUN ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime

# install docker client
RUN curl -fsSL https://get.docker.com | bash
