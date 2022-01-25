#ubuntu-xenial.Dockerfile

# install docker client
RUN curl -fsSL "http://resources.koderover.com/docker-cli-v19.03.2.tar.gz" -o docker.tgz &&\
    tar -xvzf docker.tgz &&\
    mv docker/* /usr/local/bin &&\
    rm -rf docke*

WORKDIR /app

ADD docker/dist/predator-plugin .

ENTRYPOINT ["/app/predator-plugin"]
