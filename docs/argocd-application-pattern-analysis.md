# ArgoCD Application Pattern Analysis

## Overview

ArgoCD's Application CRD is a mature, battle-tested pattern for aggregating and managing multiple Kubernetes resources as a single unit. It provides health aggregation, sync status tracking, and progressive deployment capabilities that are directly applicable to the aggregate CRD design.

**Key Characteristics:**
- Declarative application definition pointing to a Git source
- Health status aggregation from all child resources
- Sync status tracking (desired vs actual state)
- Progressive sync with waves and hooks
- Resource tracking via annotations or labels

## Core Design: Application CRD

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-application
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/example/app
    targetRevision: HEAD
    path: manifests
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
status:
  sync:
    status: Synced          # OutOfSync, Synced, Unknown
    revision: abc123
  health:
    status: Healthy         # Healthy, Progressing, Degraded, Suspended, Missing, Unknown
  operationState:
    phase: Succeeded        # Running, Terminating, Failed, Error, Succeeded
    message: "successfully synced"
  resources:
    - group: apps
      kind: Deployment
      name: web
      status: Synced
      health:
        status: Healthy
    - group: ""
      kind: Service
      name: web-svc
      status: Synced
      health:
        status: Healthy
  conditions:
    - type: SyncError
      status: "False"
```

## Health Aggregation Pattern

ArgoCD implements sophisticated health aggregation with clear rules:

### Health Status Hierarchy

| Status | Description | Aggregation Rule |
|--------|-------------|------------------|
| `Healthy` | Resource is fully healthy | All resources must be Healthy |
| `Progressing` | Resource is still starting up | Any resource Progressing (but none Degraded) |
| `Degraded` | Resource has failed | Any resource Degraded |
| `Suspended` | Resource is paused | Any resource Suspended (but none Degraded) |
| `Missing` | Resource doesn't exist | Any resource Missing |
| `Unknown` | Health cannot be determined | Default when no specific rule matches |

### Health Assessment Logic

```go
// Simplified ArgoCD health aggregation logic
func aggregateHealth(resources []ResourceHealth) HealthStatus {
    // Priority order: Degraded > Missing > Progressing > Suspended > Unknown > Healthy
    hasDegraded := false
    hasMissing := false
    hasProgressing := false
    hasSuspended := false
    hasUnknown := false

    for _, r := range resources {
        switch r.Status {
        case Degraded:
            hasDegraded = true
        case Missing:
            hasMissing = true
        case Progressing:
            hasProgressing = true
        case Suspended:
            hasSuspended = true
        case Unknown:
            hasUnknown = true
        }
    }

    if hasDegraded {
        return Degraded
    }
    if hasMissing {
        return Missing
    }
    if hasProgressing {
        return Progressing
    }
    if hasSuspended {
        return Suspended
    }
    if hasUnknown {
        return Unknown
    }
    return Healthy
}
```

### Resource-Specific Health Checks

ArgoCD includes built-in health checks for common resource types:

| Resource Type | Health Check |
|---------------|--------------|
| Deployment | `availableReplicas == replicas` |
| StatefulSet | `readyReplicas == replicas` |
| DaemonSet | `numberReady == desiredNumberScheduled` |
| Pod | Phase is Running/Succeeded, containers ready |
| Service | Always healthy (no status to check) |
| PVC | Phase is Bound |
| Job | Succeeded > 0 or Active > 0 |
| Custom Resources | Lua scripts or status.conditions[] |

## Sync Status Pattern

ArgoCD tracks whether the live state matches the desired state:

| Sync Status | Description |
|-------------|-------------|
| `Synced` | Live state matches desired state |
| `OutOfSync` | Live state differs from desired state |
| `Unknown` | Comparison couldn't be performed |

### Drift Detection

```yaml
status:
  sync:
    status: OutOfSync
    comparedTo:
      source:
        repoURL: https://github.com/example/app
        path: manifests
        targetRevision: HEAD
      destination:
        server: https://kubernetes.default.svc
        namespace: default
    revision: abc123
  resources:
    - kind: Deployment
      name: web
      status: OutOfSync  # This specific resource drifted
      diff: |
        spec.replicas: 3 -> 5  # Simplified diff representation
```

## Progressive Sync: Waves and Hooks

ArgoCD supports ordered deployment via sync waves and hooks:

### Sync Waves

Resources are deployed in wave order (lowest first):

```yaml
# Wave 0 - deployed first
apiVersion: v1
kind: Namespace
metadata:
  name: my-app
  annotations:
    argocd.argoproj.io/sync-wave: "0"
---
# Wave 1 - deployed after namespace exists
apiVersion: v1
kind: ConfigMap
metadata:
  name: config
  annotations:
    argocd.argoproj.io/sync-wave: "1"
---
# Wave 2 - deployed after config exists
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  annotations:
    argocd.argoproj.io/sync-wave: "2"
