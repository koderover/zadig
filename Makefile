# New Makefile for multi-architecture
.PHONY: all

IMAGE_REPOSITORY = ccr.ccs.tencentyun.com/koderover-rc
VERSION ?= $(shell date +'%Y%m%d%H%M%S')
VERSION := $(VERSION)
TARGETS = aslan config cron hub-agent hub-server init jenkins-plugin packager-plugin picket podexec policy predator-plugin ua user warpdrive
REAPER_OS= focal xenial bionic

ALL_IMAGES=$(TARGETS:=.image)
ALL_PUSH=$(TARGETS:=.push)
ALL_REAPER=$(REAPER_OS:=.reaper.image)
ALL_REAPER_PUSH=$(REAPER_OS:=.reaper.push)

all: $(ALL_IMAGES:=.amd64) $(ALL_IMAGES:=.arm64) $(ALL_REAPER:=.amd64) $(ALL_REAPER:=.arm64) resource-server.build.amd64 resource-server.build.arm64
all.push: $(ALL_PUSH:=.amd64) $(ALL_PUSH:=.arm64) $(ALL_REAPER_PUSH:=.amd64) $(ALL_REAPER_PUSH:=.arm64) resource-server.upload.amd64 resource-server.upload.arm64

all.amd64: $(ALL_IMAGES:=.amd64) $(ALL_REAPER:=.amd64) resource-server.build.amd64
all.arm64: $(ALL_IMAGES:=.amd64) $(ALL_REAPER:=.arm64) resource-server.build.arm64
allpush.amd64: $(ALL_PUSH:=.amd64) $(ALL_REAPER_PUSH:=.amd64) resource-server.upload.amd64
allpush.arm64: $(ALL_PUSH:=.arm64) $(ALL_REAPER_PUSH:=.arm64) resource-server.upload.arm64

%.reaper.push.arm64: MAKE_IMAGE = ${IMAGE_REPOSITORY}/reaper-plugin:${VERSION}-arm64-$*
%.reaper.push.arm64: %.reaper.image.arm64
	@docker push ${MAKE_IMAGE}

%.reaper.image.arm64: MAKE_IMAGE = ${IMAGE_REPOSITORY}/reaper-plugin:${VERSION}-arm64-$*
%.reaper.image.arm64:
	@mkdir -p docker/dist/arm64
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -v -o docker/dist/reaper cmd/reaper/main.go
	@cp docker/service/reaper-plugin-$*.Dockerfile docker/dist/arm64/reaper-plugin-$*.Dockerfile
	@sed -i -e '/#ubuntu-xenial.Dockerfile/ {' -e 'r docker/base/arm64/ubuntu-xenial.Dockerfile' -e 'd' -e '}' docker/dist/arm64/reaper-plugin-$*.Dockerfile
	@sed -i -e '/#ubuntu-focal.Dockerfile/ {' -e 'r docker/base/arm64/ubuntu-focal.Dockerfile' -e 'd' -e '}' docker/dist/arm64/reaper-plugin-$*.Dockerfile
	@sed -i -e '/#ubuntu-bionic.Dockerfile/ {' -e 'r docker/base/arm64/ubuntu-bionic.Dockerfile' -e 'd' -e '}' docker/dist/arm64/reaper-plugin-$*.Dockerfile
	@docker build -f docker/dist/arm64/reaper-plugin-$*.Dockerfile --tag ${MAKE_IMAGE} .

%.reaper.push.amd64: MAKE_IMAGE = ${IMAGE_REPOSITORY}/reaper-plugin:${VERSION}-amd64-$*
%.reaper.push.amd64: %.reaper.image.amd64
	@docker push ${MAKE_IMAGE}

%.reaper.image.amd64: MAKE_IMAGE = ${IMAGE_REPOSITORY}/reaper-plugin:${VERSION}-amd64-$*
%.reaper.image.amd64:
	@mkdir -p docker/dist/amd64
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o docker/dist/reaper cmd/reaper/main.go
	@cp docker/service/reaper-plugin-$*.Dockerfile docker/dist/amd64/reaper-plugin-$*.Dockerfile
	@sed -i -e '/#ubuntu-xenial.Dockerfile/ {' -e 'r docker/base/amd64/ubuntu-xenial.Dockerfile' -e 'd' -e '}' docker/dist/amd64/reaper-plugin-$*.Dockerfile
	@sed -i -e '/#ubuntu-focal.Dockerfile/ {' -e 'r docker/base/amd64/ubuntu-focal.Dockerfile' -e 'd' -e '}' docker/dist/amd64/reaper-plugin-$*.Dockerfile
	@sed -i -e '/#ubuntu-bionic.Dockerfile/ {' -e 'r docker/base/amd64/ubuntu-bionic.Dockerfile' -e 'd' -e '}' docker/dist/amd64/reaper-plugin-$*.Dockerfile
	@docker build -f docker/dist/amd64/reaper-plugin-$*.Dockerfile --tag ${MAKE_IMAGE} .

