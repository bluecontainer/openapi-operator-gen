# OpenAPI Operator Generator

[![Latest Release](https://img.shields.io/github/v/release/bluecontainer/openapi-operator-gen)](https://github.com/bluecontainer/openapi-operator-gen/releases/latest)
[![CI](https://github.com/bluecontainer/openapi-operator-gen/actions/workflows/ci.yaml/badge.svg)](https://github.com/bluecontainer/openapi-operator-gen/actions/workflows/ci.yaml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/bluecontainer/openapi-operator-gen)](https://go.dev/)
[![License](https://img.shields.io/github/license/bluecontainer/openapi-operator-gen)](LICENSE)

A code generator that creates Kubernetes operators from OpenAPI specifications. It maps REST API resources to Kubernetes Custom Resource Definitions (CRDs) and generates controller reconciliation logic that syncs CRs with the backing REST API.

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Requirements](#requirements)
- [Building](#building)
- [Usage](#usage)
  - [Options](#options)
  - [Example](#example)
  - [Swagger 2.0 Support](#swagger-20-support)
- [Update With POST](#update-with-post)
  - [When to Use](#when-to-use)
  - [Usage](#usage-1)
  - [How It Works](#how-it-works)
  - [Behavior by HTTP Method Availability](#behavior-by-http-method-availability)
- [OpenAPI Schema Support](#openapi-schema-support)
  - [Nested Objects](#nested-objects)
  - [Supported Types](#supported-types)
  - [Validation Markers](#validation-markers)
- [Query Endpoint Support](#query-endpoint-support)
  - [How Query Endpoints Are Detected](#how-query-endpoints-are-detected)
  - [Example: Query CRD](#example-query-crd)
  - [Typed Response Mapping](#typed-response-mapping)
  - [Query Status Structure](#query-status-structure)
  - [Query CRD Behavior](#query-crd-behavior)
  - [Query Status Fields](#query-status-fields)
  - [Accessing Results](#accessing-results)
- [Action Endpoint Support](#action-endpoint-support)
  - [How Action Endpoints Are Detected](#how-action-endpoints-are-detected)
  - [Example: Action CRD](#example-action-crd)
  - [One-Shot vs Periodic Execution](#one-shot-vs-periodic-execution)
  - [Periodic Re-Execution](#periodic-re-execution)
  - [Action Status Fields](#action-status-fields)
  - [Action Status Example](#action-status-example)
  - [Multi-Endpoint Action Execution](#multi-endpoint-action-execution)
  - [Typed Results](#typed-results)
  - [Action vs Resource vs Query Endpoints](#action-vs-resource-vs-query-endpoints)
- [Status Aggregator CRD](#status-aggregator-crd)
  - [Enabling the Aggregator CRD](#enabling-the-aggregator-crd)
  - [Supported Resource Types](#supported-resource-types)
  - [Aggregation Strategies](#aggregation-strategies)
  - [Resource Selection Methods](#resource-selection-methods)
  - [Explicit Resource References](#explicit-resource-references)
  - [Dynamic Resource Selectors](#dynamic-resource-selectors)
  - [Combining Selection Methods](#combining-selection-methods)
  - [Aggregating All CRD Types](#aggregating-all-crd-types)
  - [CEL Derived Values](#cel-derived-values)
  - [CEL Variables](#cel-variables)
  - [CEL Aggregate Functions](#cel-aggregate-functions)
  - [Aggregator Status Fields](#aggregator-status-fields)
- [Bundle CRD](#bundle-crd)
  - [Enabling the Bundle CRD](#enabling-the-bundle-crd)
  - [Bundle Spec Structure](#bundle-spec-structure)
  - [Automatic Dependency Derivation](#automatic-dependency-derivation)
  - [Explicit Dependencies](#explicit-dependencies)
  - [Conditional Resource Creation](#conditional-resource-creation)
  - [Ready Conditions](#ready-conditions)
  - [Sync Waves](#sync-waves)
  - [Bundle Examples](#bundle-examples)
  - [Bundle Status Fields](#bundle-status-fields)
- [Generated Output](#generated-output)
- [Building the Generated Operator](#building-the-generated-operator)
- [Running the Operator](#running-the-operator)
  - [No Global Configuration (Per-CR Targeting Only)](#no-global-configuration-per-cr-targeting-only)
  - [1. Static URL Mode](#1-static-url-mode)
  - [2. StatefulSet Discovery Mode](#2-statefulset-discovery-mode)
  - [3. Deployment Discovery Mode](#3-deployment-discovery-mode)
  - [4. Helm Release Discovery Mode](#4-helm-release-discovery-mode)
- [Operator Configuration Flags](#operator-configuration-flags)
- [Endpoint Selection Strategies](#endpoint-selection-strategies)
  - [Using by-ordinal Strategy](#using-by-ordinal-strategy)
  - [Per-CR Workload Targeting](#per-cr-workload-targeting)
  - [Spec Fields Reference](#spec-fields-reference)
- [Discovery Modes](#discovery-modes)
  - [DNS Mode (default for StatefulSet)](#dns-mode-default-for-statefulset)
  - [Pod IP Mode (default for Deployment)](#pod-ip-mode-default-for-deployment)
- [How Reconciliation Works](#how-reconciliation-works)
  - [Importing Existing Resources](#importing-existing-resources)
  - [Read-Only Mode](#read-only-mode)
  - [OnDelete Policy](#ondelete-policy)
  - [Partial Updates](#partial-updates)
  - [Multi-Endpoint Observation](#multi-endpoint-observation)
  - [Status Fields](#status-fields)
- [Example: Petstore Operator](#example-petstore-operator)
  - [Sample CR](#sample-cr)
- [Environment Variables](#environment-variables)
- [Observability (OpenTelemetry)](#observability-opentelemetry)
  - [Enabling OpenTelemetry](#enabling-opentelemetry)
  - [Metrics](#metrics)
  - [Tracing](#tracing)
  - [Kubernetes Deployment](#kubernetes-deployment-with-opentelemetry)
- [Helm Chart Generation](#helm-chart-generation)
- [Releasing](#releasing)
- [License](#license)

## Features

- Parses OpenAPI 3.0/3.1 and Swagger 2.0 specifications (auto-detected)
- Generates Go types for CRDs with kubebuilder markers
- Handles nested schemas and `$ref` references (generates named types)
- Generates CRD YAML manifests
- Generates controller reconciliation logic with full CRUD support
- Supports multiple endpoint discovery modes:
  - Static base URL
  - StatefulSet pod discovery (DNS or Pod IP)
  - Deployment pod discovery (Pod IP)
  - Helm release discovery (auto-detects StatefulSet or Deployment)
- Multiple endpoint selection strategies
- Per-CR workload targeting for multi-tenant scenarios
- Helm chart generation via [helmify](https://github.com/arttor/helmify)
- OpenTelemetry instrumentation for observability

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                           OPENAPI-OPERATOR-GEN ARCHITECTURE                         │
└─────────────────────────────────────────────────────────────────────────────────────┘

                              ┌─────────────────────┐
                              │   OpenAPI Spec      │
                              │  (YAML/JSON)        │
                              │                     │
                              │  paths:             │
                              │    /pets:           │
                              │    /users:          │
                              │    /stores:         │
                              └──────────┬──────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              CODE GENERATOR (CLI)                                   │
│  cmd/openapi-operator-gen                                                           │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                     │
│   ┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐            │
│   │     Parser      │      │     Mapper      │      │    Generator    │            │
│   │  pkg/parser/    │─────▶│  pkg/mapper/    │─────▶│  pkg/generator/ │            │
│   │                 │      │                 │      │                 │            │
│   │ • Load spec     │      │ • REST → CRD    │      │ • types.go      │            │
│   │ • Extract paths │      │ • Schema map    │      │ • controllers   │            │
│   │ • Parse schemas │      │ • Field types   │      │ • CRD YAML      │            │
│   └─────────────────┘      └─────────────────┘      │ • main.go       │            │
│                                                      │ • Dockerfile    │            │
│                                      ┌───────────────┴─────────────────┘            │
│                                      │                                              │
│                                      ▼                                              │
│                            ┌─────────────────┐                                      │
│                            │    Templates    │                                      │
│                            │ pkg/templates/  │                                      │
│                            │                 │                                      │
│                            │ • types.go.tmpl │                                      │
│                            │ • controller.go │                                      │
│                            │ • main.go.tmpl  │                                      │
│                            │ • crd.yaml.tmpl │                                      │
│                            │ • makefile.tmpl │                                      │
│                            │ • dockerfile    │                                      │
│                            │ • example_cr    │                                      │
│                            │ • kustomization │                                      │
│                            └─────────────────┘                                      │
│                                                                                     │
└─────────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         │ generates
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              GENERATED OPERATOR                                     │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                     │
│   api/v1alpha1/              internal/controller/           config/crd/bases/       │
│   ┌──────────────┐           ┌──────────────────┐          ┌──────────────────┐    │
│   │  types.go    │           │ pet_controller   │          │  pets.yaml       │    │
│   │  • PetSpec   │           │ user_controller  │          │  users.yaml      │    │
│   │  • PetStatus │           │ store_controller │          │  stores.yaml     │    │
│   └──────────────┘           └────────┬─────────┘          └──────────────────┘    │
│                                       │                                             │
│                                       │ imports                                     │
│                                       ▼                                             │
│                    ┌──────────────────────────────────────┐                        │
│                    │      Endpoint Resolver Library       │                        │
│                    │   pkg/endpoint/resolver.go           │◀───── SHARED LIBRARY   │
│                    │                                      │                        │
│                    │  • StatefulSet discovery             │                        │
│                    │  • Deployment discovery              │                        │
│                    │  • Helm release discovery            │                        │
│                    │  • Strategy selection                │                        │
│                    │  • Health checking                   │                        │
│                    └──────────────────────────────────────┘                        │
│                                                                                     │
└─────────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         │ at runtime
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                           KUBERNETES CLUSTER (RUNTIME)                              │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                     │
│  ┌─────────────────┐         ┌─────────────────────────────────────────┐           │
│  │  Custom         │         │         Generated Operator              │           │
│  │  Resources      │────────▶│                                         │           │
│  │                 │ watch   │  ┌───────────────────────────────────┐  │           │
│  │  Pet CR         │         │  │       Controller Reconcile        │  │           │
│  │  User CR        │         │  │                                   │  │           │
│  │  Store CR       │         │  │  1. Get CR spec                   │  │           │
│  └─────────────────┘         │  │  2. Resolve endpoint              │  │           │
│                              │  │  3. Call REST API                 │  │           │
│                              │  │  4. Update CR status              │  │           │
│                              │  └───────────────┬───────────────────┘  │           │
│                              └──────────────────│──────────────────────┘           │
│                                                 │                                   │
│                                                 │ HTTP                              │
│                                                 ▼                                   │
│  ┌─────────────────────────────────────────────────────────────────────────────┐   │
│  │                    REST API Workload                                        │   │
│  │                                                                             │   │
│  │   StatefulSet / Deployment / Helm Release                                   │   │
│  │   ┌─────────┐  ┌─────────┐  ┌─────────┐                                    │   │
│  │   │  Pod 0  │  │  Pod 1  │  │  Pod 2  │                                    │   │
│  │   │ :8080   │  │ :8080   │  │ :8080   │                                    │   │
│  │   └─────────┘  └─────────┘  └─────────┘                                    │   │
│  │        ▲            ▲            ▲                                          │   │
│  │        └────────────┼────────────┘                                          │   │
│  │                     │                                                       │   │
│  │              Endpoint Resolver selects pod(s) based on strategy:            │   │
│  │              • round-robin  → distribute across pods                        │   │
│  │              • leader-only  → always pod-0                                  │   │
│  │              • any-healthy  → first healthy pod                             │   │
│  │              • all-healthy  → fan-out to all                                │   │
│  │              • by-ordinal   → specific pod index                            │   │
│  │                                                                             │   │
│  └─────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                     │
└─────────────────────────────────────────────────────────────────────────────────────┘


ENDPOINT DISCOVERY MODES
========================

DNS Mode (StatefulSet)                    Pod IP Mode (Deployment/StatefulSet)
┌─────────────────────────────┐          ┌─────────────────────────────┐
│                             │          │                             │
│  pod-0.svc.ns.svc.cluster   │          │  Query K8s API for pods     │
│  pod-1.svc.ns.svc.cluster   │          │  Use pod.status.podIP       │
│  pod-2.svc.ns.svc.cluster   │          │  10.0.0.1, 10.0.0.2, ...    │
│                             │          │                             │
└─────────────────────────────┘          └─────────────────────────────┘


PROJECT STRUCTURE
=================

openapi-operator-gen/
│
├── cmd/openapi-operator-gen/      ◀── CLI entry point
│   └── main.go
│
├── pkg/
│   ├── parser/                    ◀── OpenAPI spec parser
│   │   └── openapi.go
│   │
│   ├── mapper/                    ◀── REST → CRD mapping
│   │   └── resource.go
│   │
│   ├── generator/                 ◀── Code generation
│   │   ├── types.go
│   │   ├── crd.go
│   │   └── controller.go
│   │
│   ├── templates/                 ◀── Go templates (.tmpl files)
│   │   ├── types.go.tmpl
│   │   ├── controller.go.tmpl
│   │   ├── main.go.tmpl
│   │   ├── crd.yaml.tmpl
│   │   ├── makefile.tmpl
│   │   ├── dockerfile.tmpl
│   │   ├── go.mod.tmpl
│   │   ├── example_cr.yaml.tmpl
│   │   ├── example_cr_ref.yaml.tmpl
│   │   └── kustomization_*.yaml.tmpl
│   │
│   └── endpoint/                  ◀── SHARED LIBRARY (imported by operators)
│       ├── resolver.go
│       └── resolver_test.go           57 unit tests
│
├── internal/config/               ◀── Configuration
│   └── config.go
│
├── examples/
│   ├── petstore.1.0.27.yaml       ◀── Sample OpenAPI spec
│   └── generated/                 ◀── Generated operator output
│
└── scripts/
    └── build-example.sh           ◀── Build script
```

## Requirements

- Go 1.25+
- controller-gen (automatically installed by generated Makefile)
- kustomize (automatically installed by generated Makefile)

## Installation

### Download pre-built binary (easiest)

Download the latest release for your platform from [GitHub Releases](https://github.com/bluecontainer/openapi-operator-gen/releases):

```bash
# Linux (amd64)
curl -sL https://github.com/bluecontainer/openapi-operator-gen/releases/latest/download/openapi-operator-gen-linux-amd64 -o openapi-operator-gen
chmod +x openapi-operator-gen
sudo mv openapi-operator-gen /usr/local/bin/

# Linux (arm64)
curl -sL https://github.com/bluecontainer/openapi-operator-gen/releases/latest/download/openapi-operator-gen-linux-arm64 -o openapi-operator-gen
chmod +x openapi-operator-gen
sudo mv openapi-operator-gen /usr/local/bin/

# macOS (Apple Silicon)
curl -sL https://github.com/bluecontainer/openapi-operator-gen/releases/latest/download/openapi-operator-gen-darwin-arm64 -o openapi-operator-gen
chmod +x openapi-operator-gen
sudo mv openapi-operator-gen /usr/local/bin/

# macOS (Intel)
curl -sL https://github.com/bluecontainer/openapi-operator-gen/releases/latest/download/openapi-operator-gen-darwin-amd64 -o openapi-operator-gen
chmod +x openapi-operator-gen
sudo mv openapi-operator-gen /usr/local/bin/

# Verify installation
openapi-operator-gen --version
```

### Via go install

If you have Go 1.25+ installed:

```bash
go install github.com/bluecontainer/openapi-operator-gen/cmd/openapi-operator-gen@latest
```

### Building from source

```bash
git clone https://github.com/bluecontainer/openapi-operator-gen.git
cd openapi-operator-gen

# Build the generator
make build

# Or directly with go
go build -o bin/openapi-operator-gen ./cmd/openapi-operator-gen/

# Check version
./bin/openapi-operator-gen --version
```

The version includes build metadata (commit hash and build date) when built with `make build`.

## Usage

```bash
openapi-operator-gen generate \
  --spec <path-to-openapi-spec> \
  --output <output-directory> \
  --group <api-group> \
  --version <api-version> \
  --module <go-module-name>
```

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `--spec`, `-s` | Path or URL to OpenAPI specification (YAML or JSON) | Required |
| `--output`, `-o` | Output directory for generated code | Required |
| `--group`, `-g` | Kubernetes API group (e.g., `myapp.example.com`) | Required |
| `--version`, `-v` | API version (e.g., `v1alpha1`) | `v1alpha1` |
| `--module`, `-m` | Go module name for generated code | Required |
| `--mapping` | Resource mapping mode: `per-resource` or `single-crd` | `per-resource` |
| `--root-kind` | Kind name for root `/` endpoint | Derived from spec filename |
| `--include-paths` | Only include paths matching these patterns (comma-separated, glob supported) | All paths |
| `--exclude-paths` | Exclude paths matching these patterns (comma-separated, glob supported) | None |
| `--include-tags` | Only include endpoints with these OpenAPI tags (comma-separated) | All tags |
| `--exclude-tags` | Exclude endpoints with these OpenAPI tags (comma-separated) | None |
| `--include-operations` | Only include operations with these operationIds (comma-separated, glob supported) | All operations |
| `--exclude-operations` | Exclude operations with these operationIds (comma-separated, glob supported) | None |
| `--update-with-post` | Use POST for updates when PUT is not available (see [Update With POST](#update-with-post)) | Disabled |
| `--id-field-map` | Explicit mapping of path params to body fields (e.g., `orderId=id,petId=id`) | Auto-detect |
| `--no-id-merge` | Disable automatic merging of path ID parameters with body 'id' fields | `false` |
| `--aggregate` | Generate a Status Aggregator CRD (see [Status Aggregator CRD](#status-aggregator-crd)) | `false` |
| `--bundle` | Generate an Inline Composition Bundle CRD (see [Bundle CRD](#bundle-crd)) | `false` |

### Example: Local File

```bash
openapi-operator-gen generate \
  --spec examples/petstore.1.0.27.yaml \
  --output examples/generated \
  --group petstore.example.com \
  --version v1alpha1 \
  --module github.com/bluecontainer/petstore-operator
```

### Example: URL

The generator can fetch OpenAPI specs directly from URLs:

```bash
openapi-operator-gen generate \
  --spec https://petstore3.swagger.io/api/v3/openapi.json \
  --output examples/generated \
  --group petstore.example.com \
  --version v1alpha1 \
  --module github.com/bluecontainer/petstore-operator
```

This is useful for generating operators from publicly available API specs or from specs hosted on internal servers.

### Swagger 2.0 Support

The generator automatically detects and converts Swagger 2.0 specifications to OpenAPI 3.0 internally. No additional flags or configuration is needed - just pass your Swagger 2.0 spec file or URL:

```bash
# From a local Swagger 2.0 file
openapi-operator-gen generate \
  --spec swagger.yaml \
  --output examples/generated \
  --group myapp.example.com \
  --version v1alpha1 \
  --module github.com/example/myapp-operator

# From the Swagger Petstore v2 URL
openapi-operator-gen generate \
  --spec https://petstore.swagger.io/v2/swagger.json \
  --output examples/generated \
  --group petstore.example.com \
  --version v1alpha1 \
  --module github.com/example/petstore-operator
```

**Version Detection:**
- Files containing `"swagger": "2.0"` or `swagger: "2.0"` are detected as Swagger 2.0
- Files containing `"openapi": "3.x.x"` or `openapi: "3.x.x"` are detected as OpenAPI 3.x
- Both YAML and JSON formats are supported for either version

**Conversion Notes:**
- Swagger 2.0 specs are converted to OpenAPI 3.0 using the [kin-openapi](https://github.com/getkin/kin-openapi) library
- The conversion handles path parameters, query parameters, request bodies, and response schemas
- Most Swagger 2.0 features map cleanly to OpenAPI 3.0 equivalents

## Update With POST

Some REST APIs use POST for both creating and updating resources, rather than using PUT for updates. The `--update-with-post` flag enables the generated operator to use POST for updates when the API doesn't support PUT.

### When to Use

Use this flag when your API:
- Uses POST for both create and update operations
- Doesn't provide a PUT endpoint for resource updates
- Implements "upsert" semantics with POST (create or update based on existence)

### Usage

The flag accepts:
- `*` - Enable POST-for-updates on all resources that lack PUT
- Comma-separated paths - Enable only for specific endpoints (glob patterns supported)

```bash
# Enable for all resources without PUT
openapi-operator-gen generate \
  --spec api.yaml \
  --output ./generated \
  --group myapp.example.com \
  --version v1alpha1 \
  --update-with-post "*"

# Enable only for specific paths
openapi-operator-gen generate \
  --spec api.yaml \
  --output ./generated \
  --group myapp.example.com \
  --version v1alpha1 \
  --update-with-post "/store/order,/users/*"
```

### How It Works

When `--update-with-post` is enabled for a resource:

1. **GET-first reconciliation**: The controller still fetches the current state from the API
2. **Drift detection**: Compares the CR spec with the fetched resource
3. **POST for updates**: If drift is detected, uses POST (instead of PUT) to update the resource
4. **ID preservation**: The external ID from the initial creation is preserved in subsequent updates

This is particularly useful for APIs like the Petstore `/store/order` endpoint, which only supports POST and GET (no PUT).

### Behavior by HTTP Method Availability

| Has GET | Has POST | Has PUT | `--update-with-post` | Behavior |
|---------|----------|---------|---------------------|----------|
| ✓ | ✓ | ✓ | Any | Uses PUT for updates (standard) |
| ✓ | ✓ | ✗ | Enabled | Uses POST for updates |
| ✓ | ✓ | ✗ | Disabled | Read-only sync (no updates) |
| ✓ | ✗ | ✗ | Any | Read-only sync |
| ✗ | ✓ | ✗ | Any | POST-only (no GET for drift detection) |

## OpenAPI Schema Support

The generator handles various OpenAPI schema patterns:

### Nested Objects

Nested objects and `$ref` references are converted to named Go types:

```yaml
# OpenAPI spec
Pet:
  properties:
    category:
      $ref: '#/components/schemas/Category'
    tags:
      type: array
      items:
        $ref: '#/components/schemas/Tag'
```

Generates:

```go
// PetCategory is a nested type used by CRD specs
type PetCategory struct {
    Id   *int64 `json:"id,omitempty"`
    Name string `json:"name,omitempty"`
}

// PetTagsItem is a nested type used by CRD specs
type PetTagsItem struct {
    Id   *int64 `json:"id,omitempty"`
    Name string `json:"name,omitempty"`
}

type PetSpec struct {
    Category PetCategory   `json:"category,omitempty"`
    Tags     []PetTagsItem `json:"tags,omitempty"`
    // ...
}
```

### Supported Types

| OpenAPI Type | Go Type |
|--------------|---------|
| `string` | `string` |
| `string` (format: date-time) | `*metav1.Time` |
| `string` (format: byte) | `[]byte` |
| `integer` | `int` |
| `integer` (format: int32) | `int32` / `*int32` |
| `integer` (format: int64) | `int64` / `*int64` |
| `number` | `float64` / `*float64` |
| `number` (format: float) | `float32` / `*float32` |
| `boolean` | `bool` |
| `array` | `[]<item-type>` |
| `object` (with properties) | Named struct type |
| `object` (without properties) | `map[string]interface{}` |

### Validation Markers

OpenAPI validation constraints are converted to kubebuilder markers:

- `minLength` / `maxLength` → `+kubebuilder:validation:MinLength/MaxLength`
- `minimum` / `maximum` → `+kubebuilder:validation:Minimum/Maximum`
- `pattern` → `+kubebuilder:validation:Pattern`
- `enum` → `+kubebuilder:validation:Enum`
- `required` → `+kubebuilder:validation:Required`

## Query Endpoint Support

The generator detects and maps query/search endpoints (GET-only paths with query parameters) to dedicated query CRDs. These are useful for endpoints like `/pet/findByTags` or `/pet/findByStatus` that don't follow typical REST resource patterns.

### How Query Endpoints Are Detected

A path is identified as a query endpoint when it has **GET method only** (no POST, PUT, PATCH, or DELETE).

This simple rule means any read-only endpoint becomes a QueryEndpoint:
- `/pet/findByStatus` (GET only) → QueryEndpoint
- `/user/login` (GET only) → QueryEndpoint
- `/api/info` (GET only) → QueryEndpoint
- `/store/inventory` (GET only) → QueryEndpoint

### Example: Query CRD

For an OpenAPI path like:
```yaml
/pet/findByTags:
  get:
    operationId: findPetsByTags
    parameters:
      - name: tags
        in: query
        schema:
          type: array
          items:
            type: string
    responses:
      200:
        content:
          application/json:
            schema:
              type: array
              items:
                $ref: '#/components/schemas/Pet'
```

The generator creates a `PetFindByTags` CRD:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetFindByTags
metadata:
  name: search-friendly-pets
spec:
  tags:
    - friendly
    - cute
  # Optional targeting fields
  targetHelmRelease: petstore-prod
  targetNamespace: production
```

### Typed Response Mapping

Query responses are mapped to typed Go structs. If the response schema references an existing resource type (via `$ref`), that type is reused:

```go
// Response uses existing Pet type (shared)
type PetFindByTagsStatus struct {
    Results   *PetFindByTagsEndpointResponse `json:"results,omitempty"`
    Responses map[string]PetFindByTagsEndpointResponse `json:"responses,omitempty"`
}

type PetFindByTagsEndpointResponse struct {
    Success     bool      `json:"success"`
    StatusCode  int       `json:"statusCode,omitempty"`
    Data        []Pet     `json:"data,omitempty"`  // Reuses Pet type
    Error       string    `json:"error,omitempty"`
    LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}
```

If the response schema doesn't match a known resource, a dedicated result type is generated (e.g., `PetFindByTagsResult`).

### Query Status Structure

Query CRDs use a unified response structure for both single and multi-endpoint modes:

```yaml
status:
  state: Queried
  resultCount: 3
  lastQueryTime: "2026-01-05T10:00:00Z"
  message: "Query executed successfully"

  # Single endpoint result (same structure as multi-endpoint entries)
  results:
    success: true
    statusCode: 200
    data:
      - id: 1
        name: "Fluffy"
        status: "available"
      - id: 2
        name: "Buddy"
        status: "available"
    lastUpdated: "2026-01-05T10:00:00Z"

  # Multi-endpoint responses (only populated with all-healthy strategy)
  responses:
    "http://pod-0.svc:8080":
      success: true
      statusCode: 200
      data: [...]
      lastUpdated: "2026-01-05T10:00:00Z"
    "http://pod-1.svc:8080":
      success: true
      statusCode: 200
      data: [...]
      lastUpdated: "2026-01-05T10:00:00Z"
```

### Query CRD Behavior

- **Periodic execution**: Queries are re-executed on a schedule (default 30 seconds)
- **No finalizers**: Query CRDs don't create external resources, so no cleanup is needed
- **Multi-endpoint fan-out**: With `all-healthy` strategy, queries are sent to all healthy pods
- **Typed results**: Response data is unmarshaled into typed Go structs for easy access

### Query Status Fields

| Field | Description |
|-------|-------------|
| `state` | Current state: `Pending`, `Querying`, `Queried`, or `Failed` |
| `lastQueryTime` | Timestamp of the last query execution |
| `resultCount` | Number of results returned |
| `message` | Human-readable status message |
| `results` | Query result from single endpoint (EndpointResponse) |
| `responses` | Map of endpoint URL to response (multi-endpoint mode) |

### Accessing Results

```go
// Single endpoint
for _, pet := range cr.Status.Results.Data {
    fmt.Printf("Pet: %s\n", pet.Name)
}

// Multi-endpoint
for endpoint, resp := range cr.Status.Responses {
    if resp.Success {
        fmt.Printf("Endpoint %s returned %d pets\n", endpoint, len(resp.Data))
    }
}
```

## Action Endpoint Support

The generator detects and maps action endpoints (operations on a parent resource) to dedicated action CRDs. These are useful for endpoints like `/pet/{petId}/uploadImage` that perform operations on an existing resource rather than CRUD operations.

### How Action Endpoints Are Detected

A path is identified as an action endpoint when it has **POST or PUT method only** (no GET, DELETE, or PATCH):

**Pattern 1: Root endpoint** - `/`
- `/` (POST only) → ActionEndpoint (uses `--root-kind` or derived from filename)

**Pattern 2: Single segment** - `/{action}`
- `/store` (POST only) → ActionEndpoint
- `/login` (POST only) → ActionEndpoint

**Pattern 3: Two segments** - `/{resource}/{action}`
- `/api/echo` (POST only) → ActionEndpoint
- `/user/register` (POST only) → ActionEndpoint

**Pattern 4: With parent resource ID** - `/{resource}/{resourceId}/{action}`
- `/pet/{petId}/uploadImage` (POST only) → ActionEndpoint
- `/user/{userId}/activate` (PUT only) → ActionEndpoint

The key distinction: **Actions never have a GET method**. Any endpoint with GET is either a Resource (if it has other methods) or a QueryEndpoint (if GET-only).

### Root Endpoint Kind Name

For the root `/` endpoint, the Kind name is determined by:
1. The `--root-kind` flag if provided
2. Otherwise, derived from the OpenAPI spec filename (e.g., `petstore.yaml` → `Petstore`)

### Example: Action CRD

For an OpenAPI path like:
```yaml
/pet/{petId}/uploadImage:
  post:
    operationId: uploadFile
    parameters:
      - name: petId
        in: path
        required: true
        schema:
          type: integer
      - name: additionalMetadata
        in: query
        schema:
          type: string
    responses:
      200:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ApiResponse'
```

The generator creates a `PetUploadImage` CRD:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetUploadImage
metadata:
  name: upload-fluffy-photo
spec:
  petId: "12345"              # Parent resource ID (required)
  additionalMetadata: "Profile photo"
  # Optional targeting fields
  targetHelmRelease: petstore-prod
```

### One-Shot vs Periodic Execution

By default, action CRDs implement a one-shot execution pattern:

1. **First Reconcile**: Action is executed, status updated to `Completed` or `Failed`
2. **Subsequent Reconciles**: Action is skipped if already completed and spec unchanged
3. **Spec Change**: If spec is modified (detected via `observedGeneration`), action re-executes
4. **No Requeue on Failure**: Failed actions stay in `Failed` state until spec is updated

### Periodic Re-Execution

Actions can be configured to re-execute periodically by setting `reExecuteInterval`:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetUploadImage
metadata:
  name: periodic-backup
spec:
  petId: "123"
  reExecuteInterval: 5m    # Re-execute every 5 minutes
```

When `reExecuteInterval` is set:
- The action executes immediately on creation
- After completion, it automatically re-executes at the specified interval
- Failed actions also retry at the interval
- Spec changes trigger immediate re-execution regardless of interval
- The `nextExecutionTime` status field shows when the next execution will occur

Supported duration formats: `30s`, `5m`, `1h`, `24h`, etc.

### Action Status Fields

| Field | Description |
|-------|-------------|
| `state` | Current state: `Pending`, `Executing`, `Completed`, or `Failed` |
| `executedAt` | Timestamp when the action was first executed |
| `completedAt` | Timestamp when the action last completed |
| `lastExecutionTime` | Time of the most recent execution (for interval calculation) |
| `nextExecutionTime` | Calculated time of next re-execution (if `reExecuteInterval` is set) |
| `executionCount` | Number of times the action has been executed |
| `httpStatusCode` | HTTP status code from the action response |
| `message` | Human-readable status message |
| `observedGeneration` | Last observed spec generation (for change detection) |
| `result` | Typed result from single endpoint execution |
| `responses` | Map of endpoint URL to response (multi-endpoint mode) |
| `successCount` | Number of successful endpoint executions |
| `totalEndpoints` | Total number of endpoints targeted |

### Action Status Example

```yaml
status:
  state: Completed
  executedAt: "2026-01-05T10:00:00Z"
  completedAt: "2026-01-05T10:05:00Z"
  lastExecutionTime: "2026-01-05T10:05:00Z"
  nextExecutionTime: "2026-01-05T10:10:00Z"  # Only if reExecuteInterval is set
  executionCount: 3
  httpStatusCode: 200
  message: "Action completed successfully"
  observedGeneration: 1
  result:
    success: true
    statusCode: 200
    data:
      code: 200
      type: "unknown"
      message: "additionalMetadata: Profile photo\nFile uploaded"
    executedAt: "2026-01-05T10:05:00Z"
```

### Multi-Endpoint Action Execution

With the `all-healthy` endpoint strategy, actions execute on all healthy pods:

```yaml
status:
  state: Completed
  message: "Action completed on 2/3 endpoints"
  successCount: 2
  totalEndpoints: 3
  responses:
    "http://pod-0.svc:8080":
      success: true
      statusCode: 200
      data: { ... }
      executedAt: "2026-01-05T10:00:01Z"
    "http://pod-1.svc:8080":
      success: true
      statusCode: 200
      data: { ... }
      executedAt: "2026-01-05T10:00:01Z"
    "http://pod-2.svc:8080":
      success: false
      statusCode: 500
      error: "Internal server error"
      executedAt: "2026-01-05T10:00:01Z"
```

### Typed Results

Action responses are mapped to typed Go structs when the response schema is defined:

```go
type PetUploadImageResult struct {
    Code    int    `json:"code,omitempty"`
    Type    string `json:"type,omitempty"`
    Message string `json:"message,omitempty"`
}
```

### Action vs Resource vs Query Endpoints

| Aspect | Resource | Query | Action |
|--------|----------|-------|--------|
| **Detection Rule** | Has multiple HTTP methods | GET only | POST/PUT only (no GET) |
| **Pattern Examples** | `/pet`, `/pet/{id}` | `/pet/findByStatus`, `/api/info` | `/store`, `/api/echo`, `/pet/{id}/upload` |
| **Methods** | GET + POST/PUT/DELETE | GET only | POST or PUT only |
| **Behavior** | CRUD lifecycle | Periodic query | One-shot or periodic execution |
| **States** | Pending, Syncing, Synced, Failed | Pending, Querying, Queried, Failed | Pending, Executing, Completed, Failed |
| **Re-execution** | Drift detection | Scheduled interval | Spec change or `reExecuteInterval` |
| **Finalizer** | Yes (cleanup) | No | Yes (but no cleanup) |

**Classification Examples:**

| Endpoint | Methods | Classification |
|----------|---------|----------------|
| `/store` | POST | ActionEndpoint |
| `/api/echo` | POST | ActionEndpoint |
| `/pet/{petId}/uploadImage` | POST | ActionEndpoint |
| `/user/login` | GET | QueryEndpoint |
| `/api/info` | GET | QueryEndpoint |
| `/pet/findByStatus` | GET | QueryEndpoint |
| `/pet` | GET, POST | Resource |
| `/pet/{petId}` | GET, PUT, DELETE | Resource |

## Status Aggregator CRD

The generator can create an optional Status Aggregator CRD that aggregates status information from multiple resources into a single view. This is useful for monitoring the health of multiple related resources, computing derived values across resources, and creating dashboards.

### Enabling the Aggregator CRD

Add the `--aggregate` flag when generating:

```bash
openapi-operator-gen generate \
  --spec petstore.yaml \
  --output ./generated \
  --group petstore.example.com \
  --version v1alpha1 \
  --module github.com/example/petstore-operator \
  --aggregate
```

This creates a `<AppName>Aggregate` CRD (e.g., `PetstoreAggregate`) that can observe and aggregate status from other CRDs.

### Supported Resource Types

The aggregate CRD can aggregate status from all CRD types generated by the operator:

| CRD Type | Success State | Timestamp Field |
|----------|---------------|-----------------|
| Resource (CRUD) | `Synced` or `Observed` | `lastSyncTime` |
| Query | `Queried` | `lastQueryTime` |
| Action | `Completed` | `lastExecutionTime` |

All CRD types use `Failed` state for failures and `Pending` for initial state.

### Aggregation Strategies

The aggregator supports three strategies for determining overall health:

| Strategy | Description |
|----------|-------------|
| `AllHealthy` | Healthy only if ALL selected resources are in success state (default) |
| `AnyHealthy` | Healthy if ANY selected resource is in success state |
| `Quorum` | Healthy if more than half of selected resources are in success state |

Example:
```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreAggregate
metadata:
  name: all-resources
spec:
  resourceSelectors:
    - kind: Order
    - kind: Pet
    - kind: User
  aggregationStrategy: AllHealthy
```

### Resource Selection Methods

Two methods are available for selecting resources to aggregate:

1. **Explicit References** (`resources`): Reference specific resources by name
2. **Dynamic Selectors** (`resourceSelectors`): Select resources by kind, labels, or patterns

You can use either method or combine both in a single aggregate.

### Explicit Resource References

Reference specific resources by name for precise control:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreAggregate
metadata:
  name: specific-resources
spec:
  resources:
    - kind: Order
      name: my-order
    - kind: Pet
      name: my-pet
      namespace: other-ns  # Optional: defaults to aggregate's namespace
  aggregationStrategy: AllHealthy
```

### Dynamic Resource Selectors

Select resources to aggregate by kind, labels, or name patterns:

```yaml
spec:
  resourceSelectors:
    # Select all resources of a kind
    - kind: Order

    # Filter by labels
    - kind: Pet
      matchLabels:
        environment: production

    # Filter by name pattern (regex)
    - kind: User
      namePattern: "^admin-.*"
```

### Combining Selection Methods

Use both explicit references and dynamic selectors together:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreAggregate
metadata:
  name: mixed-selection
spec:
  # Explicitly include critical resources
  resources:
    - kind: Order
      name: critical-order
  # Also include all resources matching criteria
  resourceSelectors:
    - kind: Pet
      matchLabels:
        tier: backend
  aggregationStrategy: AllHealthy
```

### Aggregating All CRD Types

Include Resource, Query, and Action CRDs in a single aggregate:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreAggregate
metadata:
  name: all-types
spec:
  resourceSelectors:
    # CRUD Resources (state: Synced/Observed)
    - kind: Order
    - kind: Pet
    # Query CRDs (state: Queried)
    - kind: PetFindbystatusQuery
    - kind: StoreInventoryQuery
    # Action CRDs (state: Completed)
    - kind: PetUploadimageAction
  aggregationStrategy: AllHealthy
  derivedValues:
    # Count by CRD type using state
    - name: syncedResources
      expression: "resources.filter(r, r.status.state == 'Synced' || r.status.state == 'Observed').size()"
    - name: queriedCount
      expression: "resources.filter(r, r.status.state == 'Queried').size()"
    - name: completedActions
      expression: "resources.filter(r, r.status.state == 'Completed').size()"
```

### CEL Derived Values

Compute custom values from aggregated resources using [CEL (Common Expression Language)](https://github.com/google/cel-spec) expressions. Results are stored in `.status.computedValues`.

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreAggregate
metadata:
  name: with-metrics
spec:
  resourceSelectors:
    - kind: Order
    - kind: Pet
  aggregationStrategy: AllHealthy
  derivedValues:
    # Calculate percentage of synced resources
    - name: syncPercentage
      expression: "summary.total > 0 ? (summary.synced * 100) / summary.total : 0"

    # Check if all resources are synced
    - name: allSynced
      expression: "summary.total > 0 && summary.synced == summary.total"

    # Count resources in a specific state
    - name: failedResources
      expression: "resources.filter(r, r.status.state == 'Failed').size()"
```

### CEL Variables

CEL expressions have access to the following variables:

| Variable | Type | Description |
|----------|------|-------------|
| `resources` | `list` | All aggregated resources across all kinds |
| `summary` | `map` | Object with `total`, `synced`, `failed`, `pending` counts |
| `<kind>s` | `list` | Kind-specific list (lowercase plural, e.g., `orders`, `pets`, `users`) |

Each resource in the lists contains:

| Field | Description |
|-------|-------------|
| `kind` | Resource kind (e.g., "Order", "Pet") |
| `metadata` | Map with `name`, `namespace`, `labels`, `annotations` |
| `spec` | Full spec fields from the CR |
| `status` | Full status fields from the CR |

Example expressions using different variables:

```yaml
derivedValues:
  # Using resources variable (all kinds)
  - name: totalResources
    expression: "resources.size()"

  # Using summary variable
  - name: healthPercentage
    expression: "summary.total > 0 ? (summary.synced * 100) / summary.total : 0"

  # Using kind-specific variables
  - name: orderCount
    expression: "orders.size()"

  - name: petCount
    expression: "pets.size()"

  # Accessing spec fields
  - name: highValueOrders
    expression: "orders.filter(r, has(r.spec.quantity) && r.spec.quantity > 10).size()"

  # Accessing status fields
  - name: syncedPets
    expression: "pets.filter(r, r.status.state == 'Synced').size()"
```

### CEL Aggregate Functions

Custom aggregate functions are available for computing values across lists:

| Function | Description | Example |
|----------|-------------|---------|
| `sum(list)` | Sum of numeric values | `sum(orders.map(r, r.spec.quantity))` |
| `max(list)` | Maximum value | `max(orders.map(r, r.spec.quantity))` |
| `min(list)` | Minimum value | `min(orders.map(r, r.spec.quantity))` |
| `avg(list)` | Average value | `avg(orders.map(r, r.spec.quantity))` |

Example with aggregate functions:

```yaml
derivedValues:
  # Sum of all order quantities
  - name: totalOrderQuantity
    expression: "sum(orders.map(r, has(r.spec.quantity) ? r.spec.quantity : 0))"

  # Maximum quantity across orders
  - name: maxOrderQuantity
    expression: "max(orders.map(r, has(r.spec.quantity) ? r.spec.quantity : 0))"

  # Average quantity
  - name: avgOrderQuantity
    expression: "avg(orders.map(r, has(r.spec.quantity) ? r.spec.quantity : 0))"

  # Sum of quantities for synced orders only
  - name: syncedOrdersQuantity
    expression: "sum(orders.filter(r, r.status.state == 'Synced').map(r, has(r.spec.quantity) ? r.spec.quantity : 0))"
```

### Aggregator Status Fields

The aggregator status contains:

| Field | Description |
|-------|-------------|
| `state` | `Pending`, `Healthy`, `Degraded`, or `Unknown` |
| `message` | Human-readable status message |
| `summary.total` | Total number of matched resources |
| `summary.synced` | Number of resources in Synced state |
| `summary.failed` | Number of resources in Failed state |
| `summary.pending` | Number of resources in Pending/other states |
| `resources` | List of individual resource statuses |
| `computedValues` | Results of CEL expression evaluations |
| `lastAggregationTime` | Timestamp of last aggregation |

Example status:

```yaml
status:
  state: Degraded
  message: "1 of 3 resources failed"
  summary:
    total: 3
    synced: 2
    failed: 1
    pending: 0
  resources:
    - kind: Order
      name: order-1
      namespace: default
      state: Synced
    - kind: Pet
      name: fluffy
      namespace: default
      state: Synced
    - kind: User
      name: admin
      namespace: default
      state: Failed
      message: "API returned 500"
  computedValues:
    - name: syncPercentage
      value: "66"
    - name: totalOrderQuantity
      value: "150"
  lastAggregationTime: "2026-01-05T10:00:00Z"
```

## Bundle CRD

The generator can create an optional Bundle CRD (Inline Composition) that allows you to define and manage multiple child resources as a single unit. This is useful for deploying related resources together with dependency ordering and lifecycle management.

### Enabling the Bundle CRD

Add the `--bundle` flag when generating:

```bash
openapi-operator-gen generate \
  --spec petstore.yaml \
  --output ./generated \
  --group petstore.example.com \
  --version v1alpha1 \
  --module github.com/example/petstore-operator \
  --bundle
```

This creates a `<AppName>Bundle` CRD (e.g., `PetstoreBundle`) that can create and manage child resources.

### Bundle Spec Structure

A bundle defines embedded resource specs that are created as child CRs:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreBundle
metadata:
  name: my-bundle
spec:
  # Optional: Target settings inherited by child resources
  targetHelmRelease: my-release
  targetNamespace: backend

  # Optional: Pause reconciliation
  paused: false

  # Optional: Sync wave for ordered deployment with other bundles
  syncWave: 0

  # Resource definitions
  resources:
    - id: my-pet           # Unique identifier within the bundle
      kind: Pet            # CRD kind to create
      spec:                # Spec for the child resource
        name: "Fluffy"
        status: available
```

Each resource in the `resources` array creates a child CR with owner references, ensuring automatic cleanup when the bundle is deleted.

### Automatic Dependency Derivation

Bundle CRD automatically derives dependencies from `${resources.<id>...}` variable references in your specs. You don't need to explicitly declare `dependsOn` when using variable references.

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreBundle
metadata:
  name: auto-deps-bundle
spec:
  resources:
    # Parent resource
    - id: parent
      kind: Pet
      spec:
        name: "Parent Pet"

    # Child resource - dependency on 'parent' is automatically derived
    # from the ${resources.parent...} reference below
    - id: child
      kind: Pet
      spec:
        # This reference creates an implicit dependency on 'parent'
        name: "Child of ${resources.parent.status.externalID}"
```

The controller parses all spec fields and CEL expressions to find `${resources.<id>...}` patterns and automatically adds them to the dependency graph. This means:

- References like `${resources.parent.status.externalID}` create a dependency on `parent`
- References in `readyWhen` and `skipWhen` conditions are also parsed
- No explicit `dependsOn` declaration needed for referenced resources

### Explicit Dependencies

You can also explicitly declare dependencies when there's no variable reference but you still need ordering:

```yaml
spec:
  resources:
    - id: database
      kind: Pet
      spec:
        name: "Database"

    - id: app
      kind: Pet
      spec:
        name: "Application"
      dependsOn:
        - database  # Explicit dependency
```

Explicit `dependsOn` and automatic derivation are combined - the final dependency set is the union of both.

### Conditional Resource Creation

Skip resource creation based on CEL conditions:

```yaml
spec:
  resources:
    - id: main
      kind: Pet
      spec:
        name: "Main Resource"

    - id: optional
      kind: Pet
      spec:
        name: "Optional Resource"
      skipWhen:
        - "resources.main.status.state != 'Synced'"  # Skip until main is synced
```

Resources with `skipWhen` conditions that evaluate to `true` are not created.

### Ready Conditions

Define custom conditions for when a resource is considered ready:

```yaml
spec:
  resources:
    - id: backend
      kind: Pet
      spec:
        name: "Backend Service"
      readyWhen:
        - "resources.backend.status.state == 'Synced'"
        - "resources.backend.status.externalID != ''"

    - id: frontend
      kind: Pet
      spec:
        name: "Frontend Service"
      dependsOn:
        - backend  # Wait for backend to be ready
      readyWhen:
        - "resources.frontend.status.state == 'Synced'"
```

Resources are considered ready when all `readyWhen` conditions evaluate to `true`. Dependent resources wait for their dependencies to be ready before creation.

### Sync Waves

Sync waves allow ordering of bundle deployments:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreBundle
metadata:
  name: infrastructure
spec:
  syncWave: -1  # Deploy first (lower numbers first)
  resources:
    - id: database
      kind: Pet
      spec:
        name: "Database"
---
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreBundle
metadata:
  name: application
spec:
  syncWave: 0  # Deploy after infrastructure
  resources:
    - id: app
      kind: Pet
      spec:
        name: "Application"
```

Bundles with lower `syncWave` values are processed first.

### Bundle Examples

#### Simple Bundle

Create multiple resources without explicit dependencies:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreBundle
metadata:
  name: simple-bundle
spec:
  resources:
    - id: pet-1
      kind: Pet
      spec:
        name: "Pet One"
    - id: pet-2
      kind: Pet
      spec:
        name: "Pet Two"
```

#### Bundle with Variable References

Use variable references for automatic dependency ordering:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreBundle
metadata:
  name: reference-bundle
spec:
  resources:
    - id: parent
      kind: Pet
      spec:
        name: "Parent"

    - id: child
      kind: Pet
      spec:
        # Automatic dependency on 'parent' from this reference
        name: "Child of ${resources.parent.status.externalID}"
```

#### Paused Bundle

Suspend reconciliation temporarily:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreBundle
metadata:
  name: paused-bundle
spec:
  paused: true  # No reconciliation until set to false
  resources:
    - id: example
      kind: Pet
      spec:
        name: "Paused Resource"
```

### Bundle Status Fields

The bundle status provides detailed information about child resources:

| Field | Description |
|-------|-------------|
| `state` | Overall state: `Pending`, `Progressing`, `Ready`, `Degraded`, or `Paused` |
| `message` | Human-readable status message |
| `observedGeneration` | Last observed spec generation |
| `summary.total` | Total number of resources in the bundle |
| `summary.ready` | Number of resources in ready state |
| `summary.pending` | Number of pending resources |
| `summary.failed` | Number of failed resources |
| `resources` | Per-resource status details |
| `lastReconcileTime` | Timestamp of last reconciliation |

Example status:

```yaml
status:
  state: Progressing
  message: "2 of 3 resources ready"
  observedGeneration: 1
  summary:
    total: 3
    ready: 2
    pending: 1
    failed: 0
  resources:
    - id: parent
      kind: Pet
      name: my-bundle-parent
      namespace: default
      state: Ready
      message: "Resource synced successfully"
    - id: child
      kind: Pet
      name: my-bundle-child
      namespace: default
      state: Ready
      message: "Resource synced successfully"
    - id: grandchild
      kind: Pet
      name: my-bundle-grandchild
      namespace: default
      state: Pending
      message: "Waiting for dependencies"
  lastReconcileTime: "2026-01-05T10:00:00Z"
```

The bundle controller uses a DAG (Directed Acyclic Graph) to determine the correct creation order based on both explicit `dependsOn` declarations and automatically derived dependencies from variable references.

## Generated Output

```
output/
├── api/
│   └── v1alpha1/
│       ├── types.go              # CRD Go types (including nested types)
│       ├── zz_generated.deepcopy.go  # Generated by controller-gen
│       └── groupversion_info.go  # API group registration
├── config/
│   ├── crd/
│   │   └── bases/
│   │       ├── *.yaml            # CRD manifests (generated by controller-gen)
│   │       └── kustomization.yaml
│   ├── rbac/
│   │   ├── role.yaml             # Generated by controller-gen
│   │   ├── service_account.yaml
│   │   ├── role_binding.yaml
│   │   └── kustomization.yaml
│   ├── manager/
│   │   ├── manager.yaml          # Deployment manifest
│   │   └── kustomization.yaml
│   ├── samples/
│   │   ├── v1alpha1_pet.yaml         # Example CR for creating resources
│   │   ├── v1alpha1_pet_ref.yaml     # Example CR using externalIDRef
│   │   └── kustomization.yaml
│   ├── namespace.yaml
│   └── kustomization.yaml        # Main kustomization (combines all)
├── internal/
│   └── controller/
│       └── *_controller.go       # Reconcilers
├── cmd/
│   └── manager/
│       └── main.go               # Operator entrypoint
├── hack/
│   └── boilerplate.go.txt        # License header for generated code
├── Dockerfile
├── Makefile
└── go.mod
```

### Example CRs

The generator creates example CR files in `config/samples/` for each CRD:

- **`v1alpha1_<kind>.yaml`** - Basic example showing how to create a new resource
- **`v1alpha1_<kind>_ref.yaml`** - Example using `externalIDRef` to import/sync existing resources

Apply examples with:
```bash
kubectl apply -k config/samples/
```

## Building the Generated Operator

```bash
cd <output-directory>

# Download dependencies
go mod tidy

# Generate all code (deepcopy, CRDs, RBAC)
make generate manifests rbac

# Build the binary
make build

# Build the Docker image
make docker-build IMG=myregistry/myoperator:latest
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build the manager binary |
| `make generate` | Generate deepcopy methods |
| `make manifests` | Generate CRD manifests |
| `make rbac` | Generate RBAC manifests (ClusterRole) |
| `make generate-yaml` | Generate all Kubernetes YAML |
| `make docker-build` | Build Docker image |
| `make install` | Install CRDs into cluster |
| `make deploy` | Deploy operator to cluster (uses kustomize) |
| `make undeploy` | Remove operator from cluster |
| `make kind-load` | Load Docker image into kind cluster |
| `make kind-deploy` | Build, load, and deploy to kind cluster |

## Deploying to Kubernetes

The generated operator uses kustomize for deployment. The configuration is organized in `config/`:

```bash
# Deploy to cluster (creates namespace, CRDs, RBAC, deployment)
make deploy IMG=myregistry/myoperator:latest

# Or use kustomize directly
kustomize build config | kubectl apply -f -

# Undeploy
make undeploy
```

### Customizing the Deployment

Edit the kustomization files to customize:

- **`config/manager/manager.yaml`** - Deployment settings, environment variables, resources
- **`config/kustomization.yaml`** - Image name/tag, namespace
- **`config/rbac/`** - RBAC configuration

Example: Set API base URL via environment variable in `config/manager/manager.yaml`:
```yaml
env:
- name: API_BASE_URL
  value: "http://my-api-service:8080"
```

### Deploying to kind (local development)

```bash
# Create a kind cluster
kind create cluster

# Build, load image to kind, and deploy
make kind-deploy IMG=controller:latest
```

## Running the Operator

The generated operator supports multiple modes for discovering the REST API endpoint. **All endpoint configuration is optional** - the operator can start without any endpoint flags, in which case each CR must specify its target via per-CR targeting fields.

### No Global Configuration (Per-CR Targeting Only)

You can run the operator without any endpoint flags:

```bash
./bin/manager
```

In this mode, every CR must specify where to send requests using one of these spec fields:
- `targetHelmRelease` - Target a Helm release
- `targetStatefulSet` - Target a StatefulSet by name
- `targetDeployment` - Target a Deployment by name

Example CR with per-CR targeting:
```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: fluffy
spec:
  name: Fluffy
  targetHelmRelease: petstore-prod   # Required when no global config
  targetNamespace: production         # Optional: defaults to CR namespace
```

If a CR doesn't specify a target and no global configuration exists, reconciliation will fail with:
```
no endpoint configured: set global endpoint (--base-url, --statefulset-name,
--deployment-name, or --helm-release) or specify per-CR targeting
(targetHelmRelease, targetStatefulSet, or targetDeployment)
```

### 1. Static URL Mode

Use a fixed base URL for the REST API:

```bash
./bin/manager --base-url http://api.example.com:8080
```

Environment variable: `REST_API_BASE_URL`

### 2. StatefulSet Discovery Mode

Discover endpoints from pods of a StatefulSet:

```bash
./bin/manager \
  --statefulset-name my-api \
  --namespace default \
  --port 8080
```

Environment variable: `STATEFULSET_NAME`

### 3. Deployment Discovery Mode

Discover endpoints from pods of a Deployment:

```bash
./bin/manager \
  --deployment-name my-api \
  --namespace default \
  --port 8080
```

Environment variable: `DEPLOYMENT_NAME`

Note: Deployments always use pod-ip discovery mode (no stable DNS names). The `leader-only` and `by-ordinal` strategies are not available for Deployments.

### 4. Helm Release Discovery Mode

Discover workload (StatefulSet or Deployment) from a Helm release by looking up resources with the `app.kubernetes.io/instance` label:

```bash
./bin/manager \
  --helm-release my-release \
  --namespace default \
  --port 8080
```

The operator will automatically detect whether the Helm release contains a StatefulSet or Deployment and use the appropriate discovery method.

Environment variable: `HELM_RELEASE`

## Operator Configuration Flags

All endpoint flags are optional. If no global endpoint is configured, each CR must specify its target using per-CR targeting fields (`targetHelmRelease`, `targetStatefulSet`, or `targetDeployment`).

| Flag | Description | Default |
|------|-------------|---------|
| `--base-url` | Static REST API base URL | (optional) |
| `--statefulset-name` | StatefulSet name for endpoint discovery | (optional) |
| `--deployment-name` | Deployment name for endpoint discovery | (optional) |
| `--helm-release` | Helm release name for endpoint discovery | (optional) |
| `--namespace` | Namespace of the workload | Operator namespace |
| `--service` | Headless service name (for DNS mode, StatefulSet only) | StatefulSet name |
| `--port` | REST API port on pods | `8080` |
| `--scheme` | URL scheme (`http` or `https`) | `http` |
| `--strategy` | Endpoint selection strategy | `round-robin` |
| `--discovery-mode` | Discovery mode (`dns` or `pod-ip`) | `dns` for StatefulSet, `pod-ip` for Deployment |
| `--health-path` | Health check path (empty to disable) | `/health` |
| `--workload-kind` | Workload type (`statefulset`, `deployment`, or `auto`) | `auto` |
| `--metrics-bind-address` | Metrics endpoint address | `:8080` |
| `--health-probe-bind-address` | Health probe address | `:8081` |

## Endpoint Selection Strategies

When using workload discovery, you can configure how endpoints are selected:

| Strategy | Description | StatefulSet | Deployment |
|----------|-------------|-------------|------------|
| `round-robin` | Distribute requests evenly across all healthy pods | ✓ | ✓ |
| `leader-only` | Always use pod-0 (ordinal 0) as the primary endpoint | ✓ | ✗ |
| `any-healthy` | Use any single healthy pod, failover if unhealthy | ✓ | ✓ |
| `all-healthy` | Fan-out requests to all healthy pods (for broadcast operations) | ✓ | ✓ |
| `by-ordinal` | Route to a specific pod based on `targetPodOrdinal` field in the CR | ✓ | ✗ |

### Using by-ordinal Strategy

When using `--strategy=by-ordinal`, each CR can specify which pod to target:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: my-pet
spec:
  name: Fluffy
  targetPodOrdinal: 2  # Routes to pod-2
```

### Per-CR Workload Targeting

Each CR can override the operator's global endpoint configuration by specifying a target workload. This allows a single operator instance to manage resources across multiple REST API backends.

**Note:** When the operator is started without any global endpoint configuration, per-CR targeting becomes mandatory - every CR must specify one of these target fields.

#### Target by Helm Release

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: my-pet
spec:
  name: Fluffy
  targetHelmRelease: petstore-prod  # Discovers workload from this Helm release
  targetNamespace: production        # Optional: namespace to look for the Helm release
  targetPodOrdinal: 0                # Optional: specific pod ordinal (StatefulSet only)
```

The controller auto-detects whether the Helm release contains a StatefulSet or Deployment.

#### Target by StatefulSet Name

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: my-pet
spec:
  name: Fluffy
  targetStatefulSet: petstore-api    # Routes to this StatefulSet
  targetNamespace: production         # Optional: namespace
  targetPodOrdinal: 2                 # Optional: specific pod ordinal
```

#### Target by Deployment Name

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: my-pet
spec:
  name: Fluffy
  targetDeployment: petstore-api     # Routes to this Deployment
  targetNamespace: production         # Optional: namespace
```

Note: `targetPodOrdinal` is ignored when targeting a Deployment.

### Spec Fields Reference

Every generated CR includes these controller-specific fields in the spec:

| Field | Description |
|-------|-------------|
| `targetPodOrdinal` | StatefulSet pod ordinal to route requests to (by-ordinal strategy) |
| `targetHelmRelease` | Helm release name for per-CR workload discovery |
| `targetStatefulSet` | StatefulSet name for per-CR workload discovery |
| `targetDeployment` | Deployment name for per-CR workload discovery |
| `targetNamespace` | Namespace for target workload (defaults to CR namespace) |
| `externalIDRef` | Reference an existing external resource by ID (only for CRDs without path parameters) |
| `readOnly` | If true, only observe the resource (no create/update/delete) |
| `mergeOnUpdate` | If true (default), merge spec with current API state before updates (see [Partial Updates](#partial-updates)) |
| `onDelete` | Policy for external resource on CR deletion: `Delete`, `Orphan`, or `Restore` (see [OnDelete Policy](#ondelete-policy)) |

These fields are stripped from the payload when sending requests to the REST API.

**Note:** The `externalIDRef` field is only generated for CRDs where the REST API doesn't use path parameters to identify resources. When path parameters exist (e.g., `/pet/{petId}`), the ID field in the spec (e.g., `id`) serves as the external reference instead. See [Importing Existing Resources](#importing-existing-resources) for details.

#### Targeting Behavior

When a target is specified:
1. If `targetNamespace` is not specified, the CR's namespace is used
2. The global operator endpoint configuration is ignored for this CR
3. The endpoint is discovered dynamically at reconciliation time

This is useful for multi-tenant scenarios where CRs in different namespaces need to target different REST API instances.

## Discovery Modes

### DNS Mode (default for StatefulSet)

Uses Kubernetes DNS to resolve pod addresses:
```
<pod-name>.<service-name>.<namespace>.svc.cluster.local
```

Requires a headless Service (`clusterIP: None`).

### Pod IP Mode (default for Deployment)

Queries the Kubernetes API for pod IPs directly. Useful when DNS is not available or for faster discovery.

```bash
./bin/manager \
  --statefulset-name my-api \
  --discovery-mode pod-ip
```

## How Reconciliation Works

The controller implements a GET-first reconciliation approach with drift detection:

1. **GET First**: Before any create/update, the controller GETs the current state from the REST API
2. **Drift Detection**: Compares the CR spec with the API response to detect drift
3. **Create**: If no external resource exists, POSTs the spec to create one
4. **Update**: If drift is detected, PUTs the updated spec to reconcile
5. **Skip**: If no drift is detected, skips the update (saves API calls)
6. **Delete**: When a CR is deleted, DELETEs the resource from the REST API

The controller uses finalizers to ensure external resources are cleaned up before the CR is removed.

### Importing Existing Resources

You can import an existing external resource by specifying its ID:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: imported-pet
spec:
  externalIDRef: "12345"  # References existing pet in API
  name: Fluffy
  status: available
```

When `externalIDRef` is set:
- The controller GETs the resource by this ID instead of creating a new one
- Drift detection compares your spec with the existing resource
- Updates are applied if the spec differs from the external state

#### When is `externalIDRef` available?

The `externalIDRef` field is only generated for CRDs where the REST API path does **not** contain path parameters to identify the resource. For example:

| API Pattern | Identifier | `externalIDRef` |
|-------------|------------|-----------------|
| `POST /pet` → `GET /pet/{petId}` | `petId` path param | Not needed - use `id` field in spec |
| `POST /user` → `GET /user/{username}` | `username` path param | Not needed - use `username` field in spec |
| `POST /orders` → `GET /orders/{orderId}` | `orderId` path param | Not needed - use `id` field in spec |
| `POST /config` → `GET /config` (no path param) | None in path | **Available** - use `externalIDRef` |

When the REST API uses path parameters like `/pet/{petId}`, the identifier field (e.g., `id`, `petId`) is already part of the spec and serves as the external reference. In these cases, to import an existing resource, simply set the ID field:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: imported-pet
spec:
  id: 12345  # The petId path parameter - references existing pet
  name: Fluffy
  status: available
```

The `externalIDRef` field is only generated when there's no path parameter to identify the resource.

### Read-Only Mode

For observation-only use cases, you can create read-only CRs that never modify the external resource:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: observed-pet
spec:
  externalIDRef: "12345"  # Required for read-only mode
  readOnly: true          # Only observe, never modify
```

When `readOnly: true`:
- The controller only performs GET operations
- No POST, PUT, or DELETE requests are made
- No finalizer is added (CR can be deleted without affecting external resource)
- Status is updated with the current external state
- State will be `Observed` (success) or `NotFound` (resource doesn't exist)

### OnDelete Policy

The `onDelete` field controls what happens to external resources when the CR is deleted. This is especially important for resources adopted via `externalIDRef` that weren't created by the operator.

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: imported-pet
spec:
  externalIDRef: "12345"
  name: Fluffy
  onDelete: Orphan  # Don't delete the external resource
```

#### OnDelete Values

| Value | Description |
|-------|-------------|
| `Delete` | Delete the external resource when the CR is deleted |
| `Orphan` | Leave the external resource unchanged when the CR is deleted |
| `Restore` | Restore the original state captured when the resource was adopted |

#### Default Behavior

If `onDelete` is not specified, the controller uses smart defaults:

| Resource Origin | Default `onDelete` |
|-----------------|-------------------|
| Created by controller (via POST) | `Delete` |
| Adopted via `externalIDRef` | `Orphan` |

This ensures:
- Resources created by the operator are cleaned up when the CR is deleted
- Existing resources that were imported are not accidentally deleted

#### Original State Restoration

When adopting a resource via `externalIDRef`, the controller captures the original state:

```yaml
status:
  createdByController: false
  originalState: {"id": 12345, "name": "Original Name", "status": "available"}
  adoptedAt: "2026-01-05T10:00:00Z"
```

With `onDelete: Restore`, this captured state is restored via PUT (or POST if using `--update-with-post`) before the CR is deleted.

#### Example: Safely Managing Existing Resources

```yaml
# Import an existing resource without risk of deletion
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: production-pet
spec:
  externalIDRef: "prod-12345"
  name: "Updated Name"     # Make changes via reconciliation
  onDelete: Restore        # Restore original state on CR deletion
```

When this CR is deleted:
1. Controller reads the captured `originalState`
2. Sends PUT/POST to restore the original values
3. CR is removed from Kubernetes

### Partial Updates

By default, the controller performs **partial updates** when reconciling resources. This means only the fields you specify in the CR spec are updated, while other fields in the external resource are preserved.

#### How It Works

When `mergeOnUpdate: true` (the default):

1. **GET**: Controller fetches the current state from the REST API
2. **Merge**: Your spec fields are merged with the current API state
3. **Update**: The merged payload is sent via PUT (or POST if using `--update-with-post`)

This prevents accidentally overwriting fields in the external resource that aren't specified in your CR.

#### Example

Suppose the external API has a Pet with this state:

```json
{
  "id": 123,
  "name": "Fluffy",
  "status": "available",
  "category": { "id": 1, "name": "Dogs" },
  "photoUrls": ["http://example.com/photo1.jpg"]
}
```

And your CR only specifies:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: my-pet
spec:
  id: 123
  name: "Fluffy Updated"  # Only changing the name
```

With `mergeOnUpdate: true` (default), the controller will:
1. GET the current state (all fields)
2. Merge your spec (`name: "Fluffy Updated"`) onto it
3. PUT the full merged object, preserving `status`, `category`, and `photoUrls`

Without merging (`mergeOnUpdate: false`), unspecified fields might be sent as zero values, potentially overwriting data in the API.

#### Disabling Merge

To send the spec as-is without merging (full replacement mode):

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: my-pet
spec:
  mergeOnUpdate: false  # Disable merge - send spec as-is
  id: 123
  name: "Fluffy"
  status: "available"
```

Use `mergeOnUpdate: false` when:
- You want full control over all fields
- The API expects complete objects on PUT
- You intentionally want to clear fields not in your spec

#### PATCH Support

When the OpenAPI spec includes a PATCH method for a resource, the generated controller will automatically prefer PATCH over PUT for updates. PATCH inherently performs partial updates, so the `mergeOnUpdate` setting only affects PUT requests.

| HTTP Method | Partial Update Behavior |
|-------------|------------------------|
| PATCH | Always partial (inherent to PATCH semantics) |
| PUT | Partial if `mergeOnUpdate: true` (default) |
| POST (with `--update-with-post`) | Partial if `mergeOnUpdate: true` (default) |

### Multi-Endpoint Observation

When using the `all-healthy` endpoint strategy with read-only mode, the controller queries all healthy endpoints and stores responses from each:

```yaml
status:
  state: Observed
  message: "Successfully observed from 2/3 endpoints"
  externalID: "12345"
  response: { "id": "12345", "name": "Fluffy" }  # First successful response
  responses:
    "http://pod-0.svc:8080":
      success: true
      statusCode: 200
      data: { "id": "12345", "name": "Fluffy" }
      lastUpdated: "2026-01-05T10:30:00Z"
    "http://pod-1.svc:8080":
      success: true
      statusCode: 200
      data: { "id": "12345", "name": "Fluffy" }
      lastUpdated: "2026-01-05T10:30:00Z"
    "http://pod-2.svc:8080":
      success: false
      statusCode: 404
      error: "resource 12345 not found"
      lastUpdated: "2026-01-05T10:30:00Z"
  lastGetTime: "2026-01-05T10:30:00Z"
```

This is useful for:
- Verifying data consistency across replicas
- Monitoring replication lag
- Debugging data synchronization issues

### Status Fields

Each CR has a status subresource with:

| Field | Description |
|-------|-------------|
| `state` | Current state: `Pending`, `Syncing`, `Synced`, `Failed`, `Observed`, or `NotFound` |
| `externalID` | ID of the resource in the external REST API |
| `lastSyncTime` | Timestamp of the last successful sync |
| `lastGetTime` | Timestamp of the last GET request to the REST API |
| `message` | Human-readable status message |
| `conditions` | Standard Kubernetes conditions |
| `response` | Last response body from the REST API (single endpoint) |
| `responses` | Map of endpoint URL to response (multi-endpoint mode) |
| `driftDetected` | Whether drift was detected between spec and external state |
| `createdByController` | Whether the controller created this resource (vs adopted via `externalIDRef`) |
| `originalState` | Captured state when resource was adopted (for `onDelete: Restore`) |
| `adoptedAt` | Timestamp when the resource was first adopted via `externalIDRef` |

#### EndpointResponse Structure (for multi-endpoint mode)

Each CRD generates its own EndpointResponse type (e.g., `PetEndpointResponse`, `UserEndpointResponse`):

| Field | Description |
|-------|-------------|
| `success` | Whether the request to this endpoint succeeded |
| `statusCode` | HTTP status code returned (200, 404, etc.) |
| `data` | Response body from the endpoint |
| `error` | Error message if the request failed |
| `lastUpdated` | When this endpoint was last queried |

## Example: Petstore Operator

The repository includes a petstore example:

```bash
# Generate and build the petstore operator
./scripts/build-example.sh

# Or with Docker (recommended for consistent builds)
docker run --rm -v "$(pwd):/app" -w /app golang:1.25 ./scripts/build-example.sh
```

This generates an operator with `Pet`, `Store`, and `User` CRDs from the OpenAPI petstore spec.

### Sample CR

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: fluffy
spec:
  name: Fluffy
  status: available
  photoUrls:
    - https://example.com/fluffy.jpg
  category:
    id: 1
    name: Dogs
  tags:
    - id: 1
      name: friendly
    - id: 2
      name: cute
```

## Environment Variables

All flags can be set via environment variables:

| Variable | Flag |
|----------|------|
| `REST_API_BASE_URL` | `--base-url` |
| `STATEFULSET_NAME` | `--statefulset-name` |
| `DEPLOYMENT_NAME` | `--deployment-name` |
| `HELM_RELEASE` | `--helm-release` |
| `WORKLOAD_NAMESPACE` | `--namespace` |
| `SERVICE_NAME` | `--service` |

## Observability (OpenTelemetry)

Generated operators include built-in OpenTelemetry instrumentation for distributed tracing and metrics. This provides deep visibility into reconciliation cycles, API calls, and operator health.

### Enabling OpenTelemetry

OpenTelemetry is configured via environment variables. By default, telemetry is disabled. To enable it, set the `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable:

| Variable | Description | Default |
|----------|-------------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint (e.g., `otel-collector:4317`) | (disabled) |
| `OTEL_SERVICE_NAME` | Service name for telemetry | `<app-name>-operator` |
| `OTEL_INSECURE` | Use insecure gRPC connection | `false` |
| `POD_NAME` | Pod name (auto-injected via downward API) | |
| `POD_NAMESPACE` | Pod namespace (auto-injected via downward API) | |

### Metrics

The operator exposes the following metrics via the OpenTelemetry metrics pipeline:

#### Reconciliation Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `reconcile_total` | Counter | `kind`, `result` | Total number of reconciliations |
| `reconcile_duration_seconds` | Histogram | `kind` | Duration of reconciliation cycles |
| `drift_detected_total` | Counter | `kind` | Number of drift detections (spec vs external state) |

#### API Call Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `api_call_total` | Counter | `kind`, `method`, `status` | Total API calls to the REST backend |
| `api_call_duration_seconds` | Histogram | `kind`, `method` | Duration of API calls |

#### Query Controller Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `query_total` | Counter | `kind`, `result` | Total query executions |
| `query_duration_seconds` | Histogram | `kind` | Duration of query operations |

#### Action Controller Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `action_total` | Counter | `kind`, `result` | Total action executions |
| `action_duration_seconds` | Histogram | `kind` | Duration of action operations |

### Tracing

The operator creates spans for key operations:

| Span Name | Description |
|-----------|-------------|
| `Reconcile` | Top-level span for each reconciliation cycle |
| `getResource` | GET request to fetch current external state |
| `createResource` | POST request to create a new resource |
| `updateResource` | PUT request to update an existing resource |
| `deleteFromEndpoint` | DELETE request to remove a resource |
| `syncToEndpoint` | Sync operation to a specific endpoint |
| `executeQueryToEndpoint` | Query execution to a specific endpoint |
| `executeActionToEndpoint` | Action execution to a specific endpoint |

Spans include attributes for:
- `kind`: The CRD kind being reconciled
- `name`: The CR name
- `namespace`: The CR namespace
- `endpoint`: Target REST API endpoint URL
- `method`: HTTP method (GET, POST, PUT, DELETE)
- `status_code`: HTTP response status code

HTTP client requests are automatically instrumented with `otelhttp`, providing detailed request/response tracing.

### Kubernetes Deployment with OpenTelemetry

To enable OpenTelemetry in your deployed operator, configure the environment variables in `config/manager/manager.yaml`:

```yaml
spec:
  containers:
  - name: manager
    env:
    # OpenTelemetry configuration
    - name: OTEL_EXPORTER_OTLP_ENDPOINT
      value: "otel-collector.observability:4317"
    - name: OTEL_SERVICE_NAME
      value: "petstore-operator"
    - name: OTEL_INSECURE
      value: "true"
    # Kubernetes metadata (already configured in generated manifest)
    - name: POD_NAME
      valueFrom:
        fieldRef:
          fieldPath: metadata.name
    - name: POD_NAMESPACE
      valueFrom:
        fieldRef:
          fieldPath: metadata.namespace
```

#### Example: Deploying with Jaeger

```bash
# Deploy Jaeger for local development
kubectl apply -f https://github.com/jaegertracing/jaeger-operator/releases/download/v1.51.0/jaeger-operator.yaml

# Create a Jaeger instance with OTLP receiver
kubectl apply -f - <<EOF
apiVersion: jaegertracing.io/v1
kind: Jaeger
metadata:
  name: jaeger
spec:
  strategy: allInOne
  collector:
    options:
      collector.otlp.enabled: true
      collector.otlp.grpc.host-port: ":4317"
EOF

# Update operator to point to Jaeger
kubectl set env deployment/controller-manager \
  OTEL_EXPORTER_OTLP_ENDPOINT=jaeger-collector:4317 \
  OTEL_INSECURE=true
```

#### Example: Deploying with OpenTelemetry Collector

```yaml
# otel-collector.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-collector-config
data:
  config.yaml: |
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
    processors:
      batch:
    exporters:
      logging:
        loglevel: debug
      # Add your backend exporter (Jaeger, Zipkin, etc.)
    service:
      pipelines:
        traces:
          receivers: [otlp]
          processors: [batch]
          exporters: [logging]
        metrics:
          receivers: [otlp]
          processors: [batch]
          exporters: [logging]
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otel-collector
spec:
  replicas: 1
  selector:
    matchLabels:
      app: otel-collector
  template:
    metadata:
      labels:
        app: otel-collector
    spec:
      containers:
      - name: collector
        image: otel/opentelemetry-collector:latest
        args: ["--config=/conf/config.yaml"]
        volumeMounts:
        - name: config
          mountPath: /conf
      volumes:
      - name: config
        configMap:
          name: otel-collector-config
---
apiVersion: v1
kind: Service
metadata:
  name: otel-collector
spec:
  ports:
  - port: 4317
    name: otlp-grpc
  selector:
    app: otel-collector
```

## Helm Chart Generation

Generated operators include built-in support for creating Helm charts using [helmify](https://github.com/arttor/helmify). This makes it easy to package and distribute your operator.

### Generate a Helm Chart

```bash
cd examples/generated

# Generate Helm chart from kustomize manifests
make helm
```

This creates a complete Helm chart in `chart/<app-name>/` with:
- CRDs in `crds/` directory
- Deployment, ServiceAccount, and RBAC templates
- Configurable `values.yaml`

### Install with Helm

```bash
# Build and push the operator image
make docker-build docker-push IMG=<your-registry>/petstore-operator:latest

# Install using the generated Makefile target
make helm-install IMG=<your-registry>/petstore-operator:latest

# Or install manually with helm
helm install petstore ./chart/petstore \
  -n petstore-system \
  --create-namespace \
  --set image.repository=<your-registry>/petstore-operator \
  --set image.tag=latest
```

### Package for Distribution

```bash
# Package the chart
helm package ./chart/petstore

# Push to a Helm repository (example using OCI registry)
helm push petstore-0.1.0.tgz oci://<your-registry>/charts
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make helm` | Generate Helm chart from kustomize manifests |
| `make helm-install` | Generate and install Helm chart |
| `make helm-upgrade` | Upgrade existing Helm release |
| `make helm-uninstall` | Uninstall Helm release |

## Releasing

To create a new release:

### 1. Update Version and Create Tag

```bash
# Ensure you're on main with latest changes
git checkout main
git pull origin main

# Create an annotated tag
git tag -a v0.1.0 -m "Release v0.1.0"

# Push the tag
git push origin v0.1.0
```

### 2. Automated Release Process

When you push a tag matching `v*`, GitHub Actions automatically:

1. Runs all tests
2. Builds binaries for multiple platforms:
   - Linux (amd64, arm64)
   - macOS (amd64, arm64)
   - Windows (amd64)
3. Creates SHA256 checksums
4. Creates a GitHub Release with all binaries attached

### 3. Version Embedding

The release version is automatically embedded in:

- **CLI binary**: Shown via `openapi-operator-gen --version`
- **Generated go.mod**: Operators generated with a released version will have:
  ```
  require github.com/bluecontainer/openapi-operator-gen v0.1.0
  ```

This ensures generated operators reference the exact version of the generator that created them.

### Release Checklist

- [ ] All tests passing (`make test`)
- [ ] CHANGELOG updated (if maintained)
- [ ] Version follows [Semantic Versioning](https://semver.org/)
- [ ] Tag pushed to trigger release workflow

## License

Apache License 2.0
