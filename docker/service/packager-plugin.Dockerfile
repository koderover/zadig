#golang.Dockerfile

RUN go build -v -o /packager-plugin ./cmd/packager/main.go

#alpine.Dockerfile

WORKDIR /app

COPY --from=build /packager-plugin .

ENTRYPOINT ["/app/packager-plugin"]
