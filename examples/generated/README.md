# Petstore Operator

A Kubernetes operator generated from an OpenAPI specification that manages petstore resources by syncing Custom Resources with the backing REST API.

## Overview

This operator was generated using [openapi-operator-gen](https://github.com/bluecontainer/openapi-operator-gen) from an OpenAPI specification. It provides Kubernetes-native management of petstore resources through Custom Resource Definitions (CRDs).

### Generator Command

This operator was generated with the following command:

```bash
openapi-operator-gen generate \
  --spec examples/petstore.1.0.27.yaml \
  --output examples/generated \
  --group petstore.example.com \
  --version v1alpha1 \
  --module github.com/bluecontainer/petstore-operator
```

To regenerate after modifying the OpenAPI spec, run the same command.

## Features

- Kubernetes-native resource management via CRDs
- Automatic synchronization between CRs and the REST API
- Drift detection and reconciliation
- Support for multiple endpoint discovery modes
- Per-CR targeting for multi-tenant scenarios
- OpenTelemetry instrumentation for observability

## Prerequisites

- Go 1.21+
- Kubernetes cluster (1.25+)
- kubectl configured to access your cluster
- Access to the petstore REST API

## Building

```bash
# Download dependencies
go mod tidy

# Generate code (deep copy methods, CRDs, RBAC)
make generate manifests rbac

# Build the operator binary
make build

# Build Docker image
make docker-build IMG=<your-registry>/petstore-operator:latest
```

## Testing

This operator includes comprehensive tests using Go's testing framework and Ginkgo/Gomega for BDD-style integration tests.

### Unit Tests

Unit tests run quickly without external dependencies and use mock HTTP servers to simulate the REST API:

```bash
make test
```

Unit tests cover:
- Controller reconciliation logic
- HTTP request/response handling
- Error scenarios (404, 500, timeouts, rate limiting)
- URL construction and path parameter handling
- Status updates and state transitions

### Integration Tests

Integration tests use [envtest](https://book.kubebuilder.io/reference/envtest.html) to run a real Kubernetes API server (etcd + kube-apiserver) locally. This validates CRD schemas, RBAC, and controller behavior against a real Kubernetes environment.

```bash
# Install envtest binaries (first time only)
make envtest

# Run integration tests
make test-integration
```

Integration tests cover:
- CRD creation with schema validation
- Resource updates and spec changes
- Resource deletion and cleanup
- Required field validation

### All Tests

Run both unit and integration tests together:

```bash
make test-all
```

### Test Coverage

Test coverage reports are generated in `cover.out`:

```bash
make test
go tool cover -html=cover.out -o coverage.html
```

## Custom Resource Definitions

This operator manages the following CRDs:

| Kind | Type | Description |
|------|------|-------------|
| `Order` | Resource | Full CRUD lifecycle management |
| `Pet` | Resource | Full CRUD lifecycle management |
| `User` | Resource | Full CRUD lifecycle management |
| `PetFindbystatusQuery` | Query | One-shot or periodic read-only query |
| `PetFindbytagsQuery` | Query | One-shot or periodic read-only query |
| `StoreInventoryQuery` | Query | One-shot or periodic read-only query |
| `UserLoginQuery` | Query | One-shot or periodic read-only query |
| `UserLogoutQuery` | Query | One-shot or periodic read-only query |
| `PetUploadimageAction` | Action | One-shot or periodic action |
| `UserCreatewithlistAction` | Action | One-shot or periodic action |

### Resource CRDs

Resource CRDs represent entities in the REST API with full CRUD support:
- **Create**: When a CR is created, the operator POSTs to the REST API
- **Read**: The operator GETs current state and updates CR status
- **Update**: Spec changes trigger PUT requests to sync state
- **Delete**: CR deletion triggers DELETE on the REST API

### Query CRDs

Query CRDs execute read-only queries against the REST API:
- **One-shot mode** (default): Query executes once when CR is created, results stored in status
- **Periodic mode**: Set `spec.queryInterval` to re-execute at regular intervals
- Results stored in CR status
- No external resource creation
- Re-executes automatically when spec changes

Example one-shot query:
```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetFindbystatusQuery
metadata:
  name: available-pets
spec:
  status: available
  # No queryInterval - executes once
```

Example periodic query:
```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetFindbystatusQuery
metadata:
  name: available-pets-monitor
spec:
  status: available
  queryInterval: 30s  # Re-execute every 30 seconds
```

### Action CRDs

Action CRDs execute operations on the REST API:
- One-shot execution by default
- Optional periodic re-execution via `reExecuteInterval`
- Results stored in CR status

## Running the Operator

### Option 1: No Global Configuration (Per-CR Targeting)

Run without endpoint flags - each CR must specify its target:

```bash
./bin/manager
```

CRs must include one of:
- `spec.targetHelmRelease` - Target a Helm release
- `spec.targetStatefulSet` - Target a StatefulSet
- `spec.targetDeployment` - Target a Deployment

### Option 2: Static URL Mode

Use a fixed base URL for all API requests:

```bash
./bin/manager --base-url http://petstore-api:8080
```

### Option 3: StatefulSet Discovery

Discover endpoints from StatefulSet pods:

```bash
./bin/manager \
  --statefulset-name petstore \
  --namespace default \
  --port 8080
```

### Option 4: Deployment Discovery

Discover endpoints from Deployment pods:

```bash
./bin/manager \
  --deployment-name petstore \
  --namespace default \
  --port 8080
```

### Option 5: Helm Release Discovery

Auto-discover workload from a Helm release:

```bash
./bin/manager \
  --helm-release petstore \
  --namespace default \
  --port 8080
```

## Configuration Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--base-url` | Static REST API base URL | (optional) |
| `--statefulset-name` | StatefulSet for endpoint discovery | (optional) |
| `--deployment-name` | Deployment for endpoint discovery | (optional) |
| `--helm-release` | Helm release for endpoint discovery | (optional) |
| `--namespace` | Namespace of the workload | Operator namespace |
| `--port` | REST API port on pods | `8080` |
| `--scheme` | URL scheme (http/https) | `http` |
| `--strategy` | Endpoint selection strategy | `round-robin` |
| `--health-path` | Health check path | `/health` |

### Endpoint Selection Strategies

| Strategy | Description |
|----------|-------------|
| `round-robin` | Distribute requests across healthy pods |
| `leader-only` | Always use pod-0 (StatefulSet only) |
| `any-healthy` | Use any single healthy pod |
| `all-healthy` | Fan-out to all healthy pods |
| `by-ordinal` | Route to specific pod via `targetPodOrdinal` |
## Example Usage

### Creating a Resource

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Order
metadata:
  name: example
spec:
  # Resource-specific fields here
  targetHelmRelease: petstore  # Optional: per-CR targeting
```

### Per-CR Targeting

Override the operator's global endpoint for specific CRs:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Order
metadata:
  name: example
spec:
  # Target a specific Helm release
  targetHelmRelease: petstore-prod
  targetNamespace: production

  # Or target a specific StatefulSet
  # targetStatefulSet: petstore-api
  # targetPodOrdinal: 0  # Optional: specific pod

  # Or target a specific Deployment
  # targetDeployment: petstore-api
```

### Importing Existing Resources

Reference an existing resource in the REST API:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Order
metadata:
  name: imported-resource
spec:
  externalIDRef: "existing-id-123"
  # Spec fields will be synced to match the external resource
```

### Read-Only Observation

Observe a resource without modifying it:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Order
metadata:
  name: observed-resource
spec:
  externalIDRef: "existing-id-123"
  readOnly: true
```

## Deploying to Kubernetes

### Install CRDs

```bash
make install
```

### Deploy the Operator

```bash
# Build and push image
make docker-build docker-push IMG=<your-registry>/petstore-operator:latest

# Deploy to cluster
make deploy IMG=<your-registry>/petstore-operator:latest
```

### Deploy to kind (local development)

```bash
make kind-deploy IMG=petstore-operator:latest
```

### Undeploy

```bash
make undeploy
make uninstall
```

## Environment Variables

| Variable | Flag Equivalent |
|----------|-----------------|
| `REST_API_BASE_URL` | `--base-url` |
| `STATEFULSET_NAME` | `--statefulset-name` |
| `DEPLOYMENT_NAME` | `--deployment-name` |
| `HELM_RELEASE` | `--helm-release` |
| `WORKLOAD_NAMESPACE` | `--namespace` |
| `SERVICE_NAME` | `--service` |

## Observability

This operator includes OpenTelemetry instrumentation. Enable it by setting:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317
```

### Metrics

- `reconcile_total` - Total reconciliations by status
- `reconcile_duration_seconds` - Reconciliation duration
- `api_call_total` - REST API calls by method and status
- `api_call_duration_seconds` - API call duration

### Tracing

Spans are created for:
- `Reconcile` - Full reconciliation cycle
- `getResource` - GET requests
- `createResource` - POST requests
- `updateResource` - PUT requests
- `deleteFromEndpoint` - DELETE requests

## Status Fields

### Resource CRs

| Field | Description |
|-------|-------------|
| `state` | `Pending`, `Syncing`, `Synced`, `Failed`, `Observed`, `NotFound` |
| `externalID` | ID of the resource in the REST API |
| `lastSyncTime` | Last successful sync timestamp |
| `message` | Human-readable status message |
| `driftDetected` | Whether spec differs from external state |

### Query CRs

| Field | Description |
|-------|-------------|
| `state` | `Pending`, `Querying`, `Queried`, `Failed` |
| `lastQueryTime` | Last query execution timestamp |
| `resultCount` | Number of results returned |
| `results` | Query results data |

### Action CRs

| Field | Description |
|-------|-------------|
| `state` | `Pending`, `Executing`, `Completed`, `Failed` |
| `executedAt` | First execution timestamp |
| `completedAt` | Last completion timestamp |
| `executionCount` | Number of executions |
| `httpStatusCode` | HTTP response status code |
| `result` | Action result data |

## License

Apache License 2.0

---

Generated by [openapi-operator-gen](https://github.com/bluecontainer/openapi-operator-gen)
