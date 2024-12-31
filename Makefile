# New Makefile for multi-architecture
# WARNING:
# This makefile used docker buildx to build multi-arch image
# Please make sure you have the right version of docker.
.PHONY: microservice.push swag zadig-agent zadig-agent-clean

IMAGE_REPOSITORY ?= koderover.tencentcloudcr.com/koderover-public
IMAGE_REPOSITORY := $(IMAGE_REPOSITORY)
DEV_IMAGE_REPOSITORY ?= koderover.tencentcloudcr.com/test
DEV_IMAGE_REPOSITORY := $(DEV_IMAGE_REPOSITORY)
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
buildbasetest: prereq $(BUILD_BASE_TARGETS:=.buildbasetest)
debugtools: prereq $(DEBUG_TOOLS_TARGETS:=.push)

%.image: MAKE_IMAGE_TAG ?= ${IMAGE_REPOSITORY}/$*:${VERSION}
%.image:
	@docker buildx build -t ${MAKE_IMAGE_TAG} --platform linux/amd64,linux/arm64 -f docker/$*.Dockerfile .

%.push: MAKE_IMAGE_TAG ?= ${IMAGE_REPOSITORY}/$*:${VERSION}
%.push:
	@docker buildx build -t ${MAKE_IMAGE_TAG} --platform linux/amd64,linux/arm64 -f docker/$*.Dockerfile --push .

# for zadig developers ONLY:
# the command below is used to generate amd64 file for daily developing purpose
%.dev: MAKE_IMAGE_TAG ?= ${DEV_IMAGE_REPOSITORY}/$*:${VERSION}
%.dev:
	@docker buildx build -t ${MAKE_IMAGE_TAG} --platform linux/amd64 -f docker/$*.Dockerfile --push .

%.buildbase: MAKE_IMAGE_TAG ?= ${IMAGE_REPOSITORY}/build-base:$*-with-kubectl
%.buildbase:
	@docker buildx build -t ${MAKE_IMAGE_TAG} --no-cache --platform linux/amd64,linux/arm64 -f docker/$*-base.Dockerfile --push .

%.buildbasetest: MAKE_IMAGE_TAG ?= ${IMAGE_REPOSITORY}/build-base:$*-$(shell date +'%Y%m%d%H%M%S')
%.buildbasetest:
	@docker buildx build -t ${MAKE_IMAGE_TAG} --platform linux/amd64,linux/arm64 -f docker/$*-base.Dockerfile --push .

swag:
	swag init --parseDependency --parseInternal --parseDepth 1 -d cmd/aslan,pkg/microservice/aslan -g ../../pkg/microservice/aslan/server/rest/router.go -o pkg/microservice/aslan/server/rest/doc
swag-user:
	swag init --parseDependency --parseInternal --parseDepth 1 -d cmd/aslan,pkg/microservice/user -g ../../pkg/microservice/user/server/rest/router.go -o pkg/microservice/user/server/rest/doc

# zadig-agent
# Usage:
# make zadig-agent ZADIG_AGENT_VERSION=2.1.0
# make zadig-agent-linux-amd64 ZADIG_AGENT_VERSION=2.10
# make tar-zadig-agent ZADIG_AGENT_VERSION=2.1.0
# make tar-zadig-agent-linux-amd64 ZADIG_AGENT_VERSION=2.1.0
ARCHS := amd64 arm64
PLATFORMS := windows linux darwin
BUILD_TIME := $(shell TZ=Asia/Shanghai date '+%Y-%m-%d %H:%M:%S %Z')
BUILD_COMMIT := $(shell git rev-parse --short HEAD)
BUILD_GO_VERSION := $(shell go version | awk '{print $$3}')
ZADIG_AGENT_VERSION ?= $(shell date +'%Y%m%d%H%M%S')
ZADIG_AGENT_VERSION := $(ZADIG_AGENT_VERSION)
ZADIG_AGENT_OUT_DIR := cmd/zadig-agent/out

zadig-agent: $(foreach platform,$(PLATFORMS),$(foreach arch,$(ARCHS),zadig-agent-$(platform)-$(arch)))
zadig-agent-%:
	@$(eval GOOS=$(firstword $(subst -, ,$*)))
	@$(eval GOARCH=$(lastword $(subst -, ,$*)))
	@$(eval ZADIG_AGENT_OUT_FILE=$(ZADIG_AGENT_OUT_DIR)/zadig-agent-$(GOOS)-$(GOARCH)-v$(ZADIG_AGENT_VERSION)$(if $(findstring windows,$(GOOS)),.exe))
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags '-X "main.BuildAgentVersion=$(ZADIG_AGENT_VERSION)" -X "main.BuildGoVersion=$(BUILD_GO_VERSION)" -X "main.BuildTime=$(BUILD_TIME)" -X "main.BuildCommit=$(BUILD_COMMIT)"' -o $(ZADIG_AGENT_OUT_FILE) cmd/zadig-agent/main.go

tar-zadig-agent: $(foreach platform,$(PLATFORMS),$(foreach arch,$(ARCHS),tar-zadig-agent-$(platform)-$(arch)))
tar-zadig-agent-%: zadig-agent-%
	@$(eval GOOS=$(firstword $(subst -, ,$*)))
	@$(eval GOARCH=$(lastword $(subst -, ,$*)))
	tar -czvf $(ZADIG_AGENT_OUT_DIR)/zadig-agent-$*-v$(ZADIG_AGENT_VERSION).tar.gz  -C $(ZADIG_AGENT_OUT_DIR)  zadig-agent-$*-v$(ZADIG_AGENT_VERSION)$(if $(findstring windows,$(GOOS)),.exe)

zadig-agent-clean:
	@rm -rf $(ZADIG_AGENT_OUT_DIR)/*

# debug scripts
%.build:
	./debug/build.sh $*
%.run:
	./debug/run.sh $*
%.debug:
	./debug/debug.sh $*