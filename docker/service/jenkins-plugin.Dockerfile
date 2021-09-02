#golang.Dockerfile

RUN go build -v -o /jenkins-plugin ./cmd/jenkinsplugin/main.go

#alpine.Dockerfile

WORKDIR /app

COPY --from=build /jenkins-plugin .

ENTRYPOINT ["/app/jenkins-plugin"]
