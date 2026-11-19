FROM golang:1.25.9-alpine AS build
WORKDIR /app
ENV CGO_ENABLED=0 GOOS=linux GOCACHE=/gocache
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/halman cmd/halman
COPY pkg pkg
RUN --mount=type=cache,id=gobuild,target=/gocache go build -o /halman ./cmd/halman

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /halman /app/halman

ARG BUILD_VERSION=unknown
ARG BUILD_COMMIT=unknown
ARG BUILD_BRANCH=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.version="${BUILD_VERSION}" \
      org.opencontainers.image.revision="${BUILD_COMMIT}" \
      org.opencontainers.image.ref.name="${BUILD_BRANCH}" \
      org.opencontainers.image.created="${BUILD_DATE}"

ENV BUILD_VERSION=${BUILD_VERSION} \
    BUILD_COMMIT=${BUILD_COMMIT} \
    BUILD_BRANCH=${BUILD_BRANCH} \
    BUILD_DATE=${BUILD_DATE}

USER 65532:65532
ENTRYPOINT ["/app/halman"]
