GOPATH=$(shell go env GOPATH)
# GOLANGCI_LINT_VERSION is the latest version of golangci-lint
# adjusted to match the Go version used in this project
GOLANGCI_LINT_VERSION=v1.59.1

all: lint test build

build:
	go build ./cmd/bean

build-race: ## build with race detactor
	go build -race ./cmd/bean

build-slim: ## build without symbol and DWARF table, smaller binary but no debugging and profiling ability
	go build -ldflags="-s -w" ./cmd/bean

install: ## install the binary to $GOPATH/bin
	go install ./cmd/bean

lint: ## run all the lint tools, install golangci-lint if not exist
ifeq (,$(wildcard $(GOPATH)/bin/golangci-lint))
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) > /dev/null
	$(GOPATH)/bin/golangci-lint run
else
	$(GOPATH)/bin/golangci-lint run
endif

test: ## run tests with race detactor
	go test -v -race ./...

clean: ## remove the output binary from go build, as well as go install and build cache
	go clean -i -r -cache

.PHONY: build build-race build-slim lint test clean

