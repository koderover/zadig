#alpine.Dockerfile

WORKDIR /app

COPY docker/dist/user /app/user

ENTRYPOINT ["/app/user"]
