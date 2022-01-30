GOPATH=$(shell go env GOPATH)
GOLANGCI_LINT_VERSION=latest

all: lint test build

.PHONY: build
build:
	go build

.PHONY: build-race
build-race: ## build with race detactor
	go build -race

.PHONY: build-slim
build-slim: ## build without symbol and DWARF table, smaller binary but no debugging and profiling ability
	go build -ldflags="-s -w -trimpath"

.PHONY: lint
lint: ## run all the lint tools, install golangci-lint if not exist
ifeq (,$(wildcard $(GOPATH)/bin/golangci-lint))
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) > /dev/null
	golangci-lint run
else
	golangci-lint run
endif

.PHONY: test
test: ## run tests with race detactor
	go test -race ./...

.PHONY: clean
clean: ## remove the output binary from go build, as well as go install
	go clean -i

