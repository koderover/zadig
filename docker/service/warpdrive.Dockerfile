#golang.Dockerfile

RUN go build -v -o /warpdrive ./cmd/warpdrive/main.go

#alpine.Dockerfile

WORKDIR /app

COPY --from=build /warpdrive .

ENTRYPOINT ["/app/warpdrive"]
