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

# zadig-agent
ARCHS := amd64 arm64
PLATFORMS := windows linux darwin
ZADIG_AGENT_OUT_DIR := cmd/zadig-agent/out

zadig-agent: $(foreach platform,$(PLATFORMS),$(foreach arch,$(ARCHS),zadig-agent-$(platform)-$(arch)))

zadig-agent-clean:
	@rm -rf $(ZADIG_AGENT_OUT_DIR)/*

zadig-agent-%:
	@$(eval GOOS=$(firstword $(subst -, ,$*)))
	@$(eval GOARCH=$(lastword $(subst -, ,$*)))
	@echo "Building zadig-agent for GOOS=$(GOOS), GOARCH=$(GOARCH)"
	@$(eval ZADIG_AGENT_OUT_FILE=$(ZADIG_AGENT_OUT_DIR)/zadig-agent-$(GOOS)-$(GOARCH)$(if $(findstring windows,$(GOOS)),.exe))
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(ZADIG_AGENT_OUT_FILE) cmd/zadig-agent/main.go