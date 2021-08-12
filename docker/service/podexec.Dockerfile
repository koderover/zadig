#golang.Dockerfile

RUN go build -v -o /podexec ./cmd/podexec/...

#alpine-git.Dockerfile

WORKDIR /app
COPY --from=build /podexec /app/podexec

ENTRYPOINT ["/app/podexec"]
