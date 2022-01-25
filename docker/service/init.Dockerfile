#alpine.Dockerfile

WORKDIR /app

COPY docker/dist/init /app/init

ENTRYPOINT ["/app/init"]
