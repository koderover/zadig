#alpine-git.Dockerfile

WORKDIR /app

ADD docker/dist/hub-agent .

ENTRYPOINT ["/app/hub-agent"]
