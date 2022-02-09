#alpine.Dockerfile

WORKDIR /app

ADD docker/dist/policy .

ENTRYPOINT ["/app/policy"]
