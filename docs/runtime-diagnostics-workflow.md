# Runtime Diagnostics and Troubleshooting Workflow

This document describes a model and workflow for using generated CRDs to determine application state, effect runtime changes for diagnostics, troubleshooting, and other temporary purposes.

## Table of Contents

- [Overview](#overview)
- [Application State Model](#application-state-model)
- [CRD Types for Diagnostics](#crd-types-for-diagnostics)
- [Diagnostic Workflows](#diagnostic-workflows)
- [CLI Interaction Options](#cli-interaction-options)
- [UI Interaction Options](#ui-interaction-options)
- [Implementation Recommendations](#implementation-recommendations)

## Overview

Generated operators create a Kubernetes-native control plane for REST API resources. This control plane can be leveraged for:

| Use Case | Description | CRD Types Used |
|----------|-------------|----------------|
| **State Observation** | View current application state | Resource CRDs (ReadOnly), Query CRDs |
| **Drift Detection** | Identify configuration drift | Resource CRDs, Aggregate CRDs |
| **Diagnostics** | Query application health/metrics | Query CRDs, Action CRDs |
| **Troubleshooting** | Make temporary runtime changes | Resource CRDs, Action CRDs |
| **Multi-Instance Comparison** | Compare state across replicas | Per-pod targeting, Aggregate CRDs |

### Key Advantages

1. **Declarative State**: Application state is represented as Kubernetes resources
2. **Version Control**: Changes can be tracked via GitOps
3. **Audit Trail**: Kubernetes audit logs capture all modifications
4. **RBAC**: Fine-grained access control via Kubernetes RBAC
5. **Rollback**: Easy rollback via `kubectl apply` of previous state
6. **Automation**: Integrate with existing K8s tooling (operators, controllers)

## Application State Model

### State Hierarchy

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        APPLICATION STATE MODEL                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     Aggregate CRD (Top Level)                        │   │
│  │  PetstoreAggregate: "production-health"                              │   │
│  │                                                                      │   │
│  │  status:                                                             │   │
│  │    aggregatedHealth: Healthy/Degraded/Progressing                    │   │
│  │    summary: { total: 15, synced: 14, failed: 1 }                     │   │
│  │    computedValues:                                                   │   │
│  │      - healthPercentage: "93%"                                       │   │
│  │      - avgResponseTime: "45ms"                                       │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                      │                                      │
│                     ┌────────────────┼────────────────┐                    │
│                     │                │                │                    │
│                     ▼                ▼                ▼                    │
│  ┌─────────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐  │
│  │   Resource CRDs     │ │    Query CRDs       │ │   Action CRDs       │  │
│  │                     │ │                     │ │                     │  │
│  │ Pet, Order, User    │ │ PetFindbystatusQuery│ │ PetUploadimageAction│  │
│  │                     │ │ StoreInventoryQuery │ │ UserCreatewithlist  │  │
│  │ State:              │ │                     │ │                     │  │
│  │ - Synced            │ │ State:              │ │ State:              │  │
│  │ - Failed            │ │ - Queried           │ │ - Completed         │  │
│  │ - Pending           │ │ - Failed            │ │ - Executing         │  │
│  │ - NotFound          │ │                     │ │ - Failed            │  │
│  │                     │ │ Results in status   │ │                     │  │
│  │ DriftDetected: bool │ │                     │ │ ReExecuteInterval   │  │
│  └─────────────────────┘ └─────────────────────┘ └─────────────────────┘  │
│           │                        │                        │              │
│           │                        │                        │              │
│           ▼                        ▼                        ▼              │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                      Per-Pod/Instance Targeting                      │   │
│  │                                                                      │   │
│  │  targetPod: "petstore-api-0"     # Specific pod                     │   │
│  │  targetPodOrdinal: 2             # StatefulSet ordinal              │   │
│  │  targetBaseURL: "http://..."     # Direct URL                       │   │
│  │                                                                      │   │
│  │  Enables: Per-instance diagnostics, A/B comparison, isolation       │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### State Representation

Each CRD captures different aspects of application state:

**Resource CRDs** (Pet, Order, User):
```yaml
status:
  state: Synced              # Current sync state
  externalID: "12345"        # ID in REST API
  lastSyncTime: "..."        # Last successful sync
  lastGetTime: "..."         # Last GET request
  driftDetected: false       # Spec vs API mismatch
  driftDetectedCount: 0      # Cumulative drift count
  response:
    success: true
    statusCode: 200
    data: { ... }            # Full API response
```

**Query CRDs** (PetFindbystatusQuery, StoreInventoryQuery):
```yaml
status:
  state: Queried
  lastQueryTime: "..."
  queryCount: 42             # Total queries executed
  results:                   # Query results
    - { id: 1, name: "Fluffy", status: "available" }
    - { id: 2, name: "Buddy", status: "pending" }
```

**Action CRDs** (PetUploadimageAction):
```yaml
status:
  state: Completed
  executionCount: 1
  lastExecutionTime: "..."
  result:
    success: true
    statusCode: 200
    data: { ... }
```

## CRD Types for Diagnostics

### 1. Read-Only Resource Observation

Use `readOnly: true` to observe existing resources without modifications:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: observe-pet-123
  labels:
    purpose: diagnostics
    temporary: "true"
spec:
  id: 123                    # Existing resource ID
  readOnly: true             # Observation only - no mutations
  targetPod: petstore-api-0  # Target specific instance
```

**Use cases:**
- Inspect current state of a resource
- Compare state across multiple pods
- Verify data consistency

### 2. Query CRDs for Health Checks

```yaml
# Periodic health/status query
apiVersion: petstore.example.com/v1alpha1
kind: StoreInventoryQuery
metadata:
  name: inventory-health-check
  labels:
    purpose: health-check
spec:
  targetHelmRelease: petstore
---
# Find resources in specific state
apiVersion: petstore.example.com/v1alpha1
kind: PetFindbystatusQuery
metadata:
  name: find-pending-pets
spec:
  status: pending
```

### 3. Action CRDs for Diagnostic Operations

```yaml
# One-shot diagnostic action
apiVersion: petstore.example.com/v1alpha1
kind: PetUploadimageAction
metadata:
  name: diagnostic-action
  labels:
    purpose: troubleshooting
    ttl: 1h                  # Convention for cleanup
spec:
  petId: "123"
  additionalMetadata: "diagnostic-run-2024-01-27"
---
# Periodic diagnostic action
apiVersion: petstore.example.com/v1alpha1
kind: PetUploadimageAction
metadata:
  name: periodic-health-probe
spec:
  petId: "health-check"
  reExecuteInterval: 5m      # Re-run every 5 minutes
```

### 4. Per-Instance Targeting for Isolation

```yaml
# Target specific pod for comparison
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: pod-0-pet-123
spec:
  id: 123
  readOnly: true
  targetPodOrdinal: 0        # Pod petstore-api-0
---
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: pod-1-pet-123
spec:
  id: 123
  readOnly: true
  targetPodOrdinal: 1        # Pod petstore-api-1
```

### 5. Aggregate CRDs for Unified View

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreAggregate
metadata:
  name: production-health
spec:
  resourceSelectors:
    - kind: Pet
      matchLabels:
        environment: production
    - kind: Order
      matchLabels:
        environment: production
  derivedValues:
    - name: healthPercentage
      expression: "summary.synced * 100 / summary.total"
    - name: failedResources
      expression: "resources.filter(r, r.status.state == 'Failed').map(r, r.name)"
```

## Diagnostic Workflows

### Workflow 1: Investigate Failing Resource

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     WORKFLOW: Investigate Failing Resource                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  1. IDENTIFY: Check aggregate for failed resources                          │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ $ kubectl get petstoreaggregate production-health -o yaml       │    │
│     │                                                                  │    │
│     │ status:                                                          │    │
│     │   summary: { failed: 1 }                                         │    │
│     │   resources:                                                     │    │
│     │     - kind: Order                                                │    │
│     │       name: order-456                                            │    │
│     │       health: { status: Degraded }                               │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                      │                                      │
│                                      ▼                                      │
│  2. INSPECT: Get detailed status of failing resource                        │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ $ kubectl get order order-456 -o yaml                            │    │
│     │                                                                  │    │
│     │ status:                                                          │    │
│     │   state: Failed                                                  │    │
│     │   message: "API returned 500: Internal Server Error"             │    │
│     │   response:                                                      │    │
│     │     statusCode: 500                                              │    │
│     │     error: "Database connection failed"                          │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                      │                                      │
│                                      ▼                                      │
│  3. COMPARE: Check same resource on different pods                          │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ # Create read-only observers for each pod                        │    │
│     │ $ kubectl apply -f - <<EOF                                       │    │
│     │ apiVersion: petstore.example.com/v1alpha1                        │    │
│     │ kind: Order                                                      │    │
│     │ metadata:                                                        │    │
│     │   name: diag-order-456-pod0                                      │    │
│     │ spec:                                                            │    │
│     │   id: 456                                                        │    │
│     │   readOnly: true                                                 │    │
│     │   targetPodOrdinal: 0                                            │    │
│     │ EOF                                                              │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                      │                                      │
│                                      ▼                                      │
│  4. DIAGNOSE: Run diagnostic query/action                                   │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ $ kubectl apply -f - <<EOF                                       │    │
│     │ apiVersion: petstore.example.com/v1alpha1                        │    │
│     │ kind: StoreInventoryQuery                                        │    │
│     │ metadata:                                                        │    │
│     │   name: diag-inventory-check                                     │    │
│     │ spec:                                                            │    │
│     │   targetPodOrdinal: 0  # Check specific pod                      │    │
│     │ EOF                                                              │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                      │                                      │
│                                      ▼                                      │
│  5. CLEANUP: Remove diagnostic resources                                    │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ $ kubectl delete order,query -l purpose=diagnostics              │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Workflow 2: Temporary Configuration Change

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                   WORKFLOW: Temporary Configuration Change                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  1. SNAPSHOT: Capture current state                                         │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ $ kubectl get pet fluffy -o yaml > fluffy-original.yaml          │    │
│     │ $ kubectl annotate pet fluffy \                                  │    │
│     │     diagnostics.example.com/original-state="$(date -Iseconds)"   │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                      │                                      │
│                                      ▼                                      │
│  2. MODIFY: Apply temporary change                                          │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ $ kubectl patch pet fluffy --type=merge -p '{                    │    │
│     │     "metadata": {                                                │    │
│     │       "labels": { "temporary-change": "true" },                  │    │
│     │       "annotations": {                                           │    │
│     │         "diagnostics.example.com/reason": "testing new status",  │    │
│     │         "diagnostics.example.com/ttl": "2024-01-27T18:00:00Z"    │    │
│     │       }                                                          │    │
│     │     },                                                           │    │
│     │     "spec": { "status": "pending" }                              │    │
│     │   }'                                                             │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                      │                                      │
│                                      ▼                                      │
│  3. VERIFY: Check change was applied                                        │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ $ kubectl wait --for=jsonpath='{.status.state}'=Synced \         │    │
│     │     pet/fluffy --timeout=60s                                     │    │
│     │ $ kubectl get pet fluffy -o jsonpath='{.status.response.data}'   │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                      │                                      │
│                                      ▼                                      │
│  4. ROLLBACK: Restore original state                                        │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ $ kubectl apply -f fluffy-original.yaml                          │    │
│     │ # Or use OnDelete: Restore for automatic rollback on CR deletion │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Workflow 3: Multi-Instance State Comparison

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                  WORKFLOW: Multi-Instance State Comparison                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  1. CREATE: Observer CRs for each instance                                  │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ # Generate observers for all pods in StatefulSet                 │    │
│     │ for i in 0 1 2; do                                               │    │
│     │   kubectl apply -f - <<EOF                                       │    │
│     │   apiVersion: petstore.example.com/v1alpha1                      │    │
│     │   kind: StoreInventoryQuery                                      │    │
│     │   metadata:                                                      │    │
│     │     name: inventory-pod-$i                                       │    │
│     │     labels:                                                      │    │
│     │       comparison-set: inventory-check                            │    │
│     │   spec:                                                          │    │
│     │     targetPodOrdinal: $i                                         │    │
│     │   EOF                                                            │    │
│     │ done                                                             │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                      │                                      │
│                                      ▼                                      │
│  2. WAIT: For all queries to complete                                       │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ $ kubectl wait --for=jsonpath='{.status.state}'=Queried \        │    │
│     │     storeinventoryquery -l comparison-set=inventory-check \      │    │
│     │     --timeout=120s                                               │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                      │                                      │
│                                      ▼                                      │
│  3. COMPARE: Extract and diff results                                       │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ $ for q in $(kubectl get storeinventoryquery \                   │    │
│     │     -l comparison-set=inventory-check -o name); do               │    │
│     │   echo "=== $q ==="                                              │    │
│     │   kubectl get $q -o jsonpath='{.status.results}' | jq .         │    │
│     │ done                                                             │    │
│     │                                                                  │    │
│     │ # Or use aggregate with CEL for automated comparison:            │    │
│     │ $ kubectl get petstoreaggregate inventory-comparison \           │    │
│     │     -o jsonpath='{.status.computedValues}'                       │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                      │                                      │
│                                      ▼                                      │
│  4. CLEANUP                                                                 │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │ $ kubectl delete storeinventoryquery \                           │    │
│     │     -l comparison-set=inventory-check                            │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## CLI Interaction Options

### Option 1: kubectl Plugin (krew)

A dedicated kubectl plugin provides the best native experience.

**Installation:**
```bash
kubectl krew install api-diag
```

**Usage:**
```bash
# View application state
kubectl api-diag status petstore

# Compare resource across pods
kubectl api-diag compare pet/fluffy --pods=0,1,2

# Run diagnostic query
kubectl api-diag query inventory --target-pod=0

# Make temporary change with auto-rollback
kubectl api-diag patch pet/fluffy --spec='{"status":"pending"}' --ttl=1h

# View drift report
kubectl api-diag drift-report --namespace=production
```

**Plugin Implementation Structure:**
```
kubectl-api-diag/
├── cmd/
│   ├── root.go
│   ├── status.go        # Show aggregate health
│   ├── compare.go       # Multi-pod comparison
│   ├── query.go         # Run diagnostic queries
│   ├── patch.go         # Temporary changes with TTL
│   ├── drift.go         # Drift detection report
│   └── cleanup.go       # Remove diagnostic resources
├── pkg/
│   ├── client/          # K8s client wrapper
│   ├── comparison/      # State comparison logic
│   ├── output/          # Table/JSON/YAML formatters
│   └── ttl/             # TTL tracking and cleanup
└── main.go
```

**Key Commands:**

```go
// status command - shows aggregate health
type StatusCmd struct {
    Namespace string
    Output    string // table, json, yaml
    Watch     bool
}

// Output:
// AGGREGATE          HEALTH      SYNCED  FAILED  PENDING  MESSAGE
// production-health  Degraded    14      1       0        1 resource failed
//
// FAILED RESOURCES:
// KIND   NAME       STATE   MESSAGE
// Order  order-456  Failed  API returned 500

// compare command - multi-pod state comparison
type CompareCmd struct {
    Resource  string   // pet/fluffy
    Pods      []int    // pod ordinals to compare
    Field     string   // specific field to compare (optional)
    Namespace string
}

// Output:
// FIELD              POD-0           POD-1           POD-2
// status.state       Synced          Synced          Failed
// status.response    {...}           {...}           error: timeout
// response.data.id   123             123             -

// query command - run diagnostic query
type QueryCmd struct {
    Kind       string
    Params     map[string]string
    TargetPod  int
    Namespace  string
    Watch      bool
}

// drift command - show drift report
type DriftCmd struct {
    Namespace string
    Kind      string // filter by kind
    Output    string
}

// Output:
// RESOURCE       DRIFT  FIELD           SPEC VALUE    ACTUAL VALUE
// pet/fluffy     Yes    status          available     pending
// order/order-1  No     -               -             -
```

### Option 2: Dedicated CLI Tool

A standalone CLI for more complex workflows.

**Installation:**
```bash
# Via go install
go install github.com/org/petstore-cli@latest

# Via Homebrew
brew install petstore-cli

# Via release binary
curl -LO https://github.com/org/petstore-cli/releases/latest/download/petstore-cli
chmod +x petstore-cli && mv petstore-cli /usr/local/bin/
```

**Usage:**
```bash
# Initialize (discovers CRDs and API)
petstore-cli init --kubeconfig ~/.kube/config

# Interactive mode
petstore-cli shell

# Status overview
petstore-cli status --watch

# Resource operations
petstore-cli get pets
petstore-cli get pet fluffy --show-response
petstore-cli describe order order-456

# Diagnostics
petstore-cli diagnose pet fluffy
petstore-cli compare pet fluffy --across-pods
petstore-cli health-check --all

# Temporary changes
petstore-cli edit pet fluffy --temporary --ttl=1h
petstore-cli patch pet fluffy status=pending --rollback-on-exit

# Queries
petstore-cli query find-by-status --status=pending
petstore-cli query inventory --target-pod=0

# Actions
petstore-cli action upload-image --pet-id=123 --once
petstore-cli action upload-image --pet-id=123 --every=5m

# Bulk operations
petstore-cli export --namespace=production > state.yaml
petstore-cli import state.yaml --dry-run
petstore-cli diff state.yaml
```

**Interactive Shell Mode:**
```
petstore> status
Aggregate: production-health
Health: Healthy (15/15 synced)

petstore> get pets
NAME     STATE   EXTERNAL-ID  AGE
fluffy   Synced  123          2d
buddy    Synced  124          1d

petstore> describe pet fluffy
Name:         fluffy
State:        Synced
External ID:  123
Last Sync:    2024-01-27T10:00:00Z
Drift:        No

Response Data:
  id: 123
  name: Fluffy
  status: available
  category: { id: 1, name: "Dogs" }

petstore> diagnose pet fluffy
Running diagnostics for pet/fluffy...

[OK] Resource synced successfully
[OK] No drift detected
[OK] API response healthy (200 OK)
[OK] Response time: 45ms
[WARN] Resource modified 2 days ago (consider refreshing)

petstore> compare pet fluffy --pods 0,1,2
Comparing pet/fluffy across pods 0, 1, 2...

Field              Pod-0    Pod-1    Pod-2
status.state       Synced   Synced   Synced
response.data.id   123      123      123
response.data.name Fluffy   Fluffy   Fluffy

Result: All pods consistent

petstore> exit
```

### Option 3: kubectl + jq/yq Scripts

For environments where custom tools aren't allowed.

**Script Library:**
```bash
#!/bin/bash
# petstore-diag.sh - Diagnostic utilities for Petstore operator

# Get aggregate health
function ps-health() {
    kubectl get petstoreaggregate -o json | jq -r '
        .items[] |
        "\(.metadata.name)\t\(.status.aggregatedHealth.status // "Unknown")\t\(.status.summary | "\(.synced)/\(.total)")"
    ' | column -t -s$'\t'
}

# List failed resources
function ps-failed() {
    kubectl get pets,orders,users -o json | jq -r '
        .items[] |
        select(.status.state == "Failed") |
        "\(.kind)\t\(.metadata.name)\t\(.status.message)"
    ' | column -t -s$'\t'
}

# Compare resource across pods
function ps-compare() {
    local kind=$1
    local name=$2
    local pods=${3:-"0 1 2"}

    for pod in $pods; do
        echo "=== Pod $pod ==="
        kubectl get $kind $name-pod$pod -o jsonpath='{.status.response.data}' 2>/dev/null | jq . || echo "Not found"
    done
}

# Create read-only observer
function ps-observe() {
    local kind=$1
    local id=$2
    local pod=$3

    kubectl apply -f - <<EOF
apiVersion: petstore.example.com/v1alpha1
kind: ${kind^}
metadata:
  name: diag-${kind}-${id}-pod${pod}
  labels:
    purpose: diagnostics
spec:
  id: $id
  readOnly: true
  targetPodOrdinal: $pod
EOF
}

# Cleanup diagnostic resources
function ps-cleanup() {
    kubectl delete pets,orders,users,queries,actions -l purpose=diagnostics
}

# Drift report
function ps-drift() {
    kubectl get pets,orders,users -o json | jq -r '
        .items[] |
        select(.status.driftDetected == true) |
        "\(.kind)\t\(.metadata.name)\t\(.status.driftDetectedCount) times"
    ' | column -t -s$'\t'
}
```

## UI Interaction Options

### Option 1: Backstage Plugin

Integrate with [Backstage](https://backstage.io/) developer portal.

**Features:**
- Service catalog integration
- Real-time state visualization
- Diagnostic action buttons
- Drift alerts and notifications

**Plugin Structure:**
```
backstage-plugin-petstore/
├── src/
│   ├── components/
│   │   ├── PetstoreOverview/       # Dashboard widget
│   │   ├── ResourceList/           # CRD list view
│   │   ├── ResourceDetail/         # Single resource view
│   │   ├── DiagnosticPanel/        # Run diagnostics
│   │   ├── ComparisonView/         # Multi-pod comparison
│   │   └── DriftAlert/             # Drift notifications
│   ├── api/
│   │   └── PetstoreApiClient.ts    # K8s API client
│   └── plugin.ts
├── package.json
└── README.md
```

**Dashboard View:**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│  PETSTORE OPERATOR                                            [Refresh] [⚙]│
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────┐  ┌─────────────────────────────────┐  │
│  │  HEALTH STATUS                  │  │  QUICK ACTIONS                  │  │
│  │  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ │  │  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ │  │
│  │  ● Healthy                      │  │  [Run Health Check]             │  │
│  │                                 │  │  [Query Inventory]              │  │
│  │  Synced:  14 ████████████░░  │  │  [Compare Pods]                 │  │
│  │  Pending:  1 █░░░░░░░░░░░░░░  │  │  [Export State]                 │  │
│  │  Failed:   0 ░░░░░░░░░░░░░░░  │  │                                 │  │
│  └─────────────────────────────────┘  └─────────────────────────────────┘  │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  RESOURCES                                                [Filter ▼] │   │
│  │  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ │   │
│  │  KIND     NAME         STATE    EXTERNAL-ID  DRIFT  LAST SYNC      │   │
│  │  ──────────────────────────────────────────────────────────────────│   │
│  │  Pet      fluffy       Synced   123          No     2m ago         │   │
│  │  Pet      buddy        Synced   124          No     2m ago         │   │
│  │  Order    order-789    Synced   789          Yes    5m ago    ⚠️   │   │
│  │  User     john         Synced   456          No     1m ago         │   │
│  │                                                                     │   │
│  │  [View All] [Show Failed Only] [Show Drift Only]                    │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  RECENT ACTIVITY                                                    │   │
│  │  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ │   │
│  │  10:05  Pet fluffy synced successfully                              │   │
│  │  10:04  Drift detected on Order order-789                           │   │
│  │  10:02  Query inventory-check completed (45ms)                      │   │
│  │  10:00  Health check passed (15/15 healthy)                         │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Option 2: Kubernetes Dashboard Extension

Custom views for standard Kubernetes Dashboard.

**CRD List View:**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Pets (petstore.example.com/v1alpha1)                         namespace: ▼ │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────┬──────────┬─────────┬─────────────┬───────┬───────────────────┐    │
│  │     │ NAME     │ STATE   │ EXTERNAL-ID │ DRIFT │ AGE               │    │
│  ├─────┼──────────┼─────────┼─────────────┼───────┼───────────────────┤    │
│  │ ● │ fluffy   │ Synced  │ 123         │ -     │ 2d                │    │
│  │ ● │ buddy    │ Synced  │ 124         │ -     │ 1d                │    │
│  │ ◐ │ max      │ Pending │ -           │ -     │ 5m                │    │
│  │ ● │ bella    │ Synced  │ 125         │ Yes   │ 3d                │    │
│  └─────┴──────────┴─────────┴─────────────┴───────┴───────────────────┘    │
│                                                                             │
│  [+ Create] [⟳ Refresh] [🗑 Delete Selected]                               │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Option 3: Lens/OpenLens Extension

Extension for Lens IDE.

**Features:**
- Integrated CRD management
- Real-time status updates
- One-click diagnostics
- Visual diff for drift detection

### Option 4: Headlamp Plugin

Plugin for [Headlamp](https://headlamp.dev/) dashboard.

### Option 5: Custom Web UI

Dedicated React-based dashboard.

**Architecture:**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CUSTOM WEB UI ARCHITECTURE                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         Frontend (React)                             │   │
│  │  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐           │   │
│  │  │ Dashboard │ │ Resources │ │ Diagnose  │ │ Compare   │           │   │
│  │  │   View    │ │   List    │ │   Panel   │ │   View    │           │   │
│  │  └───────────┘ └───────────┘ └───────────┘ └───────────┘           │   │
│  │                              │                                       │   │
│  │                              ▼                                       │   │
│  │  ┌───────────────────────────────────────────────────────────────┐  │   │
│  │  │                    WebSocket Connection                        │  │   │
│  │  │              (Real-time K8s watch events)                      │  │   │
│  │  └───────────────────────────────────────────────────────────────┘  │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                      │                                      │
│                                      ▼                                      │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                      Backend (Go / Node.js)                          │   │
│  │                                                                      │   │
│  │  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐           │   │
│  │  │  K8s Client   │  │  Watch Cache  │  │  Auth/RBAC    │           │   │
│  │  │  (Dynamic)    │  │  (Informers)  │  │  (OIDC/Token) │           │   │
│  │  └───────────────┘  └───────────────┘  └───────────────┘           │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                      │                                      │
│                                      ▼                                      │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     Kubernetes API Server                            │   │
│  │                   (Generated CRDs + Operator)                        │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Tech Stack:**
- Frontend: React + TypeScript + TailwindCSS
- State: React Query (for K8s API caching)
- Real-time: WebSocket via kubernetes-client/javascript
- Backend: Optional - can use kubectl proxy or in-cluster service account
- Auth: OIDC, service account tokens, or kubeconfig

### Option 6: Grafana Dashboard

For teams already using Grafana for observability.

**Features:**
- Time-series view of resource states
- Alert integration for failures/drift
- Query from Prometheus metrics emitted by operator

**Dashboard Panels:**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│  PETSTORE OPERATOR METRICS                                    Last 24h ▼   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────────────────────────┐  ┌──────────────────────────────────┐│
│  │  Sync Success Rate               │  │  Resource States Over Time       ││
│  │  ┌────────────────────────────┐  │  │  ┌────────────────────────────┐  ││
│  │  │         99.2%              │  │  │  │████████████████████████████│  ││
│  │  │  ━━━━━━━━━━━━━━━━━━━━━━━  │  │  │  │████████████████░░░░░░░░░░│  ││
│  │  │  ▲                         │  │  │  │████████████░░░░░░░░░░░░░░│  ││
│  │  │  │  ████████████████████   │  │  │  └────────────────────────────┘  ││
│  │  │  └──────────────────────▶  │  │  │  ■ Synced  ■ Pending  ■ Failed  ││
│  │  └────────────────────────────┘  │  └──────────────────────────────────┘│
│  └──────────────────────────────────┘                                       │
│                                                                             │
│  ┌──────────────────────────────────┐  ┌──────────────────────────────────┐│
│  │  Drift Events                    │  │  API Response Times              ││
│  │  ┌────────────────────────────┐  │  │  ┌────────────────────────────┐  ││
│  │  │  │    │         │          │  │  │  │    ╱╲    ╱╲               │  ││
│  │  │  │    │         │    │     │  │  │  │   ╱  ╲  ╱  ╲    ╱╲       │  ││
│  │  │  │    │    │    │    │     │  │  │  │  ╱    ╲╱    ╲  ╱  ╲      │  ││
│  │  │  └────────────────────────▶│  │  │  │ ╱            ╲╱    ╲─────│  ││
│  │  └────────────────────────────┘  │  │  └────────────────────────────┘  ││
│  │  Total: 12 drift events          │  │  p50: 45ms  p99: 120ms          ││
│  └──────────────────────────────────┘  └──────────────────────────────────┘│
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Recent Failures (from Loki logs)                                    │   │
│  │  ──────────────────────────────────────────────────────────────────  │   │
│  │  10:45:23  order-456  Failed  "API returned 500: DB connection..."   │   │
│  │  09:12:01  pet-789    Failed  "Timeout after 30s"                    │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Implementation Recommendations

### Recommended CLI Approach

| Audience | Recommendation | Reason |
|----------|----------------|--------|
| **K8s Power Users** | kubectl plugin (krew) | Native experience, no new tools |
| **Application Teams** | Dedicated CLI | Simpler commands, guided workflows |
| **Automation/CI** | kubectl + scripts | Scriptable, no dependencies |

### Recommended UI Approach

| Environment | Recommendation | Reason |
|-------------|----------------|--------|
| **Backstage Users** | Backstage plugin | Integrated developer portal |
| **K8s Dashboard Users** | Dashboard extension | Familiar interface |
| **IDE Users** | Lens/OpenLens extension | Development workflow integration |
| **Observability Focus** | Grafana dashboard | Unified metrics view |
| **Standalone Need** | Custom Web UI | Full control, custom workflows |

### Generator Enhancements

The generator could produce CLI and UI artifacts:

```bash
openapi-operator-gen \
  --spec petstore.yaml \
  --output ./generated \
  --kubectl-plugin           # Generate kubectl plugin
  --backstage-plugin         # Generate Backstage plugin scaffold
  --grafana-dashboard        # Generate Grafana dashboard JSON
```

### Labeling Convention for Diagnostic Resources

```yaml
metadata:
  labels:
    # Purpose classification
    purpose: diagnostics           # diagnostics, troubleshooting, testing
    temporary: "true"              # Mark for cleanup

    # Grouping
    diagnostic-session: "session-123"
    comparison-set: "inventory-check"

  annotations:
    # TTL for automatic cleanup
    diagnostics.example.com/ttl: "2024-01-27T18:00:00Z"

    # Tracking
    diagnostics.example.com/created-by: "user@example.com"
    diagnostics.example.com/reason: "investigating order failures"
    diagnostics.example.com/ticket: "JIRA-1234"
```

### Cleanup Controller

Consider a cleanup controller that removes diagnostic resources:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: DiagnosticCleanupPolicy
metadata:
  name: default-cleanup
spec:
  # Remove resources with expired TTL
  ttlExpired: true

  # Remove resources older than 24h with temporary=true label
  maxAge: 24h
  labelSelector:
    matchLabels:
      temporary: "true"

  # Exclude specific resources
  excludeLabels:
    keep: "true"
```

## Summary

| Component | Primary Use Case | Key Features |
|-----------|-----------------|--------------|
| **Resource CRDs (ReadOnly)** | State observation | Per-pod targeting, drift detection |
| **Query CRDs** | Health checks, diagnostics | Periodic queries, typed results |
| **Action CRDs** | One-shot operations | Re-execute interval, result capture |
| **Aggregate CRDs** | Unified health view | CEL expressions, status summary |
| **kubectl Plugin** | K8s-native CLI | compare, diagnose, patch commands |
| **Dedicated CLI** | Application teams | Interactive shell, guided workflows |
| **Backstage Plugin** | Developer portal | Visual dashboard, action buttons |
| **Grafana Dashboard** | Observability teams | Time-series metrics, alerts |

The combination of generated CRDs and appropriate tooling creates a powerful runtime control plane for application diagnostics and troubleshooting.
