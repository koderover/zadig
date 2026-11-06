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
USER 65532:65532
ENTRYPOINT ["/app/halman"]
