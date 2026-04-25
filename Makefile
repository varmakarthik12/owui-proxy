BINARY     := owui-proxy
MODULE     := github.com/varmakarthik12/owui-proxy
BIN_DIR    := bin
VERSION    := $(shell git describe --tags --always 2>/dev/null || echo "dev")
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "unknown")

LDFLAGS := -ldflags "\
  -s -w \
  -X main.Version=$(VERSION) \
  -X main.Commit=$(COMMIT) \
  -X main.BuildDate=$(BUILD_DATE)"

.PHONY: all build test lint fmt install clean release-dry help

all: build

## build: Compile the binary to ./bin/owui-proxy
build:
	@mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY) .

## test: Run all tests with race detection and coverage
test:
	go test ./... -race -cover -timeout 60s

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## fmt: Format and organize imports
fmt:
	gofmt -w .
	goimports -w .

## vet: Run go vet
vet:
	go vet ./...

## install: Install the binary with ldflags
install:
	go install $(LDFLAGS) .

## release-dry: Dry-run goreleaser
release-dry:
	goreleaser release --snapshot --clean

## clean: Remove build artifacts
clean:
	rm -rf $(BIN_DIR)

## tidy: Tidy go modules
tidy:
	go mod tidy

## help: Show this help
help:
	@echo "Usage: make [target]"
	@echo ""
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' 2>/dev/null || \
		sed -n 's/^## //p' $(MAKEFILE_LIST)
