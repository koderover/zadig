#alpine-git.Dockerfile

WORKDIR /app

COPY docker/dist/cron /app/cron

ENTRYPOINT ["/app/cron"]
