#golang.Dockerfile

RUN go build -o /init ./cmd/initconfig/main.go

#alpine.Dockerfile

WORKDIR /app
COPY --from=build /init /app/init

ENTRYPOINT ["/app/init"]
