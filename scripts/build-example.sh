#!/bin/bash
set -e

# Build the petstore example operator
# This script can be run directly if Go 1.21+ is installed, or via Docker:
#   docker run --rm -v "$(pwd):/app" -w /app golang:1.21 ./scripts/build-example.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${PROJECT_ROOT}"

echo "=== Building openapi-operator-gen ==="
go build -o bin/openapi-operator-gen ./cmd/openapi-operator-gen/

echo ""
echo "=== Generating petstore operator ==="
./bin/openapi-operator-gen generate \
    --spec examples/petstore.1.0.27.yaml \
    --output examples/generated \
    --group petstore.example.com \
    --version v1alpha1 \
    --module github.com/example/petstore-operator

cd examples/generated

echo ""
echo "=== Adding replace directive for local development ==="
echo 'replace github.com/example/openapi-operator-gen => ../..' >> go.mod

echo ""
echo "=== Running go mod tidy ==="
go mod tidy

echo ""
echo "=== Installing controller-gen ==="
GOTOOLCHAIN=auto go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

echo ""
echo "=== Generating deepcopy methods ==="
"$(go env GOPATH)/bin/controller-gen" object paths='./...'

echo ""
echo "=== Building operator ==="
make build

echo ""
echo "=== Build complete ==="
echo "Binary: examples/generated/bin/manager"
ls -lh bin/manager
