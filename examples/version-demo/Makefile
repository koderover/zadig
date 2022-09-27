.PHONY: list vet fmt default clean
all: list vet fmt default clean 
BINARY="version"
BUILD=`date +%F`
SHELL := /bin/bash
BASEDIR = $(shell pwd)

# build with verison infos
versionDir="version/utils"
gitTag=$(version_TAG)
gitBranch=$(version_BRANCH)
gitPR=$(version_PR)
buildDate=$(shell TZ=Asia/Shanghai date +%FT%T%z)
gitCommit=$(version_COMMIT_ID)
gitTreeState=$(shell if git status|grep -q 'clean';then echo clean; else echo dirty; fi)
buildURL=$(BUILD_URL)

ldflags="-s -w -X ${versionDir}.gitTag=${gitTag} -X ${versionDir}.buildDate=${buildDate} -X ${versionDir}.gitCommit=${gitCommit} -X ${versionDir}.gitPR=${gitPR} -X ${versionDir}.gitTreeState=${gitTreeState} -X ${versionDir}.version=${VERSION} -X ${versionDir}.gitBranch=${gitBranch} -X ${versionDir}.buildURL=${buildURL}"


PACKAGES=`go list ./... | grep -v /vendor/`
VETPACKAGES=`go list ./... | grep -v /vendor/ | grep -v /examples/`
GOFILES=`find . -name "*.go" -type f -not -path "./vendor/*"`

default:
	@echo "build the ${BINARY}"
	@GOOS=linux GOARCH=amd64 go build -ldflags ${ldflags} -o  build/${BINARY}.linux  -tags=jsoniter
	@go build -ldflags ${ldflags} -o  build/${BINARY}.mac  -tags=jsoniter
	@echo "build done."

list:
	@echo ${PACKAGES}
	@echo ${VETPACKAGES}
	@echo ${GOFILES}

fmt:
	@echo "fmt the project"
	@gofmt -s -w ${GOFILES}

vet:
	@echo "check the project codes."
	@go vet $(VETPACKAGES)
	@echo "check done."


clean:
	@rm -rf build/*

