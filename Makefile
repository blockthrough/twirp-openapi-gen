TAG_NAME := $(if $(TAG_NAME),$(TAG_NAME),$(shell git describe --exact-match --tags HEAD 2>/dev/null || :))
BRANCH_NAME := $(if $(BRANCH_NAME),$(BRANCH_NAME),$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || :))
LONG_VERSION := $(shell git describe --tags --long --abbrev=7 --always HEAD)$(shell echo -$(BRANCH_NAME) | tr / - | grep -v '\-master' || :)
VERSION := $(if $(TAG_NAME),$(TAG_NAME),$(LONG_VERSION))

BUF_VERSION := 1.21.0
PROTOC_GEN_GO_VERSION := 1.30.0
TWIRP_VERSION := 8.1.3

## build:
.PHONY: build
build:
	go build -o build/ ./cmd/...

## install:
install:
	go install -ldflags "-X main.version=$(VERSION)" ./cmd/twirp-openapi-gen

## test:
test:
	go test ./internal/generator

## fmt: format the code using goimports
fmt:
	goimports -w $(shell find . -type f -name "*.go" -not -name "*.pb.go" -not -name "*.twirp.go")

## gen: generate go twirp code using buf
gen:
	rm -rf ./internal/generator/testdata/gen && \
	buf generate ./internal/generator/testdata/paymentapis --template ./internal/generator/testdata/paymentapis/buf.gen.yaml && \
 	buf generate ./internal/generator/testdata/petapis --template ./internal/generator/testdata/petapis/buf.gen.yaml

## tools: download tools; buf, protoc-gen-go and protoc-gen-twirp
tools:
	go install github.com/bufbuild/buf/cmd/buf@v$(BUF_VERSION)
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v$(PROTOC_GEN_GO_VERSION)
	go install github.com/twitchtv/twirp/protoc-gen-twirp@v$(TWIRP_VERSION)

## version: print version
version:
	echo $(VERSION)

.PHONY: help
## help: prints this help message
help:
	@echo "Usage: \n"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' |  sed -e 's/^/ /'
