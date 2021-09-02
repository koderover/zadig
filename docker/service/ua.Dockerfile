#golang.Dockerfile

RUN go build -o /ua ./cmd/upgradeassistant/main.go

#alpine.Dockerfile

WORKDIR /app
COPY --from=build /ua /app/ua

ENTRYPOINT ["/app/ua"]