#alpine-git.Dockerfile

WORKDIR /app

ADD docker/dist/aslan .

ENTRYPOINT ["/app/aslan"]
