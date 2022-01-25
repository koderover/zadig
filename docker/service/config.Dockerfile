#alpine.Dockerfile

WORKDIR /app

COPY docker/dist/config /app/config

ENTRYPOINT ["/app/config"]
