#golang.Dockerfile

RUN go build -v -o /packager-plugin ./cmd/packager/main.go

#alpine.Dockerfile

RUN apk --no-cache add curl
# install docker client
RUN apk --no-cache add curl && \
    curl -fsSL "http://resources.koderover.com/docker-cli-v19.03.2.tar.gz" -o docker.tgz &&\
    tar -xvzf docker.tgz &&\
    mv docker/* /usr/local/bin &&\
    rm -rf docke*

WORKDIR /app

COPY --from=build /packager-plugin .

ENTRYPOINT ["/app/packager-plugin"]
