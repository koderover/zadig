# New Makefile for multi-architecture
# WARNING:
# This makefile used docker buildx to build multi-arch image
# Please make sure you have the right version of docker.
.PHONY: microservice.push swag zadig-agent zadig-agent-clean

IMAGE_REPOSITORY ?= koderover.tencentcloudcr.com/koderover-public
IMAGE_REPOSITORY := $(IMAGE_REPOSITORY)
VERSION ?= $(shell date +'%Y%m%d%H%M%S')
VERSION := $(VERSION)
MICROSERVICE_TARGETS = aslan cron executor hub-agent hub-server init jenkins-plugin packager-plugin predator-plugin ua user warpdrive
BUILD_BASE_TARGETS = focal bionic
DEBUG_TOOLS_TARGETS = zadig-debug zgctl-sidecar

prereq:
	@docker buildx create --node=multiarch --use --platform=linux/amd64,linux/arm64

microservice: prereq $(MICROSERVICE_TARGETS:=.image)
microservice.push: prereq $(MICROSERVICE_TARGETS:=.push)
buildbase: prereq $(BUILD_BASE_TARGETS:=.buildbase)
debugtools: prereq $(DEBUG_TOOLS_TARGETS:=.push)

%.image: MAKE_IMAGE_TAG ?= ${IMAGE_REPOSITORY}/$*:${VERSION}
%.image:
	@docker buildx build -t ${MAKE_IMAGE_TAG} --platform linux/amd64,linux/arm64 -f docker/$*.Dockerfile .

%.push: MAKE_IMAGE_TAG ?= ${IMAGE_REPOSITORY}/$*:${VERSION}
%.push:
	@docker buildx build -t ${MAKE_IMAGE_TAG} --platform linux/amd64,linux/arm64 -f docker/$*.Dockerfile --push .

# for zadig developers ONLY:
# the command below is used to generate amd64 file for daily developing purpose
%.dev: MAKE_IMAGE_TAG ?= ${IMAGE_REPOSITORY}/$*:${VERSION}
%.dev:
	@docker buildx build -t ${MAKE_IMAGE_TAG} --platform linux/amd64 -f docker/$*.Dockerfile --push .

%.buildbase: MAKE_IMAGE_TAG ?= ${IMAGE_REPOSITORY}/build-base:$*
%.buildbase:
	@docker buildx build -t ${MAKE_IMAGE_TAG} --platform linux/amd64,linux/arm64 -f docker/$*-base.Dockerfile --push .

swag:
	swag init --parseDependency --parseInternal --parseDepth 1 -d cmd/aslan,pkg/microservice/aslan -g ../../pkg/microservice/aslan/server/rest/router.go -o pkg/microservice/aslan/server/rest/doc

zadig-agent:
	GOOS=windows GOARCH=amd64 go build -o cmd/zadig-agent/out/zadig-agent-windows-amd64.exe cmd/zadig-agent/main.go
	GOOS=windows GOARCH=arm64 go build -o cmd/zadig-agent/out/zadig-agent-windows-arm64.exe cmd/zadig-agent/main.go
	GOOS=linux GOARCH=amd64 go build -o cmd/zadig-agent/out/zadig-agent-linux-amd64 cmd/zadig-agent/main.go
	GOOS=linux GOARCH=arm64 go build -o cmd/zadig-agent/out/zadig-agent-linux-arm64 cmd/zadig-agent/main.go
	GOOS=darwin GOARCH=amd64 go build -o cmd/zadig-agent/out/adig-agent-darwin-amd64 cmd/zadig-agent/main.go
	GOOS=darwin GOARCH=arm64 go build -o cmd/zadig-agent/out/adig-agent-darwin-arm64 cmd/zadig-agent/main.go

zadig-agent-clean:
	rm -rf cmd/zadig-agent/out