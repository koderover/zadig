IMAGE_REPOSITORY = koderover.tencentcloudcr.com/koderover-public
VERSION = v0.0.1

build: MAKE_IMAGE ?= ${IMAGE_REPOSITORY}/jenkins-runner:${VERSION}
build:
	@docker buildx build -t ${MAKE_IMAGE} --platform linux/amd64,linux/arm64 -f Dockerfile --push .