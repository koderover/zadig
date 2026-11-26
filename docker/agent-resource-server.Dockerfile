# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.25.9-alpine AS build
RUN apk add --no-cache make
WORKDIR /src
ENV CGO_ENABLED=0 GOCACHE=/gocache
COPY go.mod go.sum ./
RUN go mod download
COPY cmd cmd
COPY pkg pkg
COPY Makefile ./
ARG ZADIG_AGENT_VERSION
ARG BUILD_COMMIT=unknown
RUN --mount=type=cache,target=/gocache \
    test -n "$ZADIG_AGENT_VERSION" && \
    make zadig-agent-linux-amd64 zadig-agent-linux-arm64 \
      zadig-agent-darwin-amd64 zadig-agent-darwin-arm64 zadig-agent-windows-amd64 \
      ZADIG_AGENT_VERSION="$ZADIG_AGENT_VERSION" \
      BUILD_COMMIT="$BUILD_COMMIT" && \
    go run ./cmd/zadig-agent-release \
      --version "$ZADIG_AGENT_VERSION" --dir cmd/zadig-agent/out && \
    printf 'ok\n' > cmd/zadig-agent/out/healthz

FROM nginx:alpine
COPY docker/agent-resource-server-nginx.conf /etc/nginx/nginx.conf
COPY --from=build /src/cmd/zadig-agent/out/ /usr/share/nginx/html/agent-resources/
COPY pkg/microservice/aslan/core/vm/service/installer/install.sh /usr/share/nginx/html/agent-resources/install.sh
COPY pkg/microservice/aslan/core/vm/service/installer/install.ps1 /usr/share/nginx/html/agent-resources/install.ps1
USER 101
EXPOSE 8080
ENTRYPOINT ["nginx"]
CMD ["-g", "daemon off;"]
