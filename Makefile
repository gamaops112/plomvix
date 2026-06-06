.PHONY: run build test test-verbose vet lint tidy clean coverage ui-test integration-test check help

CGO_ENABLED  ?= 1
export CGO_ENABLED

ROCKSDB_LOCAL  ?= $(PWD)/.rocksdb
ROCKSDB_LIBDIR  = $(ROCKSDB_LOCAL)/usr/lib/x86_64-linux-gnu
C_INCLUDE_PATH  ?= $(ROCKSDB_LOCAL)/usr/include
CGO_LDFLAGS     ?= -L$(ROCKSDB_LIBDIR) -lrocksdb -lgflags -lstdc++ -lm -lz -lsnappy -llz4 -lzstd -lbz2 -Wl,--disable-new-dtags -Wl,-rpath,$(ROCKSDB_LIBDIR)
export C_INCLUDE_PATH
export CGO_LDFLAGS

VERSION      ?= 0.1.0
BINARY        = plomvix
BUILD_TIME   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LD_FLAGS_INNER = -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)
LDFLAGS       = -ldflags "$(LD_FLAGS_INNER)"

## run: Run Plomvix without building a binary
run:
	go run $(LDFLAGS) ./cmd/plomvix

## build: Build the Plomvix binary and the React UI
build: obs-build
	go build $(LDFLAGS) -o $(BINARY) ./cmd/plomvix

## test: Run all tests with race detector and coverage
test:
	go test -race -cover ./...

## test-verbose: Run all tests with verbose output
test-verbose:
	go test -race -cover -v ./...

## vet: Run go vet static analysis
vet:
	go vet ./...

## lint: Run golangci-lint (install: https://golangci-lint.run/usage/install)
lint:
	golangci-lint run ./...

## tidy: Tidy go modules
tidy:
	go mod tidy

## clean: Remove binary and coverage output
clean:
	rm -f $(BINARY) coverage.out coverage.html

## coverage: Generate HTML coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report written to coverage.html"

## ui-install: Install UI npm dependencies
ui-install:
	cd ui && npm install

## ui-dev: Start Vite development server on port 3000
ui-dev:
	cd ui && npm run dev

## ui-build: Build the React app into ui/dist/
ui-build:
	cd ui && npm run build

## obs-dev: Start obs_theme Vite dev server on port 3000
obs-dev:
	cd obs_theme && npm run dev

## obs-build: Build obs_theme into obs_theme/dist/
obs-build:
	cd obs_theme && npm install && npm run build

## ui-test: Run frontend tests
ui-test:
	cd ui && npm run test

## integration-test: Run integration tests
integration-test:
	export C_INCLUDE_PATH=$(C_INCLUDE_PATH) && export LD_LIBRARY_PATH=$(ROCKSDB_LIBDIR) && CGO_ENABLED=1 go test -race ./tests/integration/...

## check: Run all checks (lint, vet, tests, build)
check: lint vet test ui-test ui-build

## help: Show available make commands
help:
	@grep -E '^## ' Makefile | sed 's/## //'
