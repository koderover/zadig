FROM koderover.tencentcloudcr.com/koderover-demo/golang:1.25-alpine AS build

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/hello-agent ./agent.go

FROM koderover.tencentcloudcr.com/koderover-demo/golang:1.25-alpine

WORKDIR /
COPY --from=build /out/hello-agent /hello-agent

EXPOSE 8080

ENTRYPOINT ["/hello-agent"]
# The default container mode is the ADK A2A server. ADK advertises A2A
# streaming support from this endpoint, and the llmbridge adapter streams
# OpenAI/Anthropic-compatible SSE responses when the caller requests streaming.
CMD ["web", "-port", "8080", "api", "webui", "a2a", "-a2a_agent_url", "http://127.0.0.1:8080"]
