FROM python:3.6-alpine

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

RUN apk update && \
    apk add bash && \
    pip install selenium pytest pytest-html -i https://mirrors.aliyun.com/pypi/simple/