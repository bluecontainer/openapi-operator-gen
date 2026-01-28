# kubectl Plugin Generator Design

This document describes the design for generating a kubectl plugin alongside the operator from an OpenAPI specification.

## Overview

The generator will produce a kubectl plugin (compatible with krew) that provides a native Kubernetes CLI experience for interacting with generated CRDs. The plugin name will be derived from the API name (e.g., `kubectl petstore` for a petstore API).

## Generated Plugin Structure

```
generated/
├── cmd/
│   └── manager/           # Operator (existing)
├── kubectl-plugin/
│   ├── cmd/
│   │   ├── root.go        # Root command with subcommand routing
│   │   ├── status.go      # Aggregate health status
│   │   ├── get.go         # Get resources with rich output
│   │   ├── describe.go    # Detailed resource description
│   │   ├── compare.go     # Multi-pod state comparison
│   │   ├── diagnose.go    # Run diagnostics on a resource
│   │   ├── query.go       # Execute query CRDs
│   │   ├── action.go      # Execute action CRDs
│   │   ├── patch.go       # Temporary changes with TTL
│   │   ├── pause.go       # Pause/unpause reconciliation
│   │   ├── drift.go       # Drift detection report
│   │   └── cleanup.go     # Remove diagnostic resources
│   ├── pkg/
│   │   ├── client/        # Dynamic K8s client wrapper
│   │   ├── output/        # Table/JSON/YAML formatters
│   │   ├── comparison/    # State comparison logic
│   │   └── ttl/           # TTL annotation management
│   ├── main.go
│   ├── go.mod
│   └── Makefile
└── ...
```

## Command Design

### Plugin Naming Convention

The plugin binary follows kubectl plugin naming: `kubectl-<api-name>`

Example: For petstore API → `kubectl-petstore`

Usage: `kubectl petstore <command>`

### Command Reference

#### `status` - Aggregate Health Overview

```bash
# Show aggregate health status
kubectl petstore status

# Watch for changes
kubectl petstore status --watch

# Output formats
kubectl petstore status -o json
kubectl petstore status -o yaml
```

**Output:**
```
AGGREGATE           HEALTH     SYNCED  FAILED  PENDING  MESSAGE
production-health   Healthy    14      0       1        All resources synced

RESOURCE SUMMARY:
KIND    TOTAL  SYNCED  FAILED  PENDING  DRIFT
Pet     5      5       0       0        0
Order   8      8       0       0        1
User    2      1       0       1        0
```

#### `get` - List Resources

```bash
# List all pets
kubectl petstore get pets

# List with specific output
kubectl petstore get pets -o wide
kubectl petstore get pets -o json

# Filter by state
kubectl petstore get pets --state=Failed
kubectl petstore get pets --drift

# Filter by labels
kubectl petstore get pets -l environment=production
```

**Output (default):**
```
NAME     STATE   EXTERNAL-ID  DRIFT  AGE
fluffy   Synced  123          No     2d
buddy    Synced  124          No     1d
max      Failed  -            -      5m
```

**Output (wide):**
```
NAME     STATE   EXTERNAL-ID  DRIFT  LAST-SYNC            TARGET-POD   AGE
fluffy   Synced  123          No     2024-01-28T10:00:00  petstore-0   2d
buddy    Synced  124          No     2024-01-28T10:00:00  petstore-1   1d
max      Failed  -            -      -                    petstore-0   5m
```

#### `describe` - Detailed Resource View

```bash
kubectl petstore describe pet fluffy
kubectl petstore describe order order-456
```

**Output:**
```
Name:         fluffy
Namespace:    default
Kind:         Pet
State:        Synced
External ID:  123

Spec:
  name: Fluffy
  status: available
  category:
    id: 1
    name: Dogs

Status:
  State:              Synced
  Last Sync Time:     2024-01-28T10:00:00Z
  Last Get Time:      2024-01-28T10:05:00Z
  Drift Detected:     No
  Sync Count:         42

  Response:
    Status Code: 200
    Data:
      id: 123
      name: Fluffy
      status: available
      category:
        id: 1
        name: Dogs

Conditions:
  TYPE   STATUS  REASON  MESSAGE                    LAST TRANSITION
  Ready  True    Synced  Resource synced with API   2024-01-28T10:00:00Z

Events:
  TYPE    REASON  AGE  MESSAGE
  Normal  Synced  2m   Successfully synced with API
```

