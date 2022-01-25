#alpine-git.Dockerfile

WORKDIR /app

COPY docker/dist/aslan /app/aslan

ENTRYPOINT ["/app/aslan"]
