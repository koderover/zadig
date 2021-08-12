#golang.Dockerfile

RUN go build -v -o /hubagent ./cmd/hubagent/main.go

#alpine-git.Dockerfile

WORKDIR /app

COPY --from=build /hubagent .

ENTRYPOINT ["/app/hubagent"]
