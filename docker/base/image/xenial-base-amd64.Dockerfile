FROM ubuntu:xenial

RUN sed -i -E "s/[a-zA-Z0-9]+.ubuntu.com/mirrors.aliyun.com/g" /etc/apt/sources.list
RUN apt-get clean && apt-get update && apt-get install -y apt-transport-https ca-certificates
RUN DEBIAN_FRONTEND=noninteractive apt install -y tzdata
RUN apt-get clean && apt-get update && apt-get install -y \
  curl \
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
  python \
  net-tools \
  dnsutils \
  ca-certificates \
  lsof \
  telnet \
  sudo

# Upgrade Git to latest version,forcing IPv4 transport with apt-get
RUN echo "deb http://launchpad.proxy.ustclug.org/git-core/ppa/ubuntu xenial main" >> /etc/apt/sources.list

# Using 'launchpad.proxy.ustclug.org' to reverse proxy for 'launchpad.net'. Detail:https://lug.ustc.edu.cn/wiki/mirrors/help/revproxy/
RUN echo "deb-src http://launchpad.proxy.ustclug.org/git-core/ppa/ubuntu xenial main" >> /etc/apt/sources.list

# Import key manually to solve bad network problems
RUN echo  '-----BEGIN PGP PUBLIC KEY BLOCK----- \nVersion:  \nComment: Hostname: keyserver.ubuntu.com \n \nxo0ESXjaGwEEAMA26F3+mnRW8uRqASMsEa5EsmgvUpLD7EKpC7903OpiMGSvZ2sE \n34g7W6nUQY0R//AZS2iW4ZXfvdhQTQuuPlHM6Q3iUAt+nyXcf9xBlscs8Gm722u4 \njAtFtBS4BMQRhRRfWTHwJIOM6OpGIccjPe8pQfIeoRxkKJxlehzw2mU1ABEBAAHN \nKExhdW5jaHBhZCBQUEEgZm9yIFVidW50dSBHaXQgTWFpbnRhaW5lcnPCtgQTAQIA \nIAUCSXjaGwIbAwYLCQgHAwIEFQIIAwQWAgMBAh4BAheAAAoJEKFxXYjh3x8k/zMD \n/RKBMjavvFl71YBazSOGl2YfSsZiR/ANsby3+rUaULb8uxzCHXAQnlH5vdtLSPry \naLBvzCU8C3C02qNT8jRacU2752zsCkCi1SLRSOXdI/ATJHza5aTvYV93rTITBhU4 \nsJQeK9RW0CtaDRAxJsn/Dr6J3lL/c9m9cT5fFpxOIsF4 \n=7kFR \n-----END PGP PUBLIC KEY BLOCK----- \n' > key

RUN apt-key add key

# Forcing IPv4 transport with apt-get
RUN apt-get -o Acquire::ForceIPv4=true update

RUN apt-get -o Acquire::ForceIPv4=true install -y git

# Change timezone to Asia/Shanghai
RUN ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime

# Install docker client
RUN curl -fsSL "http://resources.koderover.com/docker-cli-v19.03.2.tar.gz" -o docker.tgz &&\
    tar -xvzf docker.tgz &&\
    mv docker/* /usr/local/bin


# Replaces the default tar（for cephfs）
RUN rm /bin/tar && curl -fsSL http://resource.koderover.com/tar -o /bin/tar && chmod +x /bin/tar