```

### Hooks

Lifecycle hooks for sync operations:

| Hook | When Executed |
|------|---------------|
| `PreSync` | Before sync begins |
| `Sync` | During sync (default) |
| `PostSync` | After all Sync resources are healthy |
| `SyncFail` | When sync fails |
| `Skip` | Resource is skipped during sync |

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: db-migration
  annotations:
    argocd.argoproj.io/hook: PreSync
    argocd.argoproj.io/hook-delete-policy: HookSucceeded
spec:
  template:
    spec:
      containers:
        - name: migrate
          image: migrate:latest
          command: ["./migrate.sh"]
      restartPolicy: Never
```

## Resource Tracking Methods

ArgoCD supports multiple methods to track which resources belong to an Application:

| Method | Description | Annotation/Label |
|--------|-------------|------------------|
| `annotation` | Track via annotation (default) | `argocd.argoproj.io/tracking-id` |
| `label` | Track via label | `app.kubernetes.io/instance` |
| `annotation+label` | Both methods | Both annotation and label |

## Application to Aggregate CRD

The ArgoCD Application pattern maps well to **Option 1 (Composition CRD)** and **Option 4 (Status Aggregator)** from the design options.

### Pattern Mapping

| ArgoCD Pattern | Application to Aggregate CRD |
|----------------|------------------------------|
| Health aggregation | `aggregationStrategy: AllHealthy/AnyHealthy/Quorum` |
| Health status hierarchy | Map: Synced→Healthy, Failed→Degraded, Pending→Progressing |
| Sync status | `driftDetected` field in resource CRDs |
| Resource tracking | `resourceSelectors` with kind/labels/namePattern |
| Sync waves | `dependsOn` or ordered resource lists |
| Hooks | Action CRDs for PreSync/PostSync operations |
| Per-resource status | `status.resources[]` with individual states |

### Proposed Design Incorporating ArgoCD Patterns

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreAggregate
metadata:
  name: store-health
spec:
  # Resource selection (similar to ArgoCD's managed resources)
  resources:
    - kind: Pet
      name: fluffy
    - kind: Order
      name: order-123
  resourceSelectors:
    - kind: Pet
      matchLabels:
        environment: production
    - kind: Order
      namePattern: "^order-.*"

  # Health aggregation strategy (inspired by ArgoCD)
  aggregationStrategy: AllHealthy  # or AnyHealthy, Quorum, Priority

  # Health status mapping (ArgoCD-style)
  healthMapping:
    healthy: ["Synced", "Observed", "Queried", "Completed"]
    progressing: ["Pending", "Syncing", "Querying", "Executing"]
    degraded: ["Failed"]

  # Sync waves for ordered operations (ArgoCD-style)
  syncWaves:
    - wave: 0
      resources:
        - kind: Pet
    - wave: 1
      resources:
        - kind: Order  # Orders depend on Pets existing

  # CEL derived values
  derivedValues:
    - name: healthPercentage
      expression: "summary.synced * 100 / summary.total"
    - name: allHealthy
      expression: "summary.failed == 0 && summary.pending == 0"

status:
  # ArgoCD-style aggregated health
  health:
    status: Degraded  # Healthy, Progressing, Degraded, Unknown
    message: "1 of 3 resources failed"

  # ArgoCD-style sync status
  sync:
    status: OutOfSync  # Synced, OutOfSync, Unknown
    driftCount: 1

  # Summary counts
  summary:
    total: 3
    healthy: 2
    progressing: 0
    degraded: 1

  # Per-resource status (ArgoCD-style)
  resources:
    - kind: Pet
      name: fluffy
      health:
        status: Healthy
      sync:
        status: Synced
      lastSyncTime: "2026-01-12T10:00:00Z"
    - kind: Pet
      name: buddy
      health:
        status: Healthy
      sync:
        status: Synced
      lastSyncTime: "2026-01-12T10:00:00Z"
    - kind: Order
      name: order-123
      health:
        status: Degraded
      sync:
        status: OutOfSync
      message: "API returned 503"
      lastSyncTime: "2026-01-12T09:55:00Z"

  # CEL computed values
  computedValues:
    - name: healthPercentage
      value: "66"
    - name: allHealthy
      value: "false"

  # Operation state (for sync waves)
  operationState:
    phase: Running  # Pending, Running, Succeeded, Failed
    wave: 1
    message: "Processing wave 1"

  lastAggregationTime: "2026-01-12T10:00:00Z"
```

## Key Differences from ArgoCD

| Aspect | ArgoCD | OpenAPI Generator Aggregate |
|--------|--------|----------------------------|
| Source of truth | Git repository | REST API responses |
| Resource types | Any Kubernetes resource | Generated CRDs only |
| Sync mechanism | Apply manifests from Git | Reconcile with REST API |
| Health checks | Lua scripts, built-in rules | State field mapping |
| Drift detection | Git vs live state | Spec vs API response |
| Scope | GitOps deployments | REST API synchronization |

## Implementation Recommendations

### 1. Adopt ArgoCD's Health Status Hierarchy

Replace simple `Healthy/Degraded` with richer status:

```go
type HealthStatus string

