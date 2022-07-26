FROM alpine/git:v2.30.2

# https://wiki.alpinelinux.org/wiki/Setting_the_timezone
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo Asia/Shanghai  > /etc/timezone && \
    apk del tzdata

WORKDIR /app

ADD update-nacos-config .

ENTRYPOINT ["/app/update-nacos-config"]
