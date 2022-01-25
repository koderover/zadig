#alpine.Dockerfile

WORKDIR /app

COPY docker/dist/picket /app/picket

ENTRYPOINT ["/app/picket"]