#### `compare` - Multi-Pod Comparison

```bash
# Compare resource across all pods
kubectl petstore compare pet fluffy

# Compare specific pods
kubectl petstore compare pet fluffy --pods=0,1,2

# Compare specific field
kubectl petstore compare pet fluffy --field=status.response.data

# Output as JSON for scripting
kubectl petstore compare pet fluffy -o json
```

**Output:**
```
Comparing pet/fluffy across pods 0, 1, 2...

FIELD                     POD-0          POD-1          POD-2
status.state              Synced         Synced         Synced
status.externalID         123            123            123
status.response.data.id   123            123            123
status.response.data.name Fluffy         Fluffy         Fluffy

Result: ✓ All pods consistent
```

**Output (with differences):**
```
Comparing pet/fluffy across pods 0, 1, 2...

FIELD                       POD-0          POD-1          POD-2
status.state                Synced         Synced         Failed
status.externalID           123            123            -
status.response.data.id     123            123            -
status.response.data.name   Fluffy         Fluffy         -

Result: ✗ Inconsistency detected on pod-2
```

#### `diagnose` - Run Diagnostics

```bash
# Run diagnostics on a resource
kubectl petstore diagnose pet fluffy

# Run diagnostics targeting specific pod
kubectl petstore diagnose pet fluffy --pod=0

# Include API response time check
kubectl petstore diagnose pet fluffy --check-latency
```

**Output:**
```
Running diagnostics for pet/fluffy...

CHECKS:
[✓] Resource exists in cluster
[✓] Resource synced with API (externalID: 123)
[✓] No drift detected
[✓] API response healthy (200 OK)
[✓] Response time: 45ms (< 1000ms threshold)
[!] Resource not synced in 2 hours (consider refreshing)

SUMMARY: 5 passed, 0 failed, 1 warning
```

#### `query` - Execute Query CRDs

```bash
# Run a query
kubectl petstore query find-by-status --status=available

# Run inventory query
kubectl petstore query inventory

# Target specific pod
kubectl petstore query inventory --pod=0

# Periodic query (creates persistent CR)
kubectl petstore query inventory --interval=5m --name=inventory-monitor
```

**Output:**
```
Executing query: PetFindbystatusQuery
Target: petstore-api-0

Results (3 items):
  - id: 123, name: Fluffy, status: available
  - id: 124, name: Buddy, status: available
  - id: 125, name: Max, status: available

Query completed in 45ms
```

#### `action` - Execute Actions

```bash
# Execute one-shot action
kubectl petstore action upload-image --pet-id=123 --file=./photo.jpg

# Execute with metadata
kubectl petstore action upload-image --pet-id=123 --metadata="Profile photo"

# Execute and watch result
kubectl petstore action upload-image --pet-id=123 --wait
```

**Output:**
```
Executing action: PetUploadimageAction
Target: petstore-api-0

Action completed successfully
  State: Completed
  Status Code: 200
  Response:
    code: 200
    type: application/json
    message: Image uploaded successfully

Execution time: 1.2s
```

#### `patch` - Temporary Changes with TTL

```bash
# Make temporary change with auto-rollback
kubectl petstore patch pet fluffy --spec='{"status":"pending"}' --ttl=1h

# Preview change without applying
kubectl petstore patch pet fluffy --spec='{"status":"pending"}' --dry-run

# Restore original state immediately
kubectl petstore patch pet fluffy --restore
```

**Output:**
```
Patching pet/fluffy with TTL 1h...

Original state saved to annotation
Applying patch: {"status": "pending"}

Patch applied successfully
  Original status: available
  New status: pending
  TTL expires: 2024-01-28T11:00:00Z

To restore immediately: kubectl petstore patch pet fluffy --restore
```

