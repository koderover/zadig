.PHONY: microservice.build microservice.image microservice.push microservice.clean

IMAGE_REPOSITORY ?= koderover.tencentcloudcr.com/koderover-demo
IMAGE_REPOSITORY := $(IMAGE_REPOSITORY)
VERSION ?= $(shell date +'%Y%m%d%H%M%S')
VERSION := $(VERSION)
MICROSERVICE_TARGETS = myapp-1 myapp-2 myapp-3

microservice.build: $(MICROSERVICE_TARGETS:=.build)
microservice.image: $(MICROSERVICE_TARGETS:=.image)
microservice.push: $(MICROSERVICE_TARGETS:=.push)
microservice.clean: $(MICROSERVICE_TARGETS:=.clean)

%.build:
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o cmd/$*/$* ./cmd/$*/main.go

%.image: MAKE_IMAGE_TAG ?= ${IMAGE_REPOSITORY}/$*:${VERSION}
%.image: %.build
	docker buildx build -t ${MAKE_IMAGE_TAG} --platform linux/amd64 cmd/$*

%.push: MAKE_IMAGE_TAG ?= ${IMAGE_REPOSITORY}/$*:${VERSION}
%.push: %.build
	docker buildx build -t ${MAKE_IMAGE_TAG} --platform linux/amd64 --push cmd/$*

%.clean:
	rm -f cmd/$*/$*