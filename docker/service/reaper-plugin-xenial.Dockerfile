#golang.Dockerfile

RUN go build -v -o /reaper ./cmd/reaper/main.go

#ubuntu-xenial.Dockerfile

RUN sed -i -E "s/[a-zA-Z0-9]+.ubuntu.com/mirrors.aliyun.com/g" /etc/apt/sources.list
RUN apt-get clean && apt-get update && apt-get install -y apt-transport-https ca-certificates
RUN sed -i s:http:https:g /etc/apt/sources.list
RUN DEBIAN_FRONTEND=noninteractive apt install -y tzdata
RUN apt-get clean && apt-get update && apt-get install -y \
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

# Upgrade Git to latest version
RUN apt-get install -y software-properties-common && add-apt-repository -y ppa:git-core/ppa && apt-get update && apt-get install -y git=1:2.33.0-0ppa1~ubuntu16.04.1

# 修改时区
RUN ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime

# 安装 docker client
RUN curl -fsSL "http://resources.koderover.com/docker-cli-v19.03.2.tar.gz" -o docker.tgz &&\
    tar -xvzf docker.tgz &&\
    mv docker/* /usr/local/bin


# 替换tar（适配cephfs）
RUN rm /bin/tar && curl -fsSL http://resource.koderover.com/tar -o /bin/tar && chmod +x /bin/tar

COPY --from=build /reaper /usr/local/bin

ENTRYPOINT ["reaper"]
