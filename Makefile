# New Makefile for multi-architecture
.PHONY: all.push

IMAGE_REPOSITORY = koderover.tencentcloudcr.com/koderover-public
VERSION ?= $(shell date +'%Y%m%d%H%M%S')
VERSION := $(VERSION)
TARGETS = aslan cron hub-agent hub-server init jenkins-plugin packager-plugin predator-plugin resource-server ua warpdrive zadig-debug zgctl-sidecar


all: $(TARGETS:=.image)
all.push: $(TARGETS:=.push)

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