#### `pause` - Pause/Unpause Reconciliation

```bash
# Pause a single resource
kubectl petstore pause pet fluffy

# Pause with a reason (stored in annotation)
kubectl petstore pause pet fluffy --reason="Investigating sync issues"

# Pause multiple resources
kubectl petstore pause pet fluffy buddy max

# Pause all resources of a kind
kubectl petstore pause pets --all

# Pause by label selector
kubectl petstore pause pets -l environment=staging

# Unpause a resource
kubectl petstore pause pet fluffy --unpause

# Unpause all paused resources
kubectl petstore pause --unpause --all

# List paused resources
kubectl petstore pause --list

# Pause with TTL (auto-unpause after duration)
kubectl petstore pause pet fluffy --ttl=1h

# Pause a Bundle CR (pauses child resource creation)
kubectl petstore pause bundle my-bundle

# Pause an Aggregate CR (pauses status aggregation)
kubectl petstore pause aggregate production-health
```

**Output (pause):**
```
Pausing pet/fluffy...

Resource paused successfully
  Kind:      Pet
  Name:      fluffy
  Namespace: default
  Paused:    true
  Reason:    Investigating sync issues
  Paused At: 2024-01-28T10:00:00Z

The operator will skip reconciliation for this resource until unpaused.
To unpause: kubectl petstore pause pet fluffy --unpause
```

**Output (unpause):**
```
Unpausing pet/fluffy...

Resource unpaused successfully
  Kind:       Pet
  Name:       fluffy
  Namespace:  default
  Paused:     false
  Was Paused: 2h15m

Reconciliation will resume on next sync cycle.
```

**Output (list paused):**
```
kubectl petstore pause --list

PAUSED RESOURCES (namespace: default)

KIND      NAME          PAUSED AT             REASON                        TTL
Pet       fluffy        2024-01-28T10:00:00Z  Investigating sync issues     -
Pet       buddy         2024-01-28T09:30:00Z  Manual testing                30m remaining
Order     order-456     2024-01-28T08:00:00Z  -                             -
Bundle    my-bundle     2024-01-28T10:15:00Z  Staged rollout                -

Total: 4 paused resources
```

**Output (pause all by kind):**
```
kubectl petstore pause pets --all

Pausing all Pet resources in namespace default...

Paused 5 resources:
  Pet  fluffy   (was already paused)
  Pet  buddy    paused
  Pet  max      paused
  Pet  bella    paused
  Pet  charlie  paused

4 newly paused, 1 already paused
```

**How it works:**

The `pause` command sets `spec.paused: true` on the CR. Generated controllers check this field and skip reconciliation when set:

```yaml
# Before pause
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: fluffy
  annotations: {}
spec:
  name: Fluffy
  status: available

# After pause
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: fluffy
  annotations:
    kubectl-plugin.example.com/paused-at: "2024-01-28T10:00:00Z"
    kubectl-plugin.example.com/paused-reason: "Investigating sync issues"
    kubectl-plugin.example.com/paused-ttl: "2024-01-28T11:00:00Z"  # if TTL specified
spec:
  name: Fluffy
  status: available
  paused: true  # Controller skips reconciliation when true
```

**Use cases:**
- **Troubleshooting**: Pause sync while investigating issues
- **Maintenance windows**: Pause during planned API maintenance
- **Staged rollouts**: Pause some resources while testing others
- **Manual overrides**: Allow manual API changes without operator reverting them
- **Bundle orchestration**: Pause bundle to stop child resource creation

#### `drift` - Drift Detection Report

```bash
# Show drift report
kubectl petstore drift

# Filter by kind
kubectl petstore drift --kind=Pet

# Show detailed diff
kubectl petstore drift --show-diff
```

