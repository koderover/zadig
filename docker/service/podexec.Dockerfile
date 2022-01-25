#alpine-git.Dockerfile

WORKDIR /app

ADD docker/dist/podexec .

ENTRYPOINT ["/app/podexec"]
