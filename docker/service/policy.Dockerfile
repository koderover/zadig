#alpine.Dockerfile

WORKDIR /app

COPY docker/dist/policy /app/policy

ENTRYPOINT ["/app/policy"]