**Output:**
```
DRIFT REPORT (namespace: default)

RESOURCE       DRIFT  COUNT  LAST DETECTED
pet/fluffy     No     0      -
pet/buddy      No     0      -
order/order-1  Yes    3      2024-01-28T09:45:00Z
order/order-2  No     0      -

DRIFT DETAILS for order/order-1:
  FIELD      SPEC VALUE    ACTUAL VALUE
  quantity   5             3
  status     approved      pending

Total: 1 resource with drift (4 total resources)
```

#### `cleanup` - Remove Diagnostic Resources

```bash
# Remove all diagnostic resources
kubectl petstore cleanup

# Remove expired TTL resources only
kubectl petstore cleanup --expired

# Dry run
kubectl petstore cleanup --dry-run

# Remove by label
kubectl petstore cleanup -l diagnostic-session=session-123
```

**Output:**
```
Cleaning up diagnostic resources...

Resources to be deleted:
  Pet      diag-pet-123-pod0      (expired TTL)
  Pet      diag-pet-123-pod1      (expired TTL)
  Query    diag-inventory-check   (label: purpose=diagnostics)

Delete 3 resources? [y/N]: y

Deleted 3 resources
```

## Implementation Details

### Template Structure

New templates to add:

```
pkg/templates/
├── kubectl_plugin/
│   ├── main.go.tmpl
│   ├── root_cmd.go.tmpl
│   ├── status_cmd.go.tmpl
│   ├── get_cmd.go.tmpl
│   ├── describe_cmd.go.tmpl
│   ├── compare_cmd.go.tmpl
│   ├── diagnose_cmd.go.tmpl
│   ├── query_cmd.go.tmpl
│   ├── action_cmd.go.tmpl
│   ├── patch_cmd.go.tmpl
│   ├── pause_cmd.go.tmpl
│   ├── drift_cmd.go.tmpl
│   ├── cleanup_cmd.go.tmpl
│   ├── client.go.tmpl
│   ├── output.go.tmpl
│   └── makefile.tmpl
```

### Template Data Model

```go
type KubectlPluginData struct {
    // From OpenAPI
    APIName      string   // e.g., "petstore"
    APIGroup     string   // e.g., "petstore.example.com"
    APIVersion   string   // e.g., "v1alpha1"

    // Generated CRD info
    ResourceKinds []KindInfo  // Pet, Order, User
    QueryKinds    []KindInfo  // PetFindbystatusQuery, StoreInventoryQuery
    ActionKinds   []KindInfo  // PetUploadimageAction

    // Plugin metadata
    PluginName    string   // e.g., "petstore"
    BinaryName    string   // e.g., "kubectl-petstore"

    // Generator info
    GeneratorVersion string
    Year             int
}

type KindInfo struct {
    Kind       string   // e.g., "Pet"
    Plural     string   // e.g., "pets"
    ShortNames []string // e.g., ["pet"]

    // For queries/actions
    Parameters []ParameterInfo
}
```

### Key Implementation Components

#### 1. Dynamic Client Wrapper

```go
// pkg/client/client.go.tmpl
type Client struct {
    dynamic   dynamic.Interface
    discovery discovery.DiscoveryInterface
    namespace string

    // Cached GVRs for generated types
    resourceGVRs map[string]schema.GroupVersionResource
}

func (c *Client) Get(ctx context.Context, kind, name string) (*unstructured.Unstructured, error)
func (c *Client) List(ctx context.Context, kind string, opts ...ListOption) (*unstructured.UnstructuredList, error)
func (c *Client) Create(ctx context.Context, kind string, obj *unstructured.Unstructured) error
func (c *Client) Patch(ctx context.Context, kind, name string, patch []byte) error
func (c *Client) Delete(ctx context.Context, kind, name string) error
```

#### 2. Output Formatter

```go
// pkg/output/output.go.tmpl
type Formatter interface {
    Format(data interface{}) (string, error)
}

type TableFormatter struct {
    Columns []Column
}

type JSONFormatter struct {
    Pretty bool
}

type YAMLFormatter struct{}

// Resource-specific formatters
func FormatResourceList(resources []Resource, format string) string
func FormatResourceDetail(resource Resource) string
func FormatComparison(results []PodResult) string
func FormatDriftReport(drifts []DriftInfo) string
```

