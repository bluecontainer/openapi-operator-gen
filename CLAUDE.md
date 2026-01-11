# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Requirements

- **Go version: 1.25** - This project requires Go 1.25. Always use `go-version: '1.25'` in GitHub Actions workflows.

## Build Commands

```bash
# Build the CLI binary (runs fmt and vet first)
make build

# Run all tests
make test

# Run tests with coverage report
make test-coverage

# Run a specific test
go test -v ./pkg/parser -run TestParseOpenAPI

# Format code
make fmt

# Run go vet
make vet

# Generate petstore example operator
make example
```

## Architecture Overview

This is a code generator that creates Kubernetes operators from OpenAPI specifications. It parses REST API specs and generates CRDs with controllers that sync Kubernetes custom resources with the backing REST API.

### Code Flow

1. **Parser** (`pkg/parser/openapi.go`) - Loads and extracts paths/schemas from OpenAPI 3.0/3.1 specs using `kin-openapi`
2. **Mapper** (`pkg/mapper/resource.go`) - Classifies endpoints and maps REST resources to CRD definitions:
   - **Resources**: Paths with multiple HTTP methods (CRUD operations)
   - **QueryEndpoints**: GET-only paths (read-only queries like `/pet/findByStatus`)
   - **ActionEndpoints**: POST/PUT-only paths (one-shot operations like `/pet/{id}/uploadImage`)
3. **Generator** (`pkg/generator/`) - Produces Go code and Kubernetes manifests using templates
4. **Templates** (`pkg/templates/`) - Go templates for all generated artifacts (types, controllers, CRDs, Makefile, Dockerfile)

### Key Packages

- `pkg/endpoint/` - **Shared library** imported by generated operators for runtime endpoint discovery (StatefulSet/Deployment/Helm release discovery, strategy selection, health checking)
- `pkg/runtime/` - URL building utilities shared between generator and generated code
- `internal/config/` - CLI configuration handling

### Generated Operator Structure

Generated operators follow kubebuilder conventions:
- `api/v1alpha1/types.go` - CRD Go types with kubebuilder markers
- `internal/controller/*_controller.go` - Reconciliation logic per resource type
- `config/` - Kustomize manifests (CRDs, RBAC, deployment)

### Endpoint Classification Logic

The mapper classifies paths based on HTTP methods present:
- Has GET + other methods → Resource (full CRUD)
- GET only → QueryEndpoint (periodic queries)
- POST/PUT only (no GET) → ActionEndpoint (one-shot or periodic actions)

### Controller Reconciliation Pattern

Generated controllers use GET-first reconciliation:
1. GET current state from REST API
2. Compare with CR spec (drift detection)
3. CREATE/UPDATE only if needed
4. Finalizers ensure cleanup on CR deletion

### Testing the Generated Operator

After `make example`, the generated operator is in `examples/generated/`. Build and test it:
```bash
cd examples/generated
go mod tidy
make generate manifests
make test
```
