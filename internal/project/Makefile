GOPATH=$(shell go env GOPATH)
GOLANGCI_LINT_VERSION=latest

all: lint test build

build:
	go build

build-race: ## build with race detactor
	go build -race

build-slim: ## build without symbol and DWARF table, smaller binary but no debugging and profiling ability
	go build -trimpath -ldflags="-s -w"

lint: ## run all the lint tools, install golangci-lint if not exist
ifeq (,$(wildcard $(GOPATH)/bin/golangci-lint))
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) > /dev/null
	$(GOPATH)/bin/golangci-lint run || exit 0
else
	$(GOPATH)/bin/golangci-lint run || exit 0
endif

test: ## run tests with race detactor
	go test -race ./...

clean: ## remove the output binary from go build, as well as go install
	go clean -i

.PHONY: build build-race build-slim lint test clean
