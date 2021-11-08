#golang.Dockerfile

RUN go build -v -o /user ./cmd/user/main.go

#alpine.Dockerfile

WORKDIR /app

COPY --from=build /user .

ENTRYPOINT ["/app/user"]
