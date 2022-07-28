BUF_VERSION := 1.6.0
PROTOC_GEN_GO_VER := 1.5.2
TWIRP_VER := 8.1.2

## build:
build:
	go build -o build/ ./cmd/...

## test:
test:
	go test ./internal/generator

## fmt: format the code using goimports
fmt:
	goimports -w $(shell find . -type f -name "*.go" -not -name "*.pb.go" -not -name "*.twirp.go")

## gen: generate go twirp code using buf
gen:
	rm -rf ./internal/generator/testdata/gen && \
	buf generate ./internal/generator/testdata/paymentapis  --template ./internal/generator/testdata/paymentapis/buf.gen.yaml && \
 	buf generate ./internal/generator/testdata/petapis  --template ./internal/generator/testdata/petapis/buf.gen.yaml

## tools: download tools; buf, protoc-gen-go and protoc-gen-twirp
tools:
	go install github.com/bufbuild/buf/cmd/buf@v$(BUF_VERSION)
	go install github.com/golang/protobuf/protoc-gen-go@v$(PROTOC_GEN_GO_VER)
	go install github.com/twitchtv/twirp/protoc-gen-twirp@v$(TWIRP_VER)

.PHONY: help
## help: prints this help message
help:
	@echo "Usage: \n"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' |  sed -e 's/^/ /'
