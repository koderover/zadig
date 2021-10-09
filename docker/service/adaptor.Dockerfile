#golang.Dockerfile

RUN go build -v -o /adaptor ./cmd/adaptor/main.go

#alpine.Dockerfile

WORKDIR /app

COPY --from=build /adaptor .

ENTRYPOINT ["/app/adaptor"]
