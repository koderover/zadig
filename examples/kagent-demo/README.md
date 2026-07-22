# Go ADK DevOps Agent Demo

This demo shows a Go ADK v2 agent that does not use Gemini. It connects to a custom model backend through either an OpenAI Chat Completions compatible API or an Anthropic Messages compatible API, then uses an ADK function tool to look up DevOps runbooks.

## What It Demonstrates

- Go ADK agent setup with `full.NewLauncher()`.
- Custom `model.LLM` implementation for non-Gemini model backends.
- ADK tool calling with a Go function tool named `lookup_runbook`.
- A DevOps incident triage flow: user report -> tool call -> runbook-backed action plan.

The built-in runbooks are mock data for `payment-api`, `order-worker`, and `gateway`. Replace `internal/runbook` with real Prometheus, Loki, Kubernetes, CMDB, or internal SOP APIs to turn this into a real assistant.

## Configure

Copy the example environment and fill in your model gateway details:

```bash
cp .env.example .env
```

OpenAI-compatible example:

```bash
MODEL_PROVIDER=openai
MODEL_BASE_URL=https://models.example.com/v1
MODEL_API_KEY=replace-me
MODEL_NAME=devops-assistant
MODEL_TIMEOUT=30s
```

Anthropic-compatible example:

```bash
MODEL_PROVIDER=anthropic
MODEL_BASE_URL=https://anthropic-compatible.example.com
MODEL_API_KEY=replace-me
MODEL_NAME=claude-compatible
MODEL_TIMEOUT=30s
```

## Run

This project defaults to the China Go proxy for local Make targets and Docker
builds:

```bash
GOPROXY=https://goproxy.cn,direct
```

Override it when needed:

```bash
make test GOPROXY=https://proxy.golang.org,direct
make docker-build GOPROXY=https://proxy.golang.org,direct
```

Console mode:

```bash
set -a; . ./.env; set +a
go run agent.go
```

Streaming console mode:

```bash
make run
```

Non-streaming console mode:

```bash
make run-non-stream
```

Web UI:

```bash
make web
```

This starts the ADK API, Web UI, and A2A endpoint together. The A2A endpoint
advertises streaming support.

A2A Web server:

```bash
make web-a2a
```

`make run` defaults to console SSE streaming. The container image and kagent BYO
Agent manifest default to the ADK A2A server, which advertises streaming support
and uses streaming model calls when the A2A client requests streaming.

Override the port if needed:

```bash
make web PORT=8090
make web-a2a PORT=8090 A2A_AGENT_URL=http://127.0.0.1:8090
```

Container image:

```bash
make docker-build REGISTRY=localhost:5001 IMAGE=hello-agent TAG=latest
make docker-push REGISTRY=localhost:5001 IMAGE=hello-agent TAG=latest
```

kagent BYO Agent:

```bash
kubectl create namespace kagent --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f deploy/kagent-byo-agent.yaml
```

Before applying to a real cluster, edit `deploy/kagent-byo-agent.yaml`:

- Set `stringData.MODEL_PROVIDER`, `MODEL_BASE_URL`, `MODEL_API_KEY`, and `MODEL_NAME`.
- Set `spec.byo.deployment.image` to the image you built and pushed.

The container starts the ADK A2A server by default:

```bash
/hello-agent web -port 8080 api webui a2a -a2a_agent_url http://127.0.0.1:8080
```

Then try:

```text
payment-api 延迟升高，帮我查一下 runbook
gateway 5xx 增多，下一步怎么排查？
order-worker queue lag 很高，应该找谁？
```

## Test

```bash
go test ./...
```
