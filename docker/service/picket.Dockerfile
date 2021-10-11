#golang.Dockerfile

RUN go build -v -o /picket ./cmd/picket/main.go

#alpine.Dockerfile

WORKDIR /app

COPY --from=build /picket .

ENTRYPOINT ["/app/picket"]
