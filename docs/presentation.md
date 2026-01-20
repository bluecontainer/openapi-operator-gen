---
marp: true
theme: default
paginate: true
size: 16:9
---

# OpenAPI Operator Generator

**Generate Kubernetes Operators from OpenAPI Specs**

---

# The Problem

- REST APIs need Kubernetes-native management
- Manual operator development is time-consuming
- Keeping CRDs in sync with API specs is error-prone

# The Solution

- **Auto-generate** CRDs from OpenAPI specs
- **Generate** reconciliation logic automatically
- **Full CRUD** lifecycle out of the box

---

# Key Features

- **OpenAPI 3.0/3.1 & Swagger 2.0** support
- **Multi-endpoint discovery** (StatefulSet, Deployment, Helm)
- **Query CRDs** for GET-only endpoints
- **Action CRDs** for POST/PUT-only endpoints
- **Aggregate CRD** for status aggregation
- **Bundle CRD** for multi-resource orchestration
- **OpenTelemetry** metrics and tracing

---

# Architecture

```
    OpenAPI Spec (YAML/JSON)
              │
              ▼
  ┌───────────────────────┐
  │   Parser → Mapper →   │
  │      Generator        │
  └───────────┬───────────┘
              │
              ▼
  ┌───────────────────────┐
  │  Generated Operator   │
  │  • types.go           │
  │  • controllers        │
  │  • CRD manifests      │
  └───────────────────────┘
```

---

# Endpoint Classification

| Type | HTTP Methods | Behavior |
|------|--------------|----------|
| **Resource** | GET + POST/PUT/DELETE | Full CRUD |
| **Query** | GET only | Periodic queries |
| **Action** | POST/PUT only | One-shot ops |

**Examples:**
- `/pets` → Pet Resource
- `/pet/findByStatus` → Query CRD
- `/pet/{id}/uploadImage` → Action CRD

---

# Resource CRDs

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: fluffy
spec:
  name: "Fluffy"
  status: "available"
```

**Reconciliation:** GET → Compare → CREATE/UPDATE → Finalizer cleanup

---

# Query CRDs

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetFindByStatus
metadata:
  name: available-pets
spec:
  status: [available, pending]
```

**Result:** Periodic execution, typed responses in status

---

# Action CRDs

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetUploadImage
metadata:
  name: upload-photo
spec:
  petId: "12345"
  reExecuteInterval: 24h  # Optional
```

**Modes:** One-shot or periodic re-execution

---

# Aggregate CRD

Observe multiple resources, compute derived values:

```yaml
kind: PetstoreAggregate
spec:
  resourceSelectors:
    - kind: Pet
    - kind: Order
  aggregationStrategy: AllHealthy
  derivedValues:
    - name: syncPct
      expression: "summary.synced * 100 / summary.total"
```

---

# Bundle CRD

Orchestrate resources with dependencies:

```yaml
kind: PetstoreBundle
spec:
  resources:
    - id: pet
      kind: Pet
      spec: { name: "Fluffy" }
    - id: order
      kind: Order
      dependsOn: [pet]
      spec: { petId: "${resources.pet.status.externalID}" }
```

---

# CEL Expressions

**Variables:** `resources`, `summary`, `<kind>s`

**Functions:**
```yaml
- expression: "sum(orders.map(r, r.spec.quantity))"
- expression: "max(orders.map(r, r.spec.quantity))"
- expression: "avg(orders.map(r, r.spec.quantity))"
```

---

# Endpoint Discovery

| Mode | Example |
|------|---------|
| Static URL | `targetBaseURL: "http://api:8080"` |
| StatefulSet | `targetStatefulSet: petstore-api` |
| Deployment | `targetDeployment: petstore-api` |
| Helm | `targetHelmRelease: petstore-prod` |

---

# Selection Strategies

| Strategy | Description |
|----------|-------------|
| `round-robin` | Distribute across pods |
| `leader-only` | Always pod-0 |
| `any-healthy` | First healthy (default) |
| `all-healthy` | Fan-out to all |

---

# Quick Start

```bash
# Install
go install github.com/bluecontainer/openapi-operator-gen/...@latest

# Generate
openapi-operator-gen generate \
  --spec petstore.yaml \
  --output ./generated \
  --group petstore.example.com \
  --aggregate --bundle

# Build & Deploy
cd generated && make docker-build deploy
```

---

# Generated Output

```
generated/
├── api/v1alpha1/types.go
├── internal/controller/*_controller.go
├── config/crd/bases/
├── config/rbac/
├── chart/           # Helm chart
├── Dockerfile
└── Makefile
```

---

# Observability

**Metrics:**
- `reconcile_total`, `reconcile_duration_seconds`
- `api_request_total`, `api_request_duration_seconds`

**Tracing:** Full OpenTelemetry support

---

# Advanced Features

| Feature | Usage |
|---------|-------|
| Read-only mode | `readOnly: true` |
| Retain on delete | `onDelete: retain` |
| Import existing | `externalID: "123"` |

---

# Use Cases

1. **Multi-Tenant SaaS** - Per-tenant API targeting
2. **GitOps** - Manage REST resources declaratively
3. **Dashboards** - Aggregate CRD for metrics
4. **Workflows** - Bundle CRD for orchestration

---

# Summary

**OpenAPI Operator Generator** transforms REST APIs into Kubernetes operators.

- Zero-boilerplate generation
- CRUD + Query + Action support
- Aggregation and orchestration
- Production-ready observability

**GitHub:** github.com/bluecontainer/openapi-operator-gen

---

# Questions?

**Try it:**
```bash
git clone https://github.com/bluecontainer/openapi-operator-gen
cd openapi-operator-gen
make example
```

**Issues:** github.com/bluecontainer/openapi-operator-gen/issues
