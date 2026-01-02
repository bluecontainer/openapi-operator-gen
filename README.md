# OpenAPI Operator Generator

A code generator that creates Kubernetes operators from OpenAPI specifications. It maps REST API resources to Kubernetes Custom Resource Definitions (CRDs) and generates controller reconciliation logic that syncs CRs with the backing REST API.

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
  --module github.com/example/petstore-operator
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

1. **Create**: When a CR is created, the controller POSTs the spec to the REST API
2. **Update**: When a CR is modified, the controller PUTs the updated spec
3. **Delete**: When a CR is deleted, the controller DELETEs the resource from the REST API

The controller uses finalizers to ensure external resources are cleaned up before the CR is removed.

### Status Fields

Each CR has a status subresource with:

| Field | Description |
|-------|-------------|
| `state` | Current state: `Pending`, `Syncing`, `Synced`, or `Failed` |
| `externalID` | ID of the resource in the external REST API |
| `lastSyncTime` | Timestamp of the last successful sync |
| `message` | Human-readable status message |
| `conditions` | Standard Kubernetes conditions |
| `response` | Last response body from the REST API |

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
