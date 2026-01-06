# OpenAPI Operator Generator

A code generator that creates Kubernetes operators from OpenAPI specifications. It maps REST API resources to Kubernetes Custom Resource Definitions (CRDs) and generates controller reconciliation logic that syncs CRs with the backing REST API.

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Requirements](#requirements)
- [Building](#building)
- [Usage](#usage)
  - [Options](#options)
  - [Example](#example)
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
- [Generated Output](#generated-output)
- [Building the Generated Operator](#building-the-generated-operator)
- [Running the Operator](#running-the-operator)
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
  - [Multi-Endpoint Observation](#multi-endpoint-observation)
  - [Status Fields](#status-fields)
- [Example: Petstore Operator](#example-petstore-operator)
  - [Sample CR](#sample-cr)
- [Environment Variables](#environment-variables)
- [License](#license)

## Features

- Parses OpenAPI 3.0/3.1 specifications
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
│   ├── templates/                 ◀── Go templates
│   │   ├── types.go.tmpl
│   │   ├── controller.go.tmpl
│   │   ├── main.go.tmpl
│   │   └── crd.yaml.tmpl
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

- Go 1.21+ (tested with Go 1.25)
- controller-gen (automatically installed by build scripts)

## Building

```bash
# Build the generator
go build -o bin/openapi-operator-gen ./cmd/openapi-operator-gen/

# Or use the build script with Docker
docker run --rm -v "$(pwd):/app" -w /app golang:1.25 ./scripts/build-example.sh
```

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
| `--spec`, `-s` | Path to OpenAPI specification (YAML or JSON) | Required |
| `--output`, `-o` | Output directory for generated code | Required |
| `--group`, `-g` | Kubernetes API group (e.g., `myapp.example.com`) | Required |
| `--version`, `-v` | API version (e.g., `v1alpha1`) | `v1alpha1` |
| `--module`, `-m` | Go module name for generated code | Required |
| `--mapping` | Resource mapping mode: `per-resource` or `single-crd` | `per-resource` |

### Example

```bash
openapi-operator-gen generate \
  --spec examples/petstore.1.0.27.yaml \
  --output examples/generated \
  --group petstore.example.com \
  --version v1alpha1 \
  --module github.com/bluecontainer/petstore-operator
```

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

## Generated Output

```
output/
├── api/
│   └── v1alpha1/
│       ├── types.go              # CRD Go types (including nested types)
│       └── groupversion_info.go  # API group registration
├── config/
│   └── crd/
│       └── bases/
│           └── *.yaml            # CRD manifests
├── internal/
│   ├── controller/
│   │   └── *_controller.go       # Reconcilers
│   └── endpoint/
│       └── resolver.go           # Endpoint discovery
├── cmd/
│   └── manager/
│       └── main.go               # Operator entrypoint
├── Dockerfile
├── Makefile
└── go.mod
```

## Building the Generated Operator

```bash
cd <output-directory>

# Download dependencies
go mod tidy

# Generate deepcopy methods (requires controller-gen)
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
controller-gen object paths='./...'

# Build
make build
```

## Running the Operator

The generated operator supports multiple modes for discovering the REST API endpoint:

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

| Flag | Description | Default |
|------|-------------|---------|
| `--base-url` | Static REST API base URL | |
| `--statefulset-name` | StatefulSet name for endpoint discovery | |
| `--deployment-name` | Deployment name for endpoint discovery | |
| `--helm-release` | Helm release name for endpoint discovery | |
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
| `externalIDRef` | Reference an existing external resource by ID |
| `readOnly` | If true, only observe the resource (no create/update/delete) |

These fields are stripped from the payload when sending requests to the REST API.

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

## License

Apache License 2.0
