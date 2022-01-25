#alpine.Dockerfile

WORKDIR /app

COPY docker/dist/hub-server /app/hub-server

ENTRYPOINT ["/app/hub-server"]
