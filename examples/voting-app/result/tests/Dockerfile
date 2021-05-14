FROM wernight/phantomjs

USER root
RUN sed -i "s/deb.debian.org/mirrors.aliyun.com/g" /etc/apt/sources.list
RUN sed -i "s/security.debian.org/mirrors.aliyun.com/g" /etc/apt/sources.list

RUN apt update && apt install -y curl
# RUN apt-get update && apt-get install -qy \ 
#     ca-certificates \
#     bzip2 \
#     curl \
#     libfontconfig \
#     --no-install-recommends
# RUN yarn config set registry https://registry.npm.taobao.org/
# RUN yarn global add --verbose phantomjs-prebuilt
ADD . /app
WORKDIR /app
CMD ["/app/tests.sh"]
