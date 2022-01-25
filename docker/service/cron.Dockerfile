#alpine-git.Dockerfile

WORKDIR /app

ADD docker/dist/cron .

ENTRYPOINT ["/app/cron"]
