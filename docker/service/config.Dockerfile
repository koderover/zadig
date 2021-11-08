#golang.Dockerfile

RUN go build -v -o /config ./cmd/systemconfig/main.go

#alpine.Dockerfile

WORKDIR /app

COPY --from=build /config .

ENTRYPOINT ["/app/config"]
