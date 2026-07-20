IMAGE ?= hello-agent
TAG ?= latest
REGISTRY ?= koderover.tencentcloudcr.com/koderover-demo
PLATFORM ?= linux/amd64
NAMESPACE ?= kagent
PORT ?= 8080
A2A_AGENT_URL ?= http://127.0.0.1:$(PORT)
GOPROXY ?= https://goproxy.cn,direct

export GOPROXY

IMG := $(REGISTRY)/$(IMAGE):$(TAG)

.PHONY: test tidy build run run-stream run-non-stream web web-a2a docker-build docker-push kagent-apply kagent-delete

test:
	go test ./...

tidy:
	go mod tidy

build:
	go build -o hello-agent ./agent.go

run:
	$(MAKE) run-stream

run-stream:
	set -a; . ./.env; set +a; go run agent.go console -streaming_mode sse

run-non-stream:
	set -a; . ./.env; set +a; go run agent.go console -streaming_mode none

web:
	set -a; . ./.env; set +a; go run agent.go web -port $(PORT) api webui a2a -a2a_agent_url $(A2A_AGENT_URL)

web-a2a:
	set -a; . ./.env; set +a; go run agent.go web -port $(PORT) a2a -a2a_agent_url $(A2A_AGENT_URL)

docker-build:
	docker build --platform $(PLATFORM) --build-arg GOPROXY=$(GOPROXY) -t $(IMG) .

docker-push:
	docker push $(IMG)

kagent-apply:
	kubectl apply -n $(NAMESPACE) -f deploy/kagent-byo-agent.yaml

kagent-delete:
	kubectl delete -n $(NAMESPACE) -f deploy/kagent-byo-agent.yaml --ignore-not-found
