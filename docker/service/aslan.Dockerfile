#golang.Dockerfile

RUN go build -v -o /aslan ./cmd/aslan/main.go

#alpine.Dockerfile

WORKDIR /app

COPY --from=build /aslan .

ENTRYPOINT ["/app/aslan"]