#### 3. TTL Management

```go
// pkg/ttl/ttl.go.tmpl
const (
    TTLAnnotation      = "kubectl-plugin.example.com/ttl"
    OriginalStateAnnotation = "kubectl-plugin.example.com/original-state"
)

func SetTTL(obj *unstructured.Unstructured, duration time.Duration)
func GetTTL(obj *unstructured.Unstructured) (*time.Time, error)
func IsExpired(obj *unstructured.Unstructured) bool
func SaveOriginalState(obj *unstructured.Unstructured) error
func RestoreOriginalState(obj *unstructured.Unstructured) error
```

### Generator Flag

Add new flag to the generator:

```bash
openapi-operator-gen generate \
  --spec petstore.yaml \
  --output ./generated \
  --kubectl-plugin          # Generate kubectl plugin
```

### Generator Code Changes

```go
// pkg/generator/generator.go
type Config struct {
    // Existing fields...

    // New field
    GenerateKubectlPlugin bool
}

func (g *Generator) Generate(config Config) error {
    // Existing generation...

    if config.GenerateKubectlPlugin {
        if err := g.generateKubectlPlugin(); err != nil {
            return fmt.Errorf("failed to generate kubectl plugin: %w", err)
        }
    }

    return nil
}

func (g *Generator) generateKubectlPlugin() error {
    data := g.prepareKubectlPluginData()

    templates := []struct {
        name   string
        output string
    }{
        {"main.go.tmpl", "kubectl-plugin/main.go"},
        {"root_cmd.go.tmpl", "kubectl-plugin/cmd/root.go"},
        {"status_cmd.go.tmpl", "kubectl-plugin/cmd/status.go"},
        {"get_cmd.go.tmpl", "kubectl-plugin/cmd/get.go"},
        // ... more templates
    }

    for _, t := range templates {
        if err := g.executeTemplate(t.name, t.output, data); err != nil {
            return err
        }
    }

    return nil
}
```

## Krew Distribution

### Krew Manifest Template

```yaml
# kubectl-plugin/krew/{{ .PluginName }}.yaml.tmpl
apiVersion: krew.googlecontainertools.github.com/v1alpha2
kind: Plugin
metadata:
  name: {{ .PluginName }}
spec:
  version: {{ .Version }}
  homepage: {{ .Homepage }}
  shortDescription: "Manage {{ .APIName }} resources"
  description: |
    kubectl plugin for managing {{ .APIName }} operator resources.

    Features:
    - View aggregate health status
    - Compare resources across pods
    - Run diagnostic queries
    - Execute actions
    - Detect and report drift

    Generated by openapi-operator-gen {{ .GeneratorVersion }}
  platforms:
  - selector:
      matchLabels:
        os: darwin
        arch: amd64
    uri: https://github.com/{{ .Org }}/{{ .Repo }}/releases/download/{{ .Version }}/kubectl-{{ .PluginName }}_darwin_amd64.tar.gz
    sha256: "{{ .SHA256Darwin }}"
    bin: kubectl-{{ .PluginName }}
  - selector:
      matchLabels:
        os: darwin
        arch: arm64
    uri: https://github.com/{{ .Org }}/{{ .Repo }}/releases/download/{{ .Version }}/kubectl-{{ .PluginName }}_darwin_arm64.tar.gz
    sha256: "{{ .SHA256DarwinArm }}"
    bin: kubectl-{{ .PluginName }}
  - selector:
      matchLabels:
        os: linux
        arch: amd64
    uri: https://github.com/{{ .Org }}/{{ .Repo }}/releases/download/{{ .Version }}/kubectl-{{ .PluginName }}_linux_amd64.tar.gz
    sha256: "{{ .SHA256Linux }}"
    bin: kubectl-{{ .PluginName }}
```

### Makefile for Plugin

