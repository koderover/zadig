#alpine-git.Dockerfile

WORKDIR /app

COPY docker/dist/podexec /app/podexec

ENTRYPOINT ["/app/podexec"]
