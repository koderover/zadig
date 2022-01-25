#alpine.Dockerfile

WORKDIR /app

COPY docker/dist/warpdrive /app/warpdrive

ENTRYPOINT ["/app/warpdrive"]
