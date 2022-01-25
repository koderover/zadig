FROM arm64v8/ubuntu:bionic

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

# install docker client
RUN curl -fsSL "http://resources.koderover.com/docker-cli-v19.03.2.tar.gz" -o docker.tgz &&\
    tar -xvzf docker.tgz &&\
    mv docker/* /usr/local/bin

# replace tar（compatible for cephfs）
RUN rm /bin/tar && curl -fsSL http://resource.koderover.com/tar -o /bin/tar && chmod +x /bin/tar
