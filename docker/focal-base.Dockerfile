FROM ubuntu:focal

RUN sed -i -E "s/[a-zA-Z0-9]+.ubuntu.com/mirrors.aliyun.com/g" /etc/apt/sources.list
RUN apt-get clean && apt-get update && apt-get install -y apt-transport-https ca-certificates
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
  sudo

# timezone modification
RUN ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime

RUN wget -qO - https://package.perforce.com/perforce.pubkey | gpg --dearmor | sudo tee /usr/share/keyrings/perforce.gpg
RUN echo deb [signed-by=/usr/share/keyrings/perforce.gpg] https://package.perforce.com/apt/ubuntu focal release > /etc/apt/sources.list.d/perforce.list
RUN cat /etc/apt/sources.list.d/perforce.list
RUN apt-get update && apt-get install -y helix-p4d

# install docker client
RUN curl -fsSL https://get.docker.com | bash

