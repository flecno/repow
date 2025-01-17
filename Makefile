-include .env

VERSION ?= master
PROJECTNAME := $(shell basename "$(PWD)")

# Use linker flags to provide version/build settings
LDFLAGS=-ldflags "-X=main.VersionLdFlag=$(VERSION)"

GOCMD=go
GOBUILD=$(GOCMD) build $(LDFLAGS)
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

GO_MAIN=cmd/app/main.go
BINARY_PATH=bin
BINARY_NAME=repow
BINARY_LINUX_64=$(BINARY_NAME)_linux-amd64
BINARY_LINUX_ARM64=$(BINARY_NAME)-linux-arm64
BINARY_DARWIN_64=$(BINARY_NAME)_darwin-amd64

DOCKER_IMAGE_NAME=repow

## install: Install missing dependencies. Runs `go get` internally. e.g; make install get=github.com/foo/bar
#install: go-get

all: test build

clean:
	@$(GOCLEAN)
	@rm -rf $(BINARY_PATH)/

test:
	$(GOTEST) -v ./...

build: build-linux64 build-linuxarm build-osx
build-linux64:
	@mkdir -p $(BINARY_PATH)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -v -o $(BINARY_PATH)/$(BINARY_LINUX_64) $(GO_MAIN)
build-linuxarm:
	@mkdir -p $(BINARY_PATH)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) -v -o $(BINARY_PATH)/$(BINARY_LINUX_ARM64) $(GO_MAIN)
build-osx:
	@mkdir -p $(BINARY_PATH)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) -v -o $(BINARY_PATH)/$(BINARY_DARWIN_64) $(GO_MAIN)
docker-build:
	docker build . -t $(DOCKER_IMAGE_NAME):$(VERSION)
docker-run:
	docker run -it --rm -p 8080:8080 --name repow $(DOCKER_IMAGE_NAME):$(VERSION)






# potential make alternative: https://taskfile.dev
# cons: requires binary to be downloaded/installed

# potental dockerfile alternative: https://buildpacks.io/