%.push.amd64: MAKE_IMAGE ?= ${IMAGE_REPOSITORY}/$*:${VERSION}-amd64
%.push.amd64: %.image.amd64
	@docker push ${MAKE_IMAGE}

%.image.amd64: MAKE_IMAGE ?= ${IMAGE_REPOSITORY}/$*:${VERSION}-amd64
%.image.amd64:
	@mkdir -p docker/dist/amd64
	@cp docker/service/$*.Dockerfile docker/dist/amd64/$*.Dockerfile
	@sed -i -e '/#alpine-git.Dockerfile/ {' -e 'r docker/base/amd64/alpine-git.Dockerfile' -e 'd' -e '}' docker/dist/amd64/$*.Dockerfile
	@sed -i -e '/#alpine.Dockerfile/ {' -e 'r docker/base/amd64/alpine.Dockerfile' -e 'd' -e '}' docker/dist/amd64/$*.Dockerfile
	@sed -i -e '/#ubuntu-xenial.Dockerfile/ {' -e 'r docker/base/amd64/ubuntu-xenial.Dockerfile' -e 'd' -e '}' docker/dist/amd64/$*.Dockerfile
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o docker/dist/$* cmd/$*/main.go
	@docker build -f docker/dist/amd64/$*.Dockerfile --tag ${MAKE_IMAGE} .

%.push.arm64: MAKE_IMAGE ?= ${IMAGE_REPOSITORY}/$*:${VERSION}-amd64
%.push.arm64: %.image.amd64
	docker push ${MAKE_IMAGE}

%.image.arm64: MAKE_IMAGE ?= ${IMAGE_REPOSITORY}/$*:${VERSION}-arm64
%.image.arm64:
	@mkdir -p docker/dist/arm64
	@cp docker/service/$*.Dockerfile docker/dist/arm64/$*.Dockerfile
	@sed -i -e '/#alpine-git.Dockerfile/ {' -e 'r docker/base/arm64/alpine-git.Dockerfile' -e 'd' -e '}' docker/dist/arm64/$*.Dockerfile
	@sed -i -e '/#alpine.Dockerfile/ {' -e 'r docker/base/arm64/alpine.Dockerfile' -e 'd' -e '}' docker/dist/arm64/$*.Dockerfile
	@sed -i -e '/#ubuntu-xenial.Dockerfile/ {' -e 'r docker/base/arm64/ubuntu-xenial.Dockerfile' -e 'd' -e '}' docker/dist/arm64/$*.Dockerfile
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -v -o docker/dist/$* cmd/$*/main.go
	@docker build -f docker/dist/arm64/$*.Dockerfile --tag ${MAKE_IMAGE} .

# USING BUILD AND UPLOAD INSTEAD OF IMAGE AND PUSH TO AVOID COLLISION
resource-server.build.amd64: MAKE_IMAGE ?= ${IMAGE_REPOSITORY}/resource-server:${VERSION}-amd64
resource-server.build.amd64:
	@mkdir -p docker/dist/amd64
	@cp docker/service/resource-server.Dockerfile docker/dist/amd64/resource-server.Dockerfile
	@sed -i -e '/#nginx.Dockerfile/ {' -e 'r docker/base/amd64/nginx.Dockerfile' -e 'd' -e '}' docker/dist/amd64/resource-server.Dockerfile
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o docker/dist/reaper cmd/reaper/main.go
	@docker build -f docker/dist/amd64/resource-server.Dockerfile --tag ${MAKE_IMAGE} .

resource-server.upload.amd64: MAKE_IMAGE ?= ${IMAGE_REPOSITORY}/resource-server:${VERSION}-amd64
resource-server.upload.amd64: resource-server.build.amd64
	@docker push ${MAKE_IMAGE}

resource-server.build.arm64: MAKE_IMAGE ?= ${IMAGE_REPOSITORY}/resource-server:${VERSION}-arm64
resource-server.build.arm64:
	@mkdir -p docker/dist/arm64
	@cp docker/service/resource-server.Dockerfile docker/dist/arm64/resource-server.Dockerfile
	@sed -i -e '/#nginx.Dockerfile/ {' -e 'r docker/base/arm64/nginx.Dockerfile' -e 'd' -e '}' docker/dist/arm64/resource-server.Dockerfile
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -v -o docker/dist/reaper cmd/reaper/main.go
	@docker build -f docker/dist/arm64/resource-server.Dockerfile --tag ${MAKE_IMAGE} .

resource-server.upload.arm64: MAKE_IMAGE ?= ${IMAGE_REPOSITORY}/resource-server:${VERSION}-arm64
resource-server.upload.arm64: resource-server.build.arm64
	@docker push ${MAKE_IMAGE}

.PHONY: clean
clean:
	@rm -rf docker/dist
