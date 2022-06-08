#alpine.Dockerfile

WORKDIR /app

ADD docker/dist/hub-server .

ENTRYPOINT ["/app/hub-server"]
