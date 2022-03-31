#alpine-curl.Dockerfile

# install docker client
RUN curl -fsSL "https://resources.koderover.com/helm-acr_0.8.2_linux_amd64.tar.gz" -o helm-acr.tar.gz &&\
    mkdir -p /app/.helm/helmplugin/helm-acr &&\
    tar -xvzf helm-acr.tar.gz -C /app/.helm/helmplugin/helm-acr &&\
    rm -rf helm-acr*

WORKDIR /app

ADD docker/dist/aslan .

ENTRYPOINT ["/app/aslan"]
