# New Makefile for multi-architecture
.PHONY: all

IMAGE_REPOSITORY = koderover.tencentcloudcr.com/koderover-public
VERSION ?= $(shell date +'%Y%m%d%H%M%S')
VERSION := $(VERSION)
TARGETS = aslan cron hub-agent hub-server init jenkins-plugin packager-plugin predator-plugin ua warpdrive

all:

%.image: MAKE_IMAGE_TAG ?= ${IMAGE_REPOSITORY}/$*:${VERSION}
%.image:
	@mkdir -p docker/dist/amd64
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o docker/dist/$* cmd/$*/main.go
