#alpine.Dockerfile

WORKDIR /app

ADD docker/dist/config .

ENTRYPOINT ["/app/config"]
