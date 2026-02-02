# OpenAPI Operator Generator

[![Latest Release](https://img.shields.io/github/v/release/bluecontainer/openapi-operator-gen)](https://github.com/bluecontainer/openapi-operator-gen/releases/latest)
[![CI](https://github.com/bluecontainer/openapi-operator-gen/actions/workflows/ci.yaml/badge.svg)](https://github.com/bluecontainer/openapi-operator-gen/actions/workflows/ci.yaml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/bluecontainer/openapi-operator-gen)](https://go.dev/)
[![License](https://img.shields.io/github/license/bluecontainer/openapi-operator-gen)](LICENSE)

A code generator that creates Kubernetes operators from OpenAPI specifications. It maps REST API resources to Kubernetes Custom Resource Definitions (CRDs) and generates controller reconciliation logic that syncs CRs with the backing REST API.

**Why use this approach?** See [Benefits of REST API to Kubernetes Operator Mapping](docs/benefits.md) for a detailed explanation of the value proposition, architectural benefits, and system architectures this enables.

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
  - [CEL DateTime Functions](#cel-datetime-functions)
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
  - [Leader Election](#leader-election)
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
  - [kstatus Compatibility](#kstatus-compatibility)
- [Example: Petstore Operator](#example-petstore-operator)
  - [Running with Docker Compose](#running-with-docker-compose)
    - [Generated Docker Compose](#generated-docker-compose)
    - [Target API Deployment Manifest](#target-api-deployment-manifest)
  - [Sample CR](#sample-cr)
- [Environment Variables](#environment-variables)
- [Observability (OpenTelemetry)](#observability-opentelemetry)
  - [Enabling OpenTelemetry](#enabling-opentelemetry)
  - [Metrics](#metrics)
  - [Tracing](#tracing)
  - [Kubernetes Deployment](#kubernetes-deployment-with-opentelemetry)
- [Helm Chart Generation](#helm-chart-generation)
- [Kubectl Plugin](#kubectl-plugin)
  - [Installing the Plugin](#installing-the-plugin)
  - [Plugin Commands](#plugin-commands)
  - [Phase 1: Core Commands](#phase-1-core-commands)
  - [Phase 2: Diagnostic Commands](#phase-2-diagnostic-commands)
  - [Phase 3: Interactive Commands](#phase-3-interactive-commands)
  - [Endpoint Targeting Flags](#endpoint-targeting-flags)
  - [TTL-Based Patches](#ttl-based-patches)
- [Rundeck Project](#rundeck-project)
  - [Generated Structure](#generated-structure)
  - [Job Types](#job-types)
  - [Docker Execution Project](#docker-execution-project)
  - [Docker Compose Integration](#docker-compose-integration)
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
- Generated kubectl plugin for operator management with endpoint targeting flags
- Generated Rundeck projects with job definitions for web-based operator management (script-based and Docker execution variants)
- Generated Docker Compose for local development (k3s, with-k8s, k3s-deploy profiles)
- Optional target API deployment manifest generation (`--target-api-image`)
- Leader election RBAC for kustomize and Helm chart deployments

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

### Configuration File

Instead of passing all options via CLI flags, you can use a YAML configuration file. This is especially useful for complex configurations or when you want to version control your generator settings.

**Create an example config file:**

```bash
openapi-operator-gen init
```

This creates `.openapi-operator-gen.yaml` with all available options documented.

**Using the config file:**

```bash
# Auto-discovery (searches for .openapi-operator-gen.yaml in current directory)
openapi-operator-gen generate

# Explicit config file
openapi-operator-gen generate --config myconfig.yaml
```

**Example configuration file:**

```yaml
# openapi-operator-gen.yaml
spec: ./api/openapi.yaml
output: ./generated
group: myapp.example.com
version: v1alpha1
module: github.com/myorg/myapp-operator

# Enable aggregate and bundle CRDs
aggregate: true
bundle: true

# Filter specific endpoints
filters:
  includePaths:
    - /users
    - /pets/*
  excludeTags:
    - deprecated
    - internal

# ID field merging
idMerge:
  fieldMap:
    orderId: id
    petId: id
```

**Precedence:** CLI flags always override config file values.

**Auto-discovery locations** (checked in order):
1. `.openapi-operator-gen.yaml`
2. `.openapi-operator-gen.yml`
3. `openapi-operator-gen.yaml`
4. `openapi-operator-gen.yml`

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `--config`, `-c` | Path to YAML config file | Auto-discover |
| `--spec`, `-s` | Path or URL to OpenAPI specification (YAML or JSON) | Required* |
| `--output`, `-o` | Output directory for generated code | `./generated` |
| `--group`, `-g` | Kubernetes API group (e.g., `myapp.example.com`) | Required* |
| `--version`, `-v` | API version (e.g., `v1alpha1`) | `v1alpha1` |
| `--module` | Go module name for generated code | `github.com/bluecontainer/generated-operator` |
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
| `--kubectl-plugin` | Generate a kubectl plugin for operator management (see [Kubectl Plugin](#kubectl-plugin)) | `false` |
| `--rundeck-project` | Generate a Rundeck project with jobs using the kubectl plugin (requires `--kubectl-plugin`; see [Rundeck Project](#rundeck-project)) | `false` |
| `--target-api-image` | Container image for target REST API (generates Deployment+Service manifest and Docker Compose target API sections) | None |
| `--target-api-port` | Container port for target REST API (overrides port from spec URL) | `8080` |

*Required flags can be provided via config file instead of CLI.

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

The controller behavior adapts based on available HTTP methods:

| Has GET | Has POST | Has PUT | Has PATCH | Has DELETE | Behavior |
|---------|----------|---------|-----------|------------|----------|
| ✓ | ✓ | ✓ | ✓ | ✓ | Full CRUD: POST creates, GET detects drift, PATCH updates, DELETE removes |
| ✓ | ✓ | ✓ | ✗ | ✓ | Full CRUD: POST creates, GET detects drift, PUT updates, DELETE removes |
| ✓ | ✓ | ✗ | ✓ | ✓ | Full CRUD: POST creates, GET detects drift, PATCH updates, DELETE removes |
| ✓ | ✓ | ✓ | ✗ | ✗ | Create/Update: POST creates, PUT updates, no cleanup on CR deletion |
| ✓ | ✓ | ✗ | ✗ | ✓ | Create/Delete: POST creates, no drift correction, DELETE removes |
| ✓ | ✓ | ✗ | ✗ | ✗ | Create-only: POST creates, read-only after creation |
| ✓ | ✗ | ✓ | ✗ | ✓ | Adopt/Update: Reference existing via path params, PUT updates, DELETE removes |
| ✓ | ✗ | ✗ | ✗ | ✗ | Read-only: Only GET and status updates, no mutations |
| ✗ | ✓ | ✗ | ✗ | ✗ | POST-only: Creates resources but cannot verify state (no GET) |

**Update preference**: When both PUT and PATCH are available, PATCH is preferred as it performs partial updates.

**`--update-with-post` flag**: When enabled and neither PUT nor PATCH is available, POST is used for updates (for APIs that use POST for upsert operations).

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
  target:
    helmRelease: petstore-prod
    namespace: production
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
  target:
    helmRelease: petstore-prod
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

Compute custom values from aggregated resources using [CEL (Common Expression Language)](https://cel.dev/) expressions. Results are stored in `.status.computedValues`.

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

### CEL DateTime Functions

DateTime functions are available for working with timestamps and durations in CEL expressions:

| Function | Description | Example |
|----------|-------------|---------|
| `now()` | Current UTC time in RFC3339 format | `now()` → `"2026-01-23T08:00:00Z"` |
| `nowUnix()` | Current Unix timestamp (seconds) | `nowUnix()` → `1737619200` |
| `nowUnixMilli()` | Current Unix timestamp (milliseconds) | `nowUnixMilli()` → `1737619200000` |
| `formatTime(unixSecs, layout)` | Format Unix timestamp with Go layout | `formatTime(1706000000, "2006-01-02")` → `"2024-01-23"` |
| `formatTimeRFC3339(unixSecs)` | Format Unix timestamp as RFC3339 | `formatTimeRFC3339(1706000000)` → `"2024-01-23T08:53:20Z"` |
| `parseTime(timeStr, layout)` | Parse time string to Unix timestamp | `parseTime("2024-01-23", "2006-01-02")` → `1705968000` |
| `parseTimeRFC3339(timeStr)` | Parse RFC3339 to Unix timestamp | `parseTimeRFC3339("2024-01-23T08:53:20Z")` → `1706000000` |
| `addDuration(timeStr, duration)` | Add duration to RFC3339 time | `addDuration("2024-01-23T08:00:00Z", "1h")` → `"2024-01-23T09:00:00Z"` |
| `timeSince(timeStr)` | Seconds since the given RFC3339 time | `timeSince("2024-01-23T08:00:00Z")` → `3600` |
| `timeUntil(timeStr)` | Seconds until the given RFC3339 time | `timeUntil("2024-01-24T08:00:00Z")` → `86400` |
| `durationSeconds(durationStr)` | Parse duration string to seconds | `durationSeconds("1h30m")` → `5400` |

Duration format uses Go's duration syntax: `"1h"`, `"30m"`, `"90s"`, `"1h30m45s"`, `"-1h"` (negative).

Example with datetime functions:

```yaml
derivedValues:
  # Check if any resource is older than 24 hours
  - name: hasStaleResources
    expression: "resources.exists(r, has(r.status.lastSyncTime) && timeSince(r.status.lastSyncTime) > durationSeconds('24h'))"

  # Calculate expiry time (24 hours from now)
  - name: expiryTime
    expression: "addDuration(now(), '24h')"

  # Format current time for logging
  - name: currentDate
    expression: "formatTime(nowUnix(), '2006-01-02 15:04:05')"
```

In Bundle specs, datetime functions can set timestamps:

```yaml
spec:
  resources:
    - id: order
      kind: Order
      spec:
        shipDate: ${now()}
        expiresAt: ${addDuration(now(), "72h")}
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
  target:
    helmRelease: my-release
    namespace: backend

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
├── chart/
│   └── <app>/                    # Helm chart (generated by helmify)
│       ├── templates/
│       │   └── leader-election-rbac.yaml  # Leader election Role/RoleBinding
│       └── ...
├── config/
│   ├── crd/
│   │   └── bases/
│   │       ├── *.yaml            # CRD manifests (generated by controller-gen)
│   │       └── kustomization.yaml
│   ├── rbac/
│   │   ├── role.yaml             # Generated by controller-gen
│   │   ├── service_account.yaml
│   │   ├── role_binding.yaml
│   │   ├── leader_election_role.yaml          # Leader election Role
│   │   ├── leader_election_role_binding.yaml  # Leader election RoleBinding
│   │   └── kustomization.yaml
│   ├── manager/
│   │   ├── manager.yaml          # Deployment manifest
│   │   └── kustomization.yaml
│   ├── target-api/               # Only with --target-api-image
│   │   └── deployment.yaml       # Target API Deployment + Service
│   ├── samples/
│   │   ├── v1alpha1_pet.yaml         # Example CR for creating resources
│   │   ├── v1alpha1_pet_ref.yaml     # Example CR using externalIDRef
│   │   └── kustomization.yaml
│   ├── namespace.yaml
│   └── kustomization.yaml        # Main kustomization (combines all)
├── internal/
│   └── controller/
│       └── *_controller.go       # Reconcilers
├── kubectl-plugin/               # Only with --kubectl-plugin
│   ├── cmd/
│   │   ├── root.go               # Plugin entrypoint and global flags
│   │   ├── targeting.go          # Shared endpoint targeting flags
│   │   ├── create.go             # Create CRUD resources
│   │   ├── query.go              # Execute query CRDs
│   │   ├── action.go             # Execute action CRDs
│   │   └── ...                   # get, describe, status, patch, etc.
│   ├── main.go
│   ├── Makefile
│   └── go.mod
├── cmd/
│   └── manager/
│       └── main.go               # Operator entrypoint
├── hack/
│   └── boilerplate.go.txt        # License header for generated code
├── docker-compose.yaml           # Docker Compose for local dev
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

In this mode, every CR must specify where to send requests using the `target` field:
- `target.helmRelease` - Target a Helm release
- `target.statefulSet` - Target a StatefulSet by name
- `target.deployment` - Target a Deployment by name

Example CR with per-CR targeting:
```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: fluffy
spec:
  name: Fluffy
  target:
    helmRelease: petstore-prod   # Required when no global config
    namespace: production         # Optional: defaults to CR namespace
```

If a CR doesn't specify a target and no global configuration exists, reconciliation will fail with:
```
no endpoint configured: set global endpoint (--base-url, --pod-name, --statefulset-name,
--deployment-name, or --helm-release) or specify per-CR targeting
(target.baseURL, target.pod, target.helmRelease, target.statefulSet, or target.deployment)
```

### 1. Static URL Mode

Use a fixed base URL for the REST API:

```bash
./bin/manager --base-url http://api.example.com:8080
```

Environment variable: `REST_API_BASE_URL`

#### Multiple Static URLs (Fan-out Mode)

For fan-out to multiple API endpoints, use `--base-urls` with a comma-separated list:

```bash
./bin/manager --base-urls http://api-1.example.com:8080,http://api-2.example.com:8080
```

Environment variable: `REST_API_BASE_URLS`

**Fan-out behavior:**
- **Write operations** (POST, PUT, DELETE): Sent to ALL URLs; all must succeed
- **Read operations** (GET): Query all URLs, use first successful response

This is useful for:
- Multi-region deployments requiring consistent state
- Active-active setups where all instances must be synchronized
- Testing and validation against multiple API versions

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

All endpoint flags are optional. If no global endpoint is configured, each CR must specify its target using the `target` sub-object (`target.helmRelease`, `target.statefulSet`, or `target.deployment`).

| Flag | Description | Default |
|------|-------------|---------|
| `--base-url` | Static REST API base URL | (optional) |
| `--base-urls` | Comma-separated list of base URLs for fan-out mode | (optional) |
| `--pod-name` | Pod name for direct endpoint targeting | (optional) |
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
| `--watch-labels` | Only watch CRs matching these labels (format: `key1=value1,key2=value2`) | All labels |
| `--watch-namespaces` | Only watch CRs in these namespaces (format: `ns1,ns2,ns3`) | All namespaces |
| `--namespace-scoped` | Only watch CRs in the operator's own namespace (auto-detected) | `false` |
| `--leader-elect` | Enable leader election for high availability (see [Leader Election](#leader-election)) | `false` |

### CR Filtering

Generated operators can filter which CRs they watch using labels, namespaces, or both. This enables running multiple operator instances on a single cluster with non-overlapping CR sets.

#### Namespace-Scoped Mode

The simplest way to restrict an operator to its own namespace:

```bash
./bin/manager --namespace-scoped
```

This auto-detects the operator's namespace from the service account and configures the cache to only watch CRs in that namespace. Useful for namespace-isolated deployments.

#### Watch Specific Namespaces

Watch CRs in specific namespaces only:

```bash
./bin/manager --watch-namespaces "prod-ns,staging-ns"
```

Or via environment variable:
```bash
WATCH_NAMESPACES="prod-ns,staging-ns" ./bin/manager
```

#### Watch by Labels (Sharding)

Run multiple operator instances that each handle a subset of CRs:

```bash
# Instance 1: handles CRs with shard=1
./bin/manager --watch-labels "shard=1"

# Instance 2: handles CRs with shard=2
./bin/manager --watch-labels "shard=2"
```

CRs must include the appropriate label:
```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: my-pet
  labels:
    shard: "1"  # Handled by instance 1
spec:
  name: Fluffy
```

#### Combining Filters

Namespace and label filters can be combined:

```bash
./bin/manager --watch-namespaces "production" --watch-labels "tier=backend"
```

### Leader Election

Enable leader election with `--leader-elect` to run multiple replicas for high availability. Only one replica actively reconciles at a time; the others remain on standby and take over if the leader fails.

```bash
./bin/manager --leader-elect
```

The operator creates a `coordination.k8s.io/Lease` resource to coordinate leadership. The lease ID is `<app-name>.<api-group>` (e.g., `petstore.petstore.example.com`).

**RBAC:** The generator produces leader election RBAC manifests for both kustomize (`config/rbac/leader_election_role.yaml`, `config/rbac/leader_election_role_binding.yaml`) and Helm chart (`chart/<app>/templates/leader-election-rbac.yaml`). These grant the operator permission to manage leases and events in its own namespace.

#### Independent Instances Per Namespace

When combined with `--namespace-scoped`, leader election supports independent operator instances in different namespaces:

```bash
# Operator in namespace "team-a" — elects its own leader, watches only "team-a" CRs
./bin/manager --leader-elect --namespace-scoped

# Operator in namespace "team-b" — independent leader, watches only "team-b" CRs
./bin/manager --leader-elect --namespace-scoped
```

This works because:

1. **Leases are namespace-scoped** — each operator creates its lease in its own namespace, so operators in different namespaces hold independent leases (even though the lease name is the same)
2. **`--namespace-scoped` restricts the watch scope** — the operator auto-detects its namespace from the service account and only watches CRs in that namespace
3. **RBAC uses `Role` (not `ClusterRole`)** — each operator only has permission to manage leases in its own namespace

This is the recommended configuration for the `k3s-deploy` Docker Compose profile, which installs the Helm chart with `--leader-elect,--namespace-scoped` by default.

## Endpoint Selection Strategies

When using workload discovery, you can configure how endpoints are selected:

| Strategy | Description | StatefulSet | Deployment |
|----------|-------------|-------------|------------|
| `round-robin` | Distribute requests evenly across all healthy pods | ✓ | ✓ |
| `leader-only` | Always use pod-0 (ordinal 0) as the primary endpoint | ✓ | ✗ |
| `any-healthy` | Use any single healthy pod, failover if unhealthy | ✓ | ✓ |
| `all-healthy` | Fan-out requests to all healthy pods (for broadcast operations) | ✓ | ✓ |
| `by-ordinal` | Route to a specific pod based on `target.podOrdinal` field in the CR | ✓ | ✗ |

### Using by-ordinal Strategy

When using `--strategy=by-ordinal`, each CR can specify which pod to target:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: my-pet
spec:
  name: Fluffy
  target:
    podOrdinal: 2  # Routes to pod-2
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
  target:
    helmRelease: petstore-prod  # Discovers workload from this Helm release
    namespace: production        # Optional: namespace to look for the Helm release
    podOrdinal: 0                # Optional: specific pod ordinal (StatefulSet only)
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
  target:
    statefulSet: petstore-api    # Routes to this StatefulSet
    namespace: production         # Optional: namespace
    podOrdinal: 2                 # Optional: specific pod ordinal
```

#### Target by Deployment Name

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: my-pet
spec:
  name: Fluffy
  target:
    deployment: petstore-api     # Routes to this Deployment
    namespace: production         # Optional: namespace
```

Note: `target.podOrdinal` is ignored when targeting a Deployment.

#### Target by Base URL

For direct control over the API endpoint without workload discovery:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: my-pet
spec:
  name: Fluffy
  target:
    baseURL: "http://api.example.com:8080"  # Direct URL to the REST API
```

This is the simplest targeting option and takes highest priority over all other targeting methods. It's useful when:
- The API is hosted externally (not in the Kubernetes cluster)
- You need to target a specific URL without workload discovery
- Testing against a local development server

#### Target by Multiple Base URLs (Fan-out)

For fan-out to multiple API endpoints at the CR level:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: my-pet
spec:
  name: Fluffy
  target:
    baseURLs:
      - "http://api-1.example.com:8080"
      - "http://api-2.example.com:8080"
```

**Fan-out behavior:**
- **Write operations** (POST, PUT, DELETE): Sent to ALL URLs; all must succeed
- **Read operations** (GET): Query all URLs, use first successful response

This is useful for:
- Multi-region deployments requiring consistent state per-resource
- Active-active setups where specific resources must be synchronized across instances
- Canary deployments where certain resources should exist on both old and new instances

#### Target by Pod Name

Target a specific pod directly by name:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: my-pet
spec:
  name: Fluffy
  target:
    pod: petstore-api-0           # Routes to this specific pod
    namespace: production          # Optional: namespace
```

This is useful when:
- You want to target a specific pod without StatefulSet/Deployment discovery
- Testing against a specific pod instance
- The pod is not managed by a StatefulSet or Deployment

### Spec Fields Reference

Every generated CR includes these controller-specific fields in the spec:

| Field | Description |
|-------|-------------|
| `target.baseURL` | Static base URL for the REST API (highest priority, overrides all other targeting) |
| `target.baseURLs` | List of base URLs for fan-out mode (writes to all, reads use first success) |
| `target.pod` | Pod name to route requests to directly |
| `target.podOrdinal` | StatefulSet pod ordinal to route requests to (by-ordinal strategy) |
| `target.helmRelease` | Helm release name for per-CR workload discovery |
| `target.statefulSet` | StatefulSet name for per-CR workload discovery |
| `target.deployment` | Deployment name for per-CR workload discovery |
| `target.namespace` | Namespace for target workload (defaults to CR namespace) |
| `externalIDRef` | Reference an existing external resource by ID (only for CRDs without path parameters) |
| `readOnly` | If true, only observe the resource (no create/update/delete) |
| `mergeOnUpdate` | If true (default), merge spec with current API state before updates (see [Partial Updates](#partial-updates)) |
| `onDelete` | Policy for external resource on CR deletion: `Delete`, `Orphan`, or `Restore` (see [OnDelete Policy](#ondelete-policy)) |
| `paused` | If true, reconciliation is suspended |

These fields are stripped from the payload when sending requests to the REST API.

**Note:** The `externalIDRef` field is only generated for CRDs where the REST API doesn't use path parameters to identify resources. When path parameters exist (e.g., `/pet/{petId}`), the ID field in the spec (e.g., `id`) serves as the external reference instead. See [Importing Existing Resources](#importing-existing-resources) for details.

#### Targeting Behavior

When a target is specified:
1. If `target.namespace` is not specified, the CR's namespace is used
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

#### kstatus Compatibility

Generated operators are fully compatible with [kstatus](https://github.com/kubernetes-sigs/cli-utils/tree/master/pkg/kstatus), the Kubernetes status library used by tools like `kubectl wait`, Flux, and Argo CD. All generated controllers set three standard conditions following the kstatus conventions:

| Condition | Description | When True |
|-----------|-------------|-----------|
| `Ready` | Resource has reached its desired state | Resource is synced, observed, or action completed |
| `Reconciling` | Controller is actively working on the resource | During sync, query, aggregation, or pending states |
| `Stalled` | Controller encountered an error and cannot proceed | On failures (API errors, validation errors, etc.) |

The conditions follow the **abnormal-true** pattern recommended by kstatus:
- `Reconciling` is `True` during active work, `False` when done
- `Stalled` is `True` when errors occur, `False` otherwise
- `Ready` uses the standard pattern: `True` for success, `False` for not ready

**State to Condition Mapping by CRD Type:**

| CRD Type | Reconciling=True | Stalled=True | Ready=True |
|----------|------------------|--------------|------------|
| Resource (CRUD) | `Syncing`, `Pending` | `Failed` | `Synced`, `Observed` |
| Query | `Querying`, `Pending` | `Failed` | `Queried` |
| Action | `Executing`, `Pending` | `Failed` | `Completed` |
| Bundle | `Pending`, `Syncing` | `Failed` or child failures | `Synced` |
| Aggregate | `Aggregating`, `Pending` | `Failed` | `Healthy` |

All CRDs also set `observedGeneration` in the status to track which generation of the spec has been processed, enabling tools to detect when a spec change has been fully reconciled.

**Paused State:**

All CRD types support a `spec.paused` field. When `paused: true`:
- Reconciliation stops immediately
- Status state is set to `Paused`
- `Reconciling=False` (not actively working)
- `Ready=False` (not in desired state)
- `Stalled=False` (not an error condition)

This allows temporarily suspending reconciliation without triggering alerts for stalled resources.

**Usage with kubectl:**

```bash
# Wait for a resource to be ready
kubectl wait --for=condition=Ready pet/fluffy --timeout=60s

# Check if resource is stalled
kubectl get pet/fluffy -o jsonpath='{.status.conditions[?(@.type=="Stalled")].status}'

# Wait for reconciliation to complete
kubectl wait --for=condition=Reconciling=False pet/fluffy --timeout=60s

# Wait for non-paused resources only (using JSONPath filter)
for r in $(kubectl get pets -o jsonpath='{.items[?(@.spec.paused!=true)].metadata.name}'); do
  kubectl wait --for=condition=Ready pet/$r --timeout=60s
done
```

## Example: Petstore Operator

The repository includes a petstore example:

```bash
# Generate and build the petstore operator
./scripts/build-example.sh

# Or with Docker (recommended for consistent builds)
docker run --rm -v "$(pwd):/app" -w /app golang:1.25 ./scripts/build-example.sh
```

This generates an operator with `Pet`, `Store`, and `User` CRDs from the OpenAPI petstore spec.

### Running with Docker Compose

#### Generated Docker Compose

The code generator produces a `docker-compose.yaml` in the output directory with three profiles for local development and testing. When `--target-api-image` is provided, the compose file includes a target API service alongside the operator; otherwise, only the operator infrastructure is generated.

```bash
cd examples/generated

# Profile: k3s - Lightweight k3s cluster with operator as Docker container
docker compose --profile k3s up -d
KUBECONFIG=./k3s-output/kubeconfig.yaml kubectl get nodes

# Profile: with-k8s - Connect to existing cluster (kind/minikube)
docker compose --profile with-k8s up -d

# Profile: k3s-deploy - Full in-cluster deployment via Helm chart
docker compose --profile k3s-deploy up -d
KUBECONFIG=./k3s-deploy-output/kubeconfig.yaml kubectl get pods -n <app>-system
```

The `k3s-deploy` profile builds the operator image, imports it into k3s, and installs the Helm chart with leader election enabled. The target API can be deployed separately if `--target-api-image` was not specified.

#### Generated Docker Compose Profiles

| Profile | Description | Components |
|---------|-------------|------------|
| `k3s` | k3s cluster + operator as container | `k3s`, `k3s-init`, `operator-k3s` (+ target API if `--target-api-image`) |
| `with-k8s` | Connect to existing cluster | `operator` (+ target API if `--target-api-image`) |
| `k3s-deploy` | Full in-cluster deployment via Helm | `k3s-deploy`, `k3s-deploy-init`, `k3s-deploy-operator-build`, `k3s-deploy-image-save` |

#### Target API Deployment Manifest

When `--target-api-image` is provided, the generator also creates `config/target-api/deployment.yaml` containing a Kubernetes Deployment and Service for the target REST API. The base path and port are extracted from the OpenAPI spec's `servers[0].url` (override port with `--target-api-port`).

```bash
openapi-operator-gen generate \
  --spec petstore.yaml \
  --target-api-image=swaggerapi/petstore3:unstable \
  --target-api-port=8080 \
  ...
```

This generates health-checked Deployment+Service manifests used by the k3s-deploy profile for deploying the target API inside k3s alongside the operator.

#### Example Docker Compose (examples/)

The `examples/` directory also includes a hand-maintained `docker-compose.yaml` for the petstore example with additional profiles:

| Profile | Description | Components |
|---------|-------------|------------|
| (default) | Petstore API only | `petstore` |
| `k3s` | Full stack with k3s | `petstore`, `k3s`, `k3s-init`, `operator-k3s` |
| `with-k8s` | Connect to existing cluster | `petstore`, `operator` |
| `multi-endpoint` | Fan-out testing with 2 APIs | `petstore-1`, `petstore-2`, `k3s-multi`, `k3s-multi-init`, `operator-multi` |

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
| `REST_API_BASE_URLS` | `--base-urls` (comma-separated) |
| `POD_NAME` | `--pod-name` |
| `STATEFULSET_NAME` | `--statefulset-name` |
| `DEPLOYMENT_NAME` | `--deployment-name` |
| `HELM_RELEASE` | `--helm-release` |
| `WORKLOAD_NAMESPACE` | `--namespace` |
| `SERVICE_NAME` | `--service` |
| `WATCH_LABELS` | `--watch-labels` |
| `WATCH_NAMESPACES` | `--watch-namespaces` |
| `NAMESPACE_SCOPED` | `--namespace-scoped` (set to `true`) |

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

### How helmify Works

Helmify uses a processor pipeline to convert flat kustomize output into a Helm chart with a useful `values.yaml`. The pipeline is:

```
kustomize build config (flat YAML stream)
  → YAML decoder (splits into individual K8s objects)
  → Processor registry (dispatches by GroupVersionKind)
  → Per-resource processors (extract values, generate templates)
  → Chart assembler (deep-merge all Values maps, write files)
```

Each Kubernetes resource kind has a dedicated processor that decides which fields to promote into `values.yaml`. The guiding principle is to **promote fields that operators commonly customize at install time** while leaving structural definitions hardcoded in templates.

**Fields promoted to `values.yaml`:**

| Category | Fields | Template Pattern |
|----------|--------|-----------------|
| Images | `image.repository`, `image.tag` | `repo:tag \| default .Chart.AppVersion` |
| Scaling | `replicas` | Direct substitution |
| Resources | Full limits + requests block | `toYaml \| nindent` |
| Security | Container + pod security contexts | `toYaml \| nindent` |
| Config | ConfigMap data entries | Direct substitution per key |
| Secrets | data/stringData keys (empty placeholders) | `required` + optional `b64enc` |
| Networking | Service type, ports array | Direct or `toYaml` |
| Container | args, imagePullPolicy, env vars with plain values | Direct or `toYaml` |
| Scheduling | nodeSelector, tolerations, topologySpreadConstraints | `toYaml` blocks |
| Metadata | ServiceAccount annotations | `toYaml \| nindent` |
| Cluster | `kubernetesClusterDomain` (from env vars) | `quote` |
| Strategy | type, maxSurge, maxUnavailable | Nested values |

**Fields left hardcoded in templates (not in `values.yaml`):**

- RBAC rules (verbs, apiGroups, resources)
- Probe configurations (liveness/readiness)
- Volume mounts and volumes
- Env vars using `fieldRef` (downward API)
- Selector labels and match expressions
- CRDs (placed in `crds/` directory, untemplatized)

All value paths are converted to **lowerCamelCase** (e.g. `controller-manager` → `controllerManager`), and object names are trimmed of the app prefix before conversion. The resulting `values.yaml` provides sensible defaults for image, replicas, resources, security contexts, and service account annotations.

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

## Kubectl Plugin

When the `--kubectl-plugin` flag is used, the generator creates a kubectl plugin that provides a convenient CLI for managing operator resources. The plugin is generated in the `kubectl-plugin/` directory.

### Installing the Plugin

```bash
cd examples/generated/kubectl-plugin

# Build and install to PATH
make install

# Verify installation
kubectl petstore --help
```

The plugin is named `kubectl-<api-name>` (e.g., `kubectl-petstore` for the petstore API). Once installed, it can be invoked as `kubectl petstore <command>`.

### Plugin Commands

The plugin provides commands organized in three phases:

| Phase | Commands | Description |
|-------|----------|-------------|
| **Phase 1: Core** | `status`, `get`, `describe` | Basic resource viewing |
| **Phase 2: Diagnostic** | `compare`, `diagnose`, `drift` | Multi-endpoint diagnostics |
| **Phase 3: Interactive** | `create`, `query`, `action`, `patch`, `pause`, `unpause`, `cleanup` | Resource management |

### Phase 1: Core Commands

**status** - View aggregate health status across all resources:
```bash
kubectl petstore status
```

**get** - List resources of a specific kind:
```bash
kubectl petstore get pets
kubectl petstore get pets -o yaml
kubectl petstore get orders --all-namespaces
```

**describe** - Get detailed information about a resource:
```bash
kubectl petstore describe pet fluffy
kubectl petstore describe pet fluffy --show-response
```

### Phase 2: Diagnostic Commands

**compare** - Compare resource state across multiple pods (multi-endpoint mode):
```bash
kubectl petstore compare pet fluffy
kubectl petstore compare pet fluffy --pods=0,1,2
```

**diagnose** - Run diagnostics on a resource:
```bash
kubectl petstore diagnose pet fluffy
```

**drift** - Show drift detection report:
```bash
kubectl petstore drift
kubectl petstore drift --kind=Pet --show-diff
kubectl petstore drift --all-namespaces
```

In multi-endpoint mode, drift report shows which specific endpoints have drift:
```
DRIFT REPORT (namespace: default)

RESOURCE      DRIFT   COUNT   LAST DETECTED           DRIFTED ENDPOINTS
pet/fluffy    Yes     3       2024-01-15T10:30:00Z    2/3: pod-0, pod-2
pet/buddy     No      0       -                       0/3
order/12345   Yes     1       2024-01-15T09:15:00Z    1/3: pod-1
```

### Phase 3: Interactive Commands

**List available types** - See available resource, query, and action types:
```bash
# List all resource types for create
kubectl petstore create types

# List all query types
kubectl petstore query queries

# List all action types
kubectl petstore action actions
```

**create** - Create CRUD resource CRs from CLI flags:
```bash
# Create a resource with spec fields as flags
kubectl petstore create pet --name=fluffy --status=available

# Create with a custom CR name
kubectl petstore create pet --cr-name=my-pet --name=fluffy

# Create without waiting for sync
kubectl petstore create pet --name=fluffy --no-wait

# Create from a YAML/JSON spec file
kubectl petstore create pet --from-file=pet-spec.yaml

# Dry run - output YAML without creating
kubectl petstore create pet --name=fluffy --dry-run

# Nested objects use JSON syntax
kubectl petstore create pet --name=fluffy --category='{"id":1,"name":"Dogs"}'

# Arrays use JSON syntax for complex types
kubectl petstore create pet --name=fluffy --tags='[{"id":1,"name":"cute"}]'
```

Create command flags:
| Flag | Description |
|------|-------------|
| `--cr-name=NAME` | Name for the CR (auto-generated as `kind-timestamp` if not specified) |
| `--no-wait` | Don't wait for the resource to sync |
| `--timeout=DURATION` | Timeout for waiting on sync (default: `60s`) |
| `--from-file=PATH` | Load spec from a YAML or JSON file |
| `--dry-run` | Output the CR YAML/JSON without creating it |

**query** - Execute read-only query CRDs:
```bash
# One-shot query (auto-waits for result)
kubectl petstore query petfindbystatusquery --status=available

# Periodic query (creates persistent CR)
kubectl petstore query storeinventoryquery --interval=5m --name=inventory-monitor

# Re-run a named query (automatically deletes and recreates the CR)
kubectl petstore query petfindbystatusquery --status=available --name=my-status-query

# Get results from an existing query CR (without creating a new one)
kubectl petstore query petfindbystatusquery --get=my-status-query

# Quiet mode - output only result data (for scripting/piping to jq)
kubectl petstore query petfindbystatusquery --status=available -q --output=json

# Combine quiet mode with JSON for easy scripting
kubectl petstore query storeinventoryquery -q | jq '.items[0].name'

# Dry run - output YAML without creating
kubectl petstore query petfindbystatusquery --status=available --dry-run
```

Query command flags:
| Flag | Description |
|------|-------------|
| `--interval=DURATION` | Create periodic query (e.g., `5m`, `1h`) |
| `--name=NAME` | Name for the query CR (required for periodic queries) |
| `--wait` | Wait for query to complete (default for one-shot queries) |
| `--timeout=DURATION` | Timeout for waiting (default: `30s`) |
| `--get=NAME` | Get results from existing query CR instead of creating new one |
| `-q, --quiet` | Output only result data, no status messages (defaults to JSON) |
| `--dry-run` | Output the CR YAML/JSON without creating it |

**action** - Execute write operation CRDs:
```bash
# Execute an action with parameters
kubectl petstore action petuploadimageaction --petId=123 --file=./photo.jpg

# Execute without waiting for result
kubectl petstore action petuploadimageaction --petId=123 --wait=false

# Custom name for the action CR (re-runs automatically if name exists)
kubectl petstore action petuploadimageaction --petId=123 --name=upload-fluffy-photo

# Dry run - output YAML without creating
kubectl petstore action petuploadimageaction --petId=123 --dry-run
```

Action command flags:
| Flag | Description |
|------|-------------|
| `--name=NAME` | Name for the action CR (auto-generated if not specified) |
| `--wait` | Wait for action to complete (default: `true`) |
| `--timeout=DURATION` | Timeout for waiting (default: `60s`) |
| `--file=PATH` | File to upload (for upload actions, base64 encoded) |
| `--dry-run` | Output the CR YAML/JSON without creating it |

**patch** - Make temporary changes with TTL auto-rollback:
```bash
# Patch with individual flags
kubectl petstore patch pet fluffy --status=pending --ttl=1h

# Patch with JSON spec
kubectl petstore patch pet fluffy --spec='{"status":"pending"}' --ttl=1h

# Mix individual flags and --spec (individual flags take precedence)
kubectl petstore patch pet fluffy --spec='{"status":"pending"}' --name="Fluffy Updated"

# Dry run - output the merge patch YAML
kubectl petstore patch pet fluffy --status=pending --dry-run

# Restore immediately
kubectl petstore patch pet fluffy --restore

# List all patched resources
kubectl petstore patch list

# Nested objects and arrays use JSON syntax
kubectl petstore patch pet fluffy --tags='[{"id":1,"name":"cute"}]'
```

Patch command flags:
| Flag | Description |
|------|-------------|
| `--spec=JSON` | JSON spec to apply (e.g., `'{"status":"pending"}'`) |
| `--ttl=DURATION` | Time-to-live for the patch (e.g., `1h`, `30m`) |
| `--restore` | Restore the original state |
| `--dry-run` | Output the merge patch YAML/JSON without applying |

**pause/unpause** - Control reconciliation:
```bash
kubectl petstore pause pet fluffy --reason="Maintenance window"
kubectl petstore unpause pet fluffy
```

**cleanup** - Remove temporary and diagnostic resources:
```bash
# Remove one-shot queries/actions
kubectl petstore cleanup --one-shot

# Restore expired TTL patches (not delete)
kubectl petstore cleanup --expired

# Dry run - output structured YAML of targets
kubectl petstore cleanup --dry-run

# Cleanup across all namespaces
kubectl petstore cleanup --all-namespaces --force
```

### Endpoint Targeting Flags

The `create`, `query`, `action`, and `patch` commands share a set of endpoint targeting flags. These set the `spec.target` fields on the created CR, allowing per-resource workload targeting:

| Flag | Description |
|------|-------------|
| `--target-base-url=URL` | Static base URL override for the API endpoint |
| `--target-base-urls=URLs` | Comma-separated base URLs for fan-out to multiple endpoints |
| `--target-statefulset=NAME` | Target a named StatefulSet for endpoint discovery |
| `--target-deployment=NAME` | Target a named Deployment for endpoint discovery |
| `--target-pod=NAME` | Target a specific pod by name |
| `--target-pod-ordinal=N` | Target specific StatefulSet pod ordinal (requires `--target-statefulset` or `--target-helm-release`) |
| `--target-helm-release=NAME` | Discover target workload via Helm release name |
| `--target-namespace=NS` | Namespace for target workload lookup |
| `--target-labels=LABELS` | Additional labels for workload discovery (`key=value,key=value`; requires `--target-helm-release`) |

Examples:

```bash
# Target a specific StatefulSet pod
kubectl petstore create pet --name=fluffy --target-statefulset=petstore --target-pod-ordinal=0

# Target a Deployment
kubectl petstore query storeinventoryquery --target-deployment=petstore-api

# Target a Helm release with label narrowing
kubectl petstore create pet --name=fluffy --target-helm-release=petstore --target-labels=tier=frontend

# Direct URL override
kubectl petstore action petuploadimageaction --petId=123 --target-base-url=http://petstore:8080/api/v3
```

The `--target-labels` flag narrows workload discovery within a Helm release. This is useful when a Helm chart deploys multiple workloads (e.g., a StatefulSet and a Deployment) and you need to target a specific one.

### Dry Run Support

All mutating commands support `--dry-run` to output YAML (or JSON with `--output=json`) of what would be submitted, without making any changes:

```bash
kubectl petstore create pet --name=fluffy --dry-run
kubectl petstore query petfindbystatusquery --status=available --dry-run
kubectl petstore action petuploadimageaction --petId=123 --dry-run
kubectl petstore patch pet fluffy --status=pending --dry-run
kubectl petstore cleanup --dry-run
```

### Idempotent CR Reuse

When a CR name is specified via `--cr-name` (create) or `--name` (query/action) and a CR with that name already exists, the plugin handles it gracefully instead of failing. This enables repeatable execution from automation tools like Rundeck.

**Create commands** use upsert semantics — the existing CR's spec is updated in place while preserving metadata, annotations, labels, and finalizers:

```bash
# First run creates the CR
kubectl petstore create pet --cr-name=my-pet --name=fluffy --status=available

# Subsequent runs update the existing CR's spec
kubectl petstore create pet --cr-name=my-pet --name=fluffy --status=sold
```

**Query and action commands** use delete-and-recreate semantics — the existing CR is deleted and a fresh one is created to re-trigger execution:

```bash
# First run creates the query CR and returns results
kubectl petstore query petfindbystatusquery --name=my-query --status=available

# Subsequent runs delete the old CR and create a new one
kubectl petstore query petfindbystatusquery --name=my-query --status=sold
```

The delete-and-recreate flow waits up to 10 seconds for the old CR to be fully removed before creating the replacement. This is automatic and requires no additional flags.

### Dynamic Parameter Parsing

The `create`, `query`, `action`, and `patch` commands all support dynamic `--key=value` flags that are passed through as spec fields. Values are automatically coerced:

| Syntax | Parsed As |
|--------|-----------|
| `--name=fluffy` | String: `"fluffy"` |
| `--petId=123` | Integer: `123` |
| `--price=9.99` | Float: `9.99` |
| `--active=true` | Boolean: `true` |
| `--category='{"id":1,"name":"Dogs"}'` | JSON object |
| `--tags='[{"id":1,"name":"cute"}]'` | JSON array |
| `--tags=cute,fluffy` | String array: `["cute", "fluffy"]` |

> **Note:** For fields that require complex types (objects or arrays of objects), always use JSON syntax. Comma-separated values produce string arrays, which may not match CRD schemas expecting arrays of objects. If a CRD validation error occurs, the CLI provides hints suggesting the correct JSON syntax.

### TTL-Based Patches

The patch command supports TTL (time-to-live) based auto-rollback. This is useful for:
- Temporary configuration changes during maintenance
- Testing changes that should auto-revert
- Preventing "forgotten" manual overrides

**How it works:**

1. When you apply a patch with `--ttl`, the plugin:
   - Saves the original spec values in an annotation
   - Sets a `patch-expires` annotation with the expiration time
   - Updates the resource spec

2. The operator automatically restores the original state when TTL expires:
   - Checked on every reconciliation loop
   - Original values are restored from annotations
   - Patch annotations are cleared

3. Manual cleanup also restores (not deletes) expired patches:
   ```bash
   kubectl petstore cleanup --expired
   ```

**Annotations used:**
- `<api-group>/patch-ttl` - Original TTL duration
- `<api-group>/patch-expires` - RFC3339 expiration timestamp
- `<api-group>/patch-original-state` - JSON of original spec values
- `<api-group>/patched-by` - kubectl-plugin marker

## Rundeck Project

When the `--rundeck-project` flag is used (requires `--kubectl-plugin`), the generator creates two [Rundeck](https://www.rundeck.com/) projects with job definitions that wrap the kubectl plugin commands. This provides a web UI for executing operator management tasks with audit trails, scheduling, and role-based access.

- **Script-based project** (`rundeck-project/`) — Jobs execute the kubectl plugin binary directly on the Rundeck server. Requires the plugin binary to be installed in Rundeck's PATH.
- **Docker execution project** (`rundeck-docker-project/`) — Jobs run the kubectl plugin inside a Docker container via `docker run`. Requires Docker to be available on the Rundeck server. See [Docker Execution Project](#docker-execution-project).

### Enabling Rundeck Generation

```bash
openapi-operator-gen generate \
  --spec petstore.yaml \
  --output ./generated \
  --group petstore.example.com \
  --version v1alpha1 \
  --module github.com/example/petstore-operator \
  --kubectl-plugin \
  --rundeck-project
```

Or in a config file:

```yaml
kubectl-plugin: true
rundeck-project: true
```

### Generated Structure

Both projects share the same directory layout and job definitions — only the execution method differs:

```
rundeck-project/                    # Script-based execution
├── project.properties
├── tokens.properties
└── jobs/
    ├── resources/                  # CRUD resource jobs (3 per resource)
    │   ├── create-pet.yaml
    │   ├── get-pets.yaml
    │   ├── describe-pet.yaml
    │   └── ...
    ├── queries/                    # Query endpoint jobs (1 per query)
    │   ├── petfindbystatusquery.yaml
    │   └── ...
    ├── actions/                    # Action endpoint jobs (1 per action)
    │   ├── petuploadimageaction.yaml
    │   └── ...
    └── operations/                 # Cluster-wide operations (always 3)
        ├── status.yaml
        ├── drift.yaml
        └── cleanup.yaml

rundeck-docker-project/             # Docker execution (same structure)
├── project.properties
├── tokens.properties
└── jobs/
    └── ...                         # Same jobs, using docker run

kubectl-plugin/
└── Dockerfile                      # Multi-stage build for plugin image
```

### Job Types

Jobs are generated based on how the OpenAPI endpoints are classified:

| CRD Classification | Jobs Generated | Rundeck Group |
|---|---|---|
| CRUD Resource (GET + write methods) | `create-{kind}`, `get-{plural}`, `describe-{kind}` | `resources/{kind}` |
| Query Endpoint (GET only) | One job per query kind | `queries` |
| Action Endpoint (POST/PUT only) | One job per action kind | `actions` |
| Operations (all operators) | `status`, `drift`, `cleanup` | `operations` |

Each job exposes the CRD's spec fields as Rundeck options. Fields with enum constraints in the OpenAPI spec become enforced dropdown selections. Nested object fields are annotated with "(JSON format)" to indicate they accept JSON input.

### Job Options

Every job includes common operational options alongside the resource-specific parameters:

| Option | Available In | Description |
|---|---|---|
| `resource_name` | create, query, action | CR name (auto-generated if empty) |
| `namespace` | All jobs | Kubernetes namespace override |
| `dry_run` | create, query, action, cleanup | Preview without executing |
| `timeout` | create, query, action | Wait timeout (default: `60s`) |
| `no_wait` | create | Skip waiting for sync |
| `wait` | query, action | Wait for results (default: `true`) |
| `output` | get | Output format (`table`, `wide`, `json`, `yaml`) |

When `resource_name` is specified and a CR with that name already exists, the kubectl plugin handles it idempotently (see [Idempotent CR Reuse](#idempotent-cr-reuse)). This allows Rundeck jobs to be re-executed safely.

### Docker Execution Project

The Docker execution project (`rundeck-docker-project/`) runs kubectl plugin commands inside a container via `docker run`. This is useful when you don't want to install the plugin binary directly on the Rundeck server, or when running in environments where container-based execution is preferred.

Each job script follows this pattern:

```bash
set -e
IMAGE="${PLUGIN_RUNNER_IMAGE:-petstore-kubectl-plugin:latest}"
NET="${DOCKER_NETWORK:-host}"
KUBE_VOL="${DOCKER_KUBE_VOLUME:-}"
KUBE="${KUBECONFIG:-/root/.kube/config}"
if [ -n "$KUBE_VOL" ]; then
  VOL_ARGS="-v $KUBE_VOL:/kube:ro -e KUBECONFIG=/kube/kube/config"
else
  VOL_ARGS="-v $KUBE:/root/.kube/config:ro"
fi
docker run --rm --network $NET $VOL_ARGS $IMAGE petstore <subcommand> [args...]
```

**Plugin Docker Image**: A `Dockerfile` is generated in `kubectl-plugin/` that produces a minimal image based on `bitnami/kubectl` with the plugin binary installed. Build it with:

```bash
cd kubectl-plugin
docker build -t petstore-kubectl-plugin:latest .
```

**Configuration via environment variables:**

| Variable | Default | Description |
|---|---|---|
| `PLUGIN_RUNNER_IMAGE` | `{app}-kubectl-plugin:latest` | Docker image containing the kubectl plugin |
| `DOCKER_NETWORK` | `host` | Docker network for the container |
| `DOCKER_KUBE_VOLUME` | *(empty)* | Named Docker volume containing kubeconfig (compose mode) |
| `KUBECONFIG` | `/root/.kube/config` | Host path to kubeconfig (standalone mode) |

**Kubeconfig mounting** uses two modes:

- **Standalone mode** (default): When `DOCKER_KUBE_VOLUME` is not set, the kubeconfig is mounted from a host path via `-v $KUBECONFIG:/root/.kube/config:ro`. This works when Rundeck runs directly on the host.
- **Compose mode**: When `DOCKER_KUBE_VOLUME` is set, a named Docker volume is mounted instead via `-v $DOCKER_KUBE_VOLUME:/kube:ro`. This is necessary when Rundeck itself runs in Docker, since `-v` always mounts from the host filesystem, not from within the calling container. The Docker Compose integration sets this automatically.

### Docker Compose Integration

When Rundeck generation is enabled, the `k3s-deploy` Docker Compose profile includes additional services for both Rundeck projects:

**rundeck** - The Rundeck server (port 4440), configured with Docker socket access for the Docker execution project:
```bash
# Start the full stack including Rundeck
docker compose --profile k3s-deploy up -d

# Access Rundeck at http://localhost:4440
# Default credentials: admin / admin
```

**rundeck-kubectl-build** - Builds the kubectl plugin binary (linux/amd64) from the generated source. The binary is stored in a shared volume for the script-based project.

**rundeck-docker-plugin-build** - Builds the kubectl plugin Docker image from `kubectl-plugin/Dockerfile`. The image is used by the Docker execution project's jobs.

**rundeck-docker-cli-stage** - Copies the Docker CLI binary into a shared volume so it's available inside the Rundeck container for executing `docker run` commands.

**rundeck-init** - Runs after Rundeck is healthy. It creates both Rundeck projects (script-based and Docker execution), imports all job definitions via the Rundeck API, copies binaries into the Rundeck container, and configures kubeconfig for cluster access.

The Docker Compose configuration uses named resources so Docker execution jobs can reference them:
- **Network**: `{app}-operator-net` — shared between Rundeck and plugin containers
- **Volume**: `{app}-rundeck-data` — contains kubeconfig accessible to plugin containers

The Rundeck service sets `DOCKER_NETWORK` and `DOCKER_KUBE_VOLUME` environment variables automatically, so Docker execution jobs work without manual configuration.

To update the plugin after code changes:

```bash
# Rebuild the plugin binary and Docker image
docker compose --profile k3s-deploy up -d rundeck-kubectl-build rundeck-docker-plugin-build --force-recreate

# Wait for builds, then restage binaries and reimport jobs
docker compose --profile k3s-deploy up -d rundeck-init --force-recreate
```

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
