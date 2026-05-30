.PHONY: run build test test-verbose vet lint tidy clean coverage help

VERSION      ?= 0.1.0
BINARY        = plomvix
BUILD_TIME   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LD_FLAGS_INNER = -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)
LDFLAGS       = -ldflags "$(LD_FLAGS_INNER)"

## run: Run Plomvix without building a binary
run:
	go run $(LDFLAGS) ./cmd/plomvix

## build: Build the Plomvix binary with version injected
build:
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

## help: Show available make commands
help:
	@grep -E '^## ' Makefile | sed 's/## //'
