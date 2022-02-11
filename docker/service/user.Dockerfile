#alpine.Dockerfile

WORKDIR /app

ADD docker/dist/user .

ENTRYPOINT ["/app/user"]
