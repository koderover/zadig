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
ARG ZADIG_AGENT_UPGRADE_PUBLIC_KEY
ARG BUILD_COMMIT=unknown
RUN --mount=type=cache,target=/gocache \
    --mount=type=secret,id=agent_release_private_key,required=true \
    test -n "$ZADIG_AGENT_VERSION" && test -n "$ZADIG_AGENT_UPGRADE_PUBLIC_KEY" && \
    make zadig-agent ZADIG_AGENT_VERSION="$ZADIG_AGENT_VERSION" \
      ZADIG_AGENT_UPGRADE_PUBLIC_KEY="$ZADIG_AGENT_UPGRADE_PUBLIC_KEY" BUILD_COMMIT="$BUILD_COMMIT" && \
    go run ./cmd/zadig-agent-release --format binary \
      --version "$ZADIG_AGENT_VERSION" --dir cmd/zadig-agent/out \
      --public-key "$ZADIG_AGENT_UPGRADE_PUBLIC_KEY" \
      --private-key /run/secrets/agent_release_private_key && \
    printf 'ok\n' > cmd/zadig-agent/out/healthz

FROM nginx:alpine
COPY docker/agent-resource-server-nginx.conf /etc/nginx/nginx.conf
COPY --from=build /src/cmd/zadig-agent/out/ /usr/share/nginx/html/agent-resources/
USER 101
EXPOSE 8080
ENTRYPOINT ["nginx"]
CMD ["-g", "daemon off;"]
