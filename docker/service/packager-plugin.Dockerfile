#alpine.Dockerfile

WORKDIR /app

COPY docker/dist/packager-plugin /app/packager-plugin

ENTRYPOINT ["/app/packager-plugin"]
