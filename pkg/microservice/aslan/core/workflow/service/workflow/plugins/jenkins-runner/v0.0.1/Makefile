IMAGE_REPOSITORY = koderover.tencentcloudcr.com/koderover-public
VERSION = v0.0.2

build: MAKE_IMAGE ?= ${IMAGE_REPOSITORY}/jenkins-runner:${VERSION}
build:
	@go mod init jenkins-runner
	@go mod tidy
	@docker buildx build -t ${MAKE_IMAGE} --platform linux/amd64,linux/arm64 -f Dockerfile --push .