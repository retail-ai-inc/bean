
ARGS="$@"
GOBIN=$(shell pwd)/
GOFILES=$(wildcard *.go)
GONAME=bean
ROOT_DIR=$(shell echo $(cd $(dirname $0);pwd))
GOLANG=$(shell which go)

# Compile and install with race detection and place the binary into root directory as `bean`.
# Enable data race detection. Supported only on linux/amd64, freebsd/amd64, darwin/amd64 and windows/amd64 
build:
	@echo "Building $(GOFILES) to /"
	@GOBIN=$(GOBIN) $(GOLANG) mod download
	@GOBIN=$(GOBIN) $(GOLANG) mod tidy
	@GOBIN=$(GOBIN) $(GOLANG) vet .
	@GOBIN=$(GOBIN) $(GOLANG) build -race -o $(GONAME) $(GOFILES)

# Compile and install with race detection without symbol-DWARF table and place the binary into root directory as `bean`.
# Enable data race detection. Supported only on linux/amd64, freebsd/amd64, darwin/amd64 and windows/amd64 
# To strip the debugging information, use ldflags. It will reduce the binary size.
# -s Omit the symbol table and debug information.
# -w Omit the DWARF symbol table.
slim:
	@echo "Building $(GOFILES) to /"
	@GOBIN=$(GOBIN) $(GOLANG) mod download
	@GOBIN=$(GOBIN) $(GOLANG) mod tidy
	@GOBIN=$(GOBIN) $(GOLANG) vet .
	@GOBIN=$(GOBIN) $(GOLANG) build -ldflags="-s -w" -race -o $(GONAME) $(GOFILES)

lint:
ifeq (,$(wildcard golangci-lint))
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b . latest
	@GOBIN=$(GOBIN) ./golangci-lint run || exit 0
else
	@GOBIN=$(GOBIN) ./golangci-lint run || exit 0
endif

clean:
	@echo "Cleaning entire module cache and remove bean binary"
	@GOBIN=$(GOBIN) $(GOLANG) clean -modcache
	rm -rf bean golangci-lint

debug:
	@dlv --headless --listen=:40000 --api-version=2 exec bean -- -start
    
# Why .PHONY - Let's assume you have install target, which is a very common in makefiles.
# If you do not use .PHONY, and a file named install exists in the same directory as the Makefile,
# then make install will do nothing. This is because Make interprets the rule to mean
# "execute such-and-such recipe to create the file named install". Since the file is already there,
# and its dependencies didn't change, nothing will be done. Generally all targets in your Makefile which
# do not produce an output file with the same name as the target name should be PHONY.
# This typically includes all, install, clean, distclean, and so on :)

.PHONY: build slim lint clean debug
