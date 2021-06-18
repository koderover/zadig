#golang-deps.Dockerfile.inc

RUN go build -v -o /spock ./cmd/spock/main.go

#ubuntu-base.Dockerfile.inc

RUN curl -fsSL https://public-tinystack-1258321144.cos.ap-shanghai.myqcloud.com/tools/kubectl.linux-amd64.v1.10.11 -o /usr/local/bin/kubectl && \
  chmod +x /usr/local/bin/kubectl

WORKDIR /app

COPY --from=build /spock .

ENTRYPOINT ["/app/spock"]
