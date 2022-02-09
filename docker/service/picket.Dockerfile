#alpine.Dockerfile

WORKDIR /app

ADD docker/dist/picket .

ENTRYPOINT ["/app/picket"]
