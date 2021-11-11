#golang.Dockerfile

RUN go build -v -o /predator-plugin ./cmd/predator/main.go

#ubuntu-xenial.Dockerfile

# 安装 docker client
RUN curl -fsSL "http://resources.koderover.com/docker-cli-v19.03.2.tar.gz" -o docker.tgz &&\
    tar -xvzf docker.tgz &&\
    mv docker/* /usr/local/bin &&\
    rm -rf docke*

WORKDIR /app

COPY --from=build /predator-plugin .

ENTRYPOINT ["/app/predator-plugin"]
