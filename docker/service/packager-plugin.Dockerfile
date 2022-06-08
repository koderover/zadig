#alpine.Dockerfile

WORKDIR /app

ADD docker/dist/packager-plugin .

ENTRYPOINT ["/app/packager-plugin"]
