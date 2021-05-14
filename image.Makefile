# Makefile to release koderover images
# release all images: make allpush
# release reaper: make reaperpush


.DEFAULT: all
.PHONY: all

IMAGE_PREFIX = ccr.ccs.tencentyun.com/koderover-rc/
DATE:=$(shell date +'%Y%m%d%H%M%S')
IMAGE_TAG:=$(shell ./docker/image-tag)
VCS_REF:=$(shell git rev-parse HEAD)
TAG ?= ${DATE}-${IMAGE_TAG}

BASE_TARGETS = aslan warpdrive cron podexec jenkins-plugin


TARGETS =
ifndef ENTERPRISE_TARGETS
TARGETS = $(BASE_TARGETS)
else
TARGETS = $(ENTERPRISE_TARGETS)
endif

ALL_IMAGES=$(TARGETS:=.image)
ALL_PUSHES=$(TARGETS:=.push)


DOCKERFILES=$(addsuffix .Dockerfile,$(addprefix docker/dist/,$(TARGETS)))
DOCKERFILES+=$(addsuffix .Dockerfile,$(addprefix docker/dist/reaper-plugin.,$(REAPER_TARGETS)))

all: $(ALL_IMAGES) $(ALL_REAPER_IMAGES)
allpush: $(ALL_PUSHES) $(ALL_REAPER_PUSHES)
dockerfiles: $(DOCKERFILES)

# make
docker/dist/%.Dockerfile: docker/%.Dockerfile.template docker/ubuntu-base.Dockerfile.inc docker/golang-deps.Dockerfile.inc
	@mkdir -p docker/dist
	@cp docker/$*.Dockerfile.template docker/dist/$*.Dockerfile
	@sed -i -e '/#golang-deps.Dockerfile.inc/ {' -e 'r docker/golang-deps.Dockerfile.inc' -e 'd' -e '}' docker/dist/$*.Dockerfile
	@sed -i -e '/#ubuntu-base.Dockerfile.inc/ {' -e 'r docker/ubuntu-base.Dockerfile.inc' -e 'd' -e '}' docker/dist/$*.Dockerfile
	@echo docker/dist/$*.Dockerfile is generated

%.image: IMAGE ?= ${IMAGE_PREFIX}$*:${TAG}
%.image: docker/dist/%.Dockerfile
	@echo building $*
	docker build -t ${IMAGE} -f docker/dist/$*.Dockerfile .

%.push: IMAGE ?= ${IMAGE_PREFIX}$*:${TAG}
%.push: %.image
	docker push ${IMAGE}


$(TARGETS):%:%.image

.PHONY: clean
clean:
	@rm -rf docker/dist
