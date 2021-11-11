#golang.Dockerfile

RUN go build -v -o /cron ./cmd/cron/main.go

#alpine-git.Dockerfile

WORKDIR /app

COPY --from=build /cron .

ENTRYPOINT ["/app/cron"]
