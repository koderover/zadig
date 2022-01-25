#alpine-git.Dockerfile

WORKDIR /app

COPY docker/dist/hub-agent /app/hub-agent

ENTRYPOINT ["/app/hub-agent"]