```makefile
# kubectl-plugin/Makefile.tmpl
PLUGIN_NAME := kubectl-{{ .PluginName }}
VERSION ?= $(shell git describe --tags --always --dirty)

.PHONY: build
build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(PLUGIN_NAME) .

.PHONY: install
install: build
	cp bin/$(PLUGIN_NAME) $(shell go env GOPATH)/bin/

.PHONY: release
release:
	GOOS=darwin GOARCH=amd64 go build -o dist/$(PLUGIN_NAME)_darwin_amd64 .
	GOOS=darwin GOARCH=arm64 go build -o dist/$(PLUGIN_NAME)_darwin_arm64 .
	GOOS=linux GOARCH=amd64 go build -o dist/$(PLUGIN_NAME)_linux_amd64 .
	cd dist && tar -czvf $(PLUGIN_NAME)_darwin_amd64.tar.gz $(PLUGIN_NAME)_darwin_amd64
	cd dist && tar -czvf $(PLUGIN_NAME)_darwin_arm64.tar.gz $(PLUGIN_NAME)_darwin_arm64
	cd dist && tar -czvf $(PLUGIN_NAME)_linux_amd64.tar.gz $(PLUGIN_NAME)_linux_amd64
```

## Usage Examples

### Example Session

```bash
# Check overall health
$ kubectl petstore status
AGGREGATE           HEALTH     SYNCED  FAILED  PENDING
production-health   Degraded   14      1       0

# Find the failing resource
$ kubectl petstore get pets --state=Failed
NAME   STATE   EXTERNAL-ID  AGE
max    Failed  -            5m

# Diagnose the issue
$ kubectl petstore diagnose pet max
Running diagnostics for pet/max...

CHECKS:
[✓] Resource exists in cluster
[✗] API sync failed: connection refused
[!] Target pod petstore-api-2 may be unhealthy

# Compare across pods
$ kubectl petstore compare pet fluffy --pods=0,1,2
FIELD         POD-0    POD-1    POD-2
state         Synced   Synced   Failed (timeout)
externalID    123      123      -

# Run health check query
$ kubectl petstore query inventory --pod=0
Results: {"approved": 10, "pending": 5, "delivered": 3}

# Pause a problematic resource while investigating
$ kubectl petstore pause pet max --reason="Investigating sync failure"
Resource paused successfully

# Check paused resources
$ kubectl petstore pause --list
KIND  NAME  PAUSED AT             REASON
Pet   max   2024-01-28T10:00:00Z  Investigating sync failure

# Unpause after fixing
$ kubectl petstore pause pet max --unpause
Resource unpaused successfully

# Cleanup diagnostic resources
$ kubectl petstore cleanup --expired
Deleted 2 expired resources
```

## Implementation Phases

### Phase 1: Core Commands
- `status` - Aggregate health view
- `get` - List resources with state
- `describe` - Detailed resource view

### Phase 2: Diagnostic Commands
- `compare` - Multi-pod comparison
- `diagnose` - Resource diagnostics
- `drift` - Drift detection report

### Phase 3: Interactive Commands
- `query` - Execute query CRDs
- `action` - Execute action CRDs
- `patch` - Temporary changes with TTL
- `pause` - Pause/unpause reconciliation
- `cleanup` - Resource cleanup

### Phase 4: Distribution
- Krew manifest generation
- GitHub Actions for releases
- Documentation generation

## Dependencies

The generated plugin will use:
- `github.com/spf13/cobra` - CLI framework
- `k8s.io/client-go` - Kubernetes client
- `k8s.io/apimachinery` - K8s types
- `sigs.k8s.io/yaml` - YAML handling
- `github.com/olekukonko/tablewriter` - Table output
- `github.com/fatih/color` - Colored output

## Summary

The kubectl plugin generator provides:

1. **Native K8s Experience**: Uses standard kubectl plugin conventions
2. **Rich CLI**: Subcommands for all diagnostic workflows
3. **Generated from Spec**: All commands derived from OpenAPI specification
4. **Krew Compatible**: Easy distribution via krew
5. **Minimal Dependencies**: Uses standard K8s client libraries
