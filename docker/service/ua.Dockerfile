#alpine.Dockerfile

WORKDIR /app

ADD docker/dist/ua .

ENTRYPOINT ["/app/ua"]