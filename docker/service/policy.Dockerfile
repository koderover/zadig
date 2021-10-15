#golang.Dockerfile

RUN go build -v -o /policy ./cmd/policy/main.go

#alpine.Dockerfile

WORKDIR /app

COPY --from=build /policy .

ENTRYPOINT ["/app/policy"]
