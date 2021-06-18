#golang-deps.Dockerfile.inc

RUN go build -v -o /aslan ./cmd/aslan/main.go

#alpine-git-base.Dockerfile.inc

WORKDIR /app

COPY --from=build /aslan .

ENTRYPOINT ["/app/aslan"]
