#alpine.Dockerfile

WORKDIR /app

COPY docker/dist/ua /app/ua

ENTRYPOINT ["/app/ua"]