const (
    HealthStatusHealthy     HealthStatus = "Healthy"
    HealthStatusProgressing HealthStatus = "Progressing"
    HealthStatusDegraded    HealthStatus = "Degraded"
    HealthStatusSuspended   HealthStatus = "Suspended"
    HealthStatusMissing     HealthStatus = "Missing"
    HealthStatusUnknown     HealthStatus = "Unknown"
)

func mapResourceState(state string) HealthStatus {
    switch state {
    case "Synced", "Observed", "Queried", "Completed":
        return HealthStatusHealthy
    case "Pending", "Syncing", "Querying", "Executing":
        return HealthStatusProgressing
    case "Failed":
        return HealthStatusDegraded
    case "NotFound":
        return HealthStatusMissing
    default:
        return HealthStatusUnknown
    }
}
```

### 2. Add Sync Status Tracking

Track whether resources are in sync with their external state:

```go
type SyncStatus string

const (
    SyncStatusSynced    SyncStatus = "Synced"
    SyncStatusOutOfSync SyncStatus = "OutOfSync"
    SyncStatusUnknown   SyncStatus = "Unknown"
)

// In aggregate status
type AggregateStatus struct {
    Health HealthSummary `json:"health"`
    Sync   SyncSummary   `json:"sync"`
    // ...
}

type SyncSummary struct {
    Status     SyncStatus `json:"status"`
    DriftCount int        `json:"driftCount,omitempty"`
}
```

### 3. Support Sync Waves for Ordered Operations

For future Option 2/3 (Composition) implementation:

```yaml
spec:
  syncWaves:
    - wave: 0
      resources:
        - kind: Pet
          spec:
            name: "Fluffy"
    - wave: 1
      resources:
        - kind: Order
          spec:
            petId: "${wave0.Pet.status.externalID}"
```

### 4. Add Resource-Level Health in Status

Match ArgoCD's detailed per-resource status:

```go
type ResourceStatus struct {
    Kind      string       `json:"kind"`
    Name      string       `json:"name"`
    Namespace string       `json:"namespace,omitempty"`
    Health    HealthDetail `json:"health"`
    Sync      SyncDetail   `json:"sync,omitempty"`
    Message   string       `json:"message,omitempty"`
}

type HealthDetail struct {
    Status  HealthStatus `json:"status"`
    Message string       `json:"message,omitempty"`
}

type SyncDetail struct {
    Status        SyncStatus `json:"status"`
    DriftDetected bool       `json:"driftDetected,omitempty"`
}
```

## Comparison: ArgoCD vs Crossplane vs Kro

| Aspect | ArgoCD Application | Crossplane function-status-transformer | Kro ResourceGraphDefinition |
|--------|-------------------|---------------------------------------|----------------------------|
| Primary purpose | GitOps deployment | Status aggregation | Resource composition |
| Creates resources | Yes (from Git) | No (read-only) | Yes (from templates) |
| Health aggregation | Built-in hierarchy | Matcher-based conditions | CEL expressions |
| Sync tracking | Git vs live state | N/A | N/A |
| Progressive deployment | Sync waves + hooks | N/A | DAG from dependencies |
| Expression language | Lua (for health) | CEL + regex | CEL |
| Resource discovery | Managed resources in Git | Selector-based | Defined in template |

## Conclusion

The ArgoCD Application pattern provides valuable patterns for the aggregate CRD design:

1. **Health Status Hierarchy** - Adopt `Healthy/Progressing/Degraded/Unknown` instead of simple binary states
2. **Sync Status Tracking** - Add explicit sync status alongside health status
3. **Per-Resource Detail** - Include individual resource health in aggregate status
4. **Progressive Sync** - Consider sync waves for future composition features (Option 2/3)

### Recommended Adoption Priority

| Pattern | Priority | Applicable To |
|---------|----------|---------------|
| Health status hierarchy | High | Option 1, Option 4 (current) |
| Per-resource status detail | High | Option 1, Option 4 (current) |
| Sync status tracking | Medium | Option 1, Option 4 (current) |
| Sync waves/hooks | Low | Option 2, Option 3 (future) |

The current aggregate CRD implementation already incorporates some ArgoCD patterns (health aggregation, per-resource status). Adding the health status hierarchy and sync status tracking would bring it closer to ArgoCD's mature model.

## Sources

- [ArgoCD Application CRD Documentation](https://argo-cd.readthedocs.io/en/stable/operator-manual/declarative-setup/)
- [ArgoCD Resource Health](https://argo-cd.readthedocs.io/en/stable/operator-manual/health/)
- [ArgoCD Sync Waves and Hooks](https://argo-cd.readthedocs.io/en/stable/user-guide/sync-waves/)
- [ArgoCD Resource Tracking](https://argo-cd.readthedocs.io/en/stable/user-guide/resource_tracking/)
- [github.com/argoproj/argo-cd](https://github.com/argoproj/argo-cd)
