#golang.Dockerfile

RUN go build -v -o /hubserver ./cmd/hubserver/main.go

#alpine.Dockerfile

WORKDIR /app

COPY --from=build /hubserver .

ENTRYPOINT ["/app/hubserver"]
