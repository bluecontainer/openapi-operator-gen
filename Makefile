# openapi-operator-gen Makefile

BINARY_NAME=openapi-operator-gen
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse HEAD 2>/dev/null || echo "none")
# Use commit timestamp in UTC (not build time) for Go module pseudo-version generation
DATE?=$(shell date -u -d @$$(git log -1 --date=unix --format=%cd 2>/dev/null) +%Y%m%d%H%M%S 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: all build clean test fmt vet lint install release next-version

all: build

## Build the binary
build: fmt vet
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/openapi-operator-gen

## Build the CEL test utility
build-cel-test: fmt vet
	go build -o bin/cel-test ./cmd/cel-test

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

## Calculate next patch version from latest tag
LATEST_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
NEXT_VERSION := $(shell echo $(LATEST_TAG) | awk -F. '{print $$1"."$$2"."$$3+1}')
RELEASE_VERSION ?= $(NEXT_VERSION)

## Display version information
next-version:
	@echo "Latest tag:   $(LATEST_TAG)"
	@echo "Next version: $(NEXT_VERSION)"

## Create a new release (auto-increments patch version by default)
## Usage: make release                    # Auto-increment patch (v0.1.0 -> v0.1.1)
##        make release RELEASE_VERSION=v1.0.0  # Explicit version
release:
	@echo "Latest tag: $(LATEST_TAG)"
	@echo "Release version: $(RELEASE_VERSION)"
	@read -p "Create release $(RELEASE_VERSION)? [y/N] " confirm && [ "$$confirm" = "y" ] || exit 1
	git tag -a $(RELEASE_VERSION) -m "Release $(RELEASE_VERSION)"
	git push origin $(RELEASE_VERSION)
	@echo "Tag $(RELEASE_VERSION) pushed. GitHub Actions will create the release."

## Show help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  build-cel-test - Build the CEL test utility"
	@echo "  install        - Install to GOPATH/bin"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  fmt            - Format code"
	@echo "  vet            - Run go vet"
	@echo "  clean          - Clean build artifacts"
	@echo "  example        - Run with example petstore spec"
	@echo "  deps           - Download and tidy dependencies"
	@echo "  next-version   - Display latest tag and next version"
	@echo "  release        - Create a GitHub release (auto-increments patch, or set RELEASE_VERSION=vX.Y.Z)"
