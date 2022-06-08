#alpine.Dockerfile

WORKDIR /app

ADD docker/dist/init /app/init

ENTRYPOINT ["/app/init"]
