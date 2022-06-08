#alpine.Dockerfile

WORKDIR /app

COPY docker/dist/warpdrive .

ENTRYPOINT ["/app/warpdrive"]
