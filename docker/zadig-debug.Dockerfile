FROM alpine:3.13.5

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

RUN apk update
RUN apk upgrade
RUN apk add bash curl wget iputils busybox-extras bind-tools tcpdump net-tools procps sysstat
