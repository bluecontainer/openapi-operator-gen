# openapi-operator-gen Makefile

BINARY_NAME=openapi-operator-gen
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: all build clean test fmt vet lint install

all: build

## Build the binary
build: fmt vet
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/openapi-operator-gen

## Install the binary to GOPATH/bin
install: build
	go install $(LDFLAGS) ./cmd/openapi-operator-gen

## Run tests
test:
	go test -v ./...

## Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

## Format code
fmt:
	go fmt ./...

## Run go vet
vet:
	go vet ./...

## Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

## Run with example
example: build
	./bin/$(BINARY_NAME) generate \
		--spec examples/petstore.1.0.27.yaml \
		--output examples/generated \
		--group petstore.example.com \
		--version v1alpha1 \
		--module github.com/bluecontainer/petstore-operator \
		--aggregate \
		--bundle \
		--exclude-operations="updatePetWithForm" \
		--update-with-post="/store/order"
	echo 'replace github.com/bluecontainer/openapi-operator-gen => ../..' >> examples/generated/go.mod


## Download dependencies
deps:
	go mod download
	go mod tidy

## Show help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  install        - Install to GOPATH/bin"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  fmt            - Format code"
	@echo "  vet            - Run go vet"
	@echo "  clean          - Clean build artifacts"
	@echo "  example        - Run with example petstore spec"
	@echo "  deps           - Download and tidy dependencies"
