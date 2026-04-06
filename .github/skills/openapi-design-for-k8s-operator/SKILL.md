---
name: openapi-design-for-k8s-operator
description: >
  Guide for designing OpenAPI specifications that produce high-quality Kubernetes operators
  via openapi-operator-gen. Covers endpoint classification, schema constraints, path structure,
  field mapping, and best practices for CRUD resources, query endpoints, and action endpoints.
  Also covers how the generated operator works at runtime: deployment topology, endpoint
  discovery, drift detection, cross-resource observability (Aggregate and Bundle CRDs),
  CEL expressions, OpenTelemetry metrics/tracing, kubectl plugin diagnostics, and
  deployment scenario guidance.
---

# Designing OpenAPI Specs for Kubernetes Operator Generation

This skill helps you design REST API OpenAPI specifications that integrate optimally with
**openapi-operator-gen** — a tool that generates Kubernetes operators (CRDs + controllers)
from OpenAPI 3.0/3.1 specs. The generated operator creates a CRD for each API resource and
reconciles Kubernetes custom resources against the backing REST API using a GET-first,
drift-detection pattern.

## How Endpoints Are Classified

openapi-operator-gen classifies each path into one of three CRD types based on which HTTP
methods are present:

| Type | Detection Rule | CRD Behavior |
|------|---------------|--------------|
| **Resource** | Path has GET + at least one of POST/PUT/DELETE/PATCH | Full CRUD reconciliation with drift detection |
| **QueryEndpoint** | Path has GET only (no write methods) | Periodic read-only queries, results in status |
| **ActionEndpoint** | Path has POST or PUT only (no GET) | One-shot or periodic execution of an action |

**Special case:** GET endpoints whose path contains action keywords (`login`, `logout`,
`register`, `verify`, `activate`, `reset`, `refresh`) are reclassified as ActionEndpoints.

---

## Resource Endpoints (CRUD)

Resources are the primary CRD type. The generated controller implements:
1. **GET** current state from the API
2. **Compare** with the CR spec (drift detection)
3. **CREATE** (POST) or **UPDATE** (PUT/PATCH) only when needed
4. **DELETE** via finalizer when the CR is removed

### Ideal Path Structure

Use a two-path pattern — one for collection operations, one for instance operations:

```yaml
paths:
  /widgets:
    post:
      operationId: createWidget
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Widget'
      responses:
        '201':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Widget'
  /widgets/{widgetId}:
    get:
      operationId: getWidget
      parameters:
        - name: widgetId
          in: path
          required: true
          schema:
            type: integer
            format: int64
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Widget'
    put:
      operationId: updateWidget
      # ... same schema as POST
    delete:
      operationId: deleteWidget
```

This produces a single **Widget** CRD with full create/read/update/delete capabilities.

### Method Availability

- **PUT** → enables drift-detected updates (sends full object)
- **PATCH** → enables partial updates (sends only changed fields, preferred over PUT)
- **DELETE** → enables finalizer-based cleanup on CR deletion
- **POST** → enables creation of new resources
- If only POST is available (no PUT/PATCH), `updateWithPost` config option can be used

### Modeling State Changes (pause/resume, enable/disable)

Model mutable state as **fields on the resource schema**, not separate endpoints:

```yaml
# GOOD: State is a field — changing spec.state triggers PUT/PATCH
components:
  schemas:
    Widget:
      type: object
      properties:
        name:
          type: string
        state:
          type: string
          enum: [running, paused, stopped]
```

The user changes `spec.state: "paused"` on the CR. The controller detects drift and sends
PUT/PATCH to the API. This is the Kubernetes-native declarative model.

```yaml
# LESS IDEAL: Separate /pause and /resume endpoints become separate ActionEndpoint CRDs
# The user must create/delete separate CR objects instead of editing one field
paths:
  /widgets/{widgetId}/pause:
    post: ...
  /widgets/{widgetId}/resume:
    post: ...
```

### Request vs Response Schema

The CRD spec is generated from the **POST/PUT request body** schema only. The GET response
schema is used at runtime for drift detection, not for CRD generation. This means POST and
GET do **not** need to use the same type.

```yaml
# This works — different request and response types
paths:
  /widgets:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateWidgetRequest'   # name, description
      responses:
        '201':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Widget'              # id, name, description, createdAt
  /widgets/{widgetId}:
    get:
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Widget'              # id, name, description, createdAt
```

Extra read-only fields in the GET response (`id`, `createdAt`, `updatedAt`) won't cause
false drift because `mergeOnUpdate` (enabled by default) only compares fields present in the
spec. On update, the controller GETs current state, overlays spec fields, and sends the
merged result to PUT — preserving server-managed fields.

**Important:** field names that appear in both request and response schemas must match. If
POST takes `title` but GET returns `name` for the same value, drift detection will always
see a mismatch and trigger unnecessary updates.

### Drift Detection Requires Schema Overlap

**Critical:** The CRD spec fields come from the POST/PUT/PATCH request body, but drift
detection compares those fields against the GET response at runtime. If the request and
response schemas share **no field names**, drift detection will silently never detect any
drift — the controller will find no matching keys to compare and always report "no drift".

The generator warns about this during mapping, but catch it early during spec review:

```yaml
# BROKEN — drift detection is useless
#   POST body:  { title, body, authorId }
#   GET returns: { headline, content, writer_id, publishedAt }
#   Zero field overlap → drift is never detected
paths:
  /articles:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateArticle'    # title, body, authorId
  /articles/{articleId}:
    get:
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Article'        # headline, content, writer_id
```

```yaml
# FIXED — shared field names enable drift detection
#   POST body:  { title, body, authorId }
#   GET returns: { id, title, body, authorId, publishedAt }
#   3 overlapping fields → drift detection works
paths:
  /articles:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateArticle'    # title, body, authorId
  /articles/{articleId}:
    get:
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Article'        # id, title, body, authorId, publishedAt
```

When reviewing an OpenAPI spec, compare the field names from the write schema (POST/PUT/PATCH
request body) with the read schema (GET 200 response body):
- **Full overlap**: Ideal — all mutable fields are observable via GET.
- **Partial overlap**: Acceptable — drift is detected only for overlapping fields. Consider
  whether the non-overlapping fields are important for state management.
- **No overlap**: Broken — drift detection is completely non-functional. The schemas must be
  aligned, or the resource should not be classified as a CRUD resource.

### ID Field Mapping

When the path parameter name differs from the body's ID field, use the `x-k8s-id-field`
extension to tell the generator how they relate:

```yaml
parameters:
  - name: widgetId
    in: path
    required: true
    x-k8s-id-field: id    # Maps path param "widgetId" to body field "id"
    schema:
      type: integer
      format: int64
```

Without this, the generator auto-detects if the path param follows the `{kindName}Id`
pattern (e.g., `widgetId` for Kind `Widget` → auto-maps to body `id`).

---

## Query Endpoints (Read-Only)

GET-only paths become QueryEndpoint CRDs. The controller periodically executes the query and
stores results in the CR's status.

```yaml
# This becomes a "WidgetSearchQuery" CRD
paths:
  /widgets/search:
    get:
      operationId: searchWidgets
      parameters:
        - name: category
          in: query
          schema:
            type: string
        - name: limit
          in: query
          schema:
            type: integer
            format: int32
      responses:
        '200':
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Widget'
```

### Response Type Mapping

| Response Schema | Generated Go Type | Notes |
|----------------|-------------------|-------|
| Array of objects | `[]WidgetResult` | Struct generated from item schema |
| Single object | `*WidgetResult` | Struct generated from schema |
| Array of strings | `[]string` | Primitive array |
| Unknown/complex | `*runtime.RawExtension` | Fallback for untyped JSON |

Always provide a **typed response schema** for best results. Avoid untyped responses.

---

## Action Endpoints (One-Shot / Periodic Operations)

POST/PUT-only paths (no GET) become ActionEndpoint CRDs. These are for operations that
"do something" rather than manage a persistent entity.

```yaml
# This becomes a "WidgetUploadImage" ActionEndpoint CRD
paths:
  /widgets/{widgetId}/uploadImage:
    post:
      operationId: uploadWidgetImage
      parameters:
        - name: widgetId
          in: path
          required: true
          schema:
            type: integer
            format: int64
      requestBody:
        content:
          application/octet-stream:
            schema:
              type: string
              format: binary
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/UploadResult'
```

Action controllers support:
- **One-shot execution** (default): runs once when CR is created
- **Re-execution on spec change**: updates to CR spec trigger re-execution
- **Periodic execution**: configurable interval for repeated actions
- **Binary uploads**: from inline base64, ConfigMap, Secret, URL, or file

---

## Schema Design Rules

### Supported Types

| OpenAPI Type + Format | Generated Go Type |
|-----------------------|-------------------|
| `string` | `string` |
| `string` + `date-time` | `metav1.Time` (Kubernetes native) |
| `string` + `byte` | `[]byte` |
| `integer` + `int64` | `int64` |
| `integer` + `int32` | `int32` |
| `integer` (no format) | `int` |
| `number` + `double` | `float64` |
| `number` + `float` | `float32` |
| `boolean` | `bool` |
| `array` of typed items | `[]ItemType` |
| `object` with properties | Generated struct |
| `object` without properties | `*runtime.RawExtension` |

### Use Flat Schemas — Avoid Composition Keywords

The generator does **not** support `oneOf`, `anyOf`, or `allOf`. Flatten composed schemas
into concrete object definitions:

```yaml
# BAD — will not be processed correctly
components:
  schemas:
    Widget:
      allOf:
        - $ref: '#/components/schemas/BaseEntity'
        - type: object
          properties:
            color:
              type: string

# GOOD — flat schema with all properties
components:
  schemas:
    Widget:
      type: object
      required: [name]
      properties:
        id:
          type: integer
          format: int64
        name:
          type: string
        color:
          type: string
```

### Use String Enums for Validation

String enums become kubebuilder validation markers on the CRD:

```yaml
# GOOD — generates kubebuilder Enum validation
status:
  type: string
  enum: [active, inactive, archived]

# BAD — non-string enums are silently filtered
priority:
  type: integer
  enum: [1, 2, 3]   # Will NOT generate validation
```

### Validation Annotations

These OpenAPI validations are preserved in kubebuilder markers:

- `minLength` / `maxLength` → string length constraints
- `minimum` / `maximum` → numeric range constraints
- `pattern` → regex validation
- `minItems` / `maxItems` → array size constraints
- `required` → field presence enforcement

### Avoid Circular References

Recursive or circular `$ref` chains are not supported and will cause generation failures.

### Map Types Use RawExtension

`additionalProperties` (map types) fall back to `*runtime.RawExtension`. If you need typed
maps, model them as arrays of key-value objects instead:

```yaml
# AVOID — becomes untyped RawExtension
metadata:
  type: object
  additionalProperties:
    type: string

# PREFER — becomes typed []MetadataEntry
metadata:
  type: array
  items:
    type: object
    properties:
      key:
        type: string
      value:
        type: string
```

---

## Parameter Rules

### Path Parameters

- **Always required** (OpenAPI spec requirement)
- Become required fields in the CRD spec
- Use standard types: `string`, `integer` (with `int64` or `int32` format)

### Query Parameters

- Become optional CRD spec fields by default
- Optional numeric query params are wrapped in pointer types (`*int64`)
- Array query params (`type: array`) become Go slices

### Unsupported Parameter Locations

- `in: header` — **ignored** (manage headers externally)
- `in: cookie` — **ignored**

---

## Content Type Requirements

### Request Bodies

- **`application/json`** — fully parsed and typed (strongly preferred)
- **`application/octet-stream`** — binary upload support for action endpoints
- **`multipart/form-data`** — parsed with binary field detection
- Other content types (XML, form-urlencoded) — **ignored**

### Responses

- **`application/json`** — fully typed from schema
- Other content types — **ignored**, falls back to `RawExtension`

Always use `application/json` for both requests and responses.

---

## Naming Conventions

CRD Kind names are derived from **path structure**, not `operationId`:

| Path | Generated Kind |
|------|---------------|
| `/pets` + `/pets/{petId}` | `Pet` |
| `/store/orders` + `/store/orders/{orderId}` | `StoreOrder` |
| `/pets/findByStatus` (GET only) | `PetFindByStatusQuery` |
| `/pets/{petId}/uploadImage` (POST only) | `PetUploadImage` |

- Paths are PascalCased and singularized automatically
- `operationId` is used for filtering and diagnostics, not naming
- Include meaningful `operationId` values anyway — they enable `excludeOperations` /
  `includeOperations` filtering in the generator config

---

## Configuration Options That Affect Mapping

These generator settings influence how your spec is processed:

| Config Option | Effect |
|---------------|--------|
| `updateWithPost: [/path]` | Use POST for updates when PUT is unavailable on listed paths |
| `idFieldMap: {paramName: bodyField}` | Explicit path-param-to-body-field mapping |
| `filters.excludeOperations` | Skip specific operations by operationId |
| `filters.excludePaths` | Skip entire paths by glob pattern |
| `filters.includeTags` | Only process operations with specific tags |
| `aggregate: true` | Generate a StatusAggregator CRD for cross-resource observability |
| `bundle: true` | Generate an InlineCompositionBundle CRD for creating multiple resources together |

---

## Complete Example: Well-Designed Spec

```yaml
openapi: "3.0.3"
info:
  title: Widget Service API
  version: "1.0.0"
paths:
  /widgets:
    post:
      operationId: createWidget
      tags: [widgets]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Widget'
      responses:
        '201':
          description: Created
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Widget'

  /widgets/{widgetId}:
    get:
      operationId: getWidget
      tags: [widgets]
      parameters:
        - name: widgetId
          in: path
          required: true
          x-k8s-id-field: id
          schema:
            type: integer
            format: int64
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Widget'
    put:
      operationId: updateWidget
      tags: [widgets]
      parameters:
        - name: widgetId
          in: path
          required: true
          schema:
            type: integer
            format: int64
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Widget'
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Widget'
    delete:
      operationId: deleteWidget
      tags: [widgets]
      parameters:
        - name: widgetId
          in: path
          required: true
          schema:
            type: integer
            format: int64
      responses:
        '204':
          description: Deleted

  /widgets/search:
    get:
      operationId: searchWidgets
      tags: [widgets]
      parameters:
        - name: category
          in: query
          schema:
            type: string
        - name: status
          in: query
          schema:
            type: string
            enum: [active, inactive]
      responses:
        '200':
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Widget'

  /widgets/{widgetId}/reset:
    post:
      operationId: resetWidget
      tags: [widgets]
      parameters:
        - name: widgetId
          in: path
          required: true
          schema:
            type: integer
            format: int64
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                force:
                  type: boolean
                reason:
                  type: string
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  resetAt:
                    type: string
                    format: date-time
                  previousState:
                    type: string

components:
  schemas:
    Widget:
      type: object
      required: [name]
      properties:
        id:
          type: integer
          format: int64
        name:
          type: string
          minLength: 1
          maxLength: 255
        description:
          type: string
        state:
          type: string
          enum: [running, paused, stopped]
        priority:
          type: string
          enum: [low, medium, high, critical]
        tags:
          type: array
          items:
            type: string
          maxItems: 10
        config:
          type: object
          properties:
            retryCount:
              type: integer
              format: int32
              minimum: 0
              maximum: 10
            timeoutSeconds:
              type: integer
              format: int64
        createdAt:
          type: string
          format: date-time
```

This produces:
- **Widget** Resource CRD (full CRUD with drift detection, state changes via spec.state)
- **WidgetSearchQuery** QueryEndpoint CRD (periodic search with category/status filters)
- **WidgetReset** ActionEndpoint CRD (one-shot reset operation with force/reason params)

---

# Generated Operator Runtime Guide

The sections below describe how the generated operator works at runtime — how it discovers
API endpoints, reconciles state, reports health, and supports operational workflows. Use
this when answering questions about deploying, configuring, monitoring, or troubleshooting
a generated operator.

---

## Deployment Topology & Endpoint Discovery

The generated operator discovers the target REST API at runtime using one of several
deployment modes. The choice depends on how the target application is deployed in Kubernetes.

### Deployment Modes

| Mode | Configuration | When to Use |
|------|--------------|-------------|
| **Static URL** | `--base-url` / `REST_API_BASE_URL` | External APIs, development, non-Kubernetes targets |
| **Static URLs (fan-out)** | `--base-urls` / `REST_API_BASE_URLS` | Multi-instance writes (comma-separated URLs) |
| **StatefulSet discovery** | `--statefulset-name` / `STATEFULSET_NAME` | Databases, stateful services with stable pod identities |
| **Deployment discovery** | `--deployment-name` / `DEPLOYMENT_NAME` | Stateless services behind a Deployment |
| **Helm release auto-detection** | `--helm-release` / `HELM_RELEASE` | Any Helm-deployed workload (finds it via `app.kubernetes.io/instance` label) |
| **Direct pod** | `--pod-name` / `TARGET_POD_NAME` | Target a single specific pod |
| **Per-CR targeting** | *(no global config)* | Each CR specifies its own target in `spec.target` |

Helm release auto-detection tries StatefulSet → Deployment → DaemonSet, picking the workload
with the most replicas. It also auto-discovers the associated headless Service.

### Endpoint Selection Strategies

Once pods are discovered, the operator selects which pod(s) to route requests to:

| Strategy | Flag Value | Behavior |
|----------|-----------|----------|
| **Round-robin** | `round-robin` | Distributes requests across all healthy pods (default) |
| **Leader-only** | `leader-only` | Always routes to pod-0 (StatefulSet only) |
| **Any-healthy** | `any-healthy` | Uses one healthy pod, fails over on health check failure |
| **All-healthy** | `all-healthy` | Fan-out: sends requests to ALL healthy pods (for broadcast/comparison) |
| **By-ordinal** | `by-ordinal` | Routes to a specific pod ordinal set in the CR's `spec.target.podOrdinal` |

`leader-only` and `by-ordinal` are auto-downgraded to `round-robin` for Deployments and
DaemonSets (since they have no stable ordinals).

### Discovery Modes

| Mode | Flag Value | How Endpoints Are Found |
|------|-----------|------------------------|
| **DNS** | `dns` | Headless service DNS names (`pod-N.svc.ns.svc.cluster.local`). StatefulSet only. |
| **Pod IP** | `pod-ip` | Queries Kubernetes API for pod IPs. Works with all workload types. |
| **Service DNS** | `service-dns` | Discovers pods behind a Service via selector labels. Useful for DaemonSets. |

Default: `dns` for StatefulSets, `pod-ip` for Deployments and DaemonSets.

### Per-CR Targeting

Every generated CRD has an optional `spec.target` field that overrides the global endpoint
configuration for that specific resource:

```yaml
apiVersion: example.io/v1alpha1
kind: Widget
metadata:
  name: my-widget
spec:
  name: "test-widget"
  target:
    helmRelease: my-release        # Discover workload from Helm release
    statefulSet: my-sts            # Or target a specific StatefulSet
    deployment: my-deploy          # Or target a specific Deployment
    pod: specific-pod-name         # Or target a specific pod
    namespace: other-ns            # Namespace of the target workload
    baseURL: "http://api.example.com:8080"      # Static URL override
    baseURLs:                      # Fan-out to multiple URLs
      - "http://api-1:8080"
      - "http://api-2:8080"
    podOrdinal: 2                  # Route to specific StatefulSet pod ordinal
    labels:                        # Narrow workload discovery by labels
      env: production
```

This enables a single operator instance to manage resources across multiple API instances —
for example, creating a Widget CR per environment or per StatefulSet member.

### Health Checking

The resolver performs background health checks on discovered endpoints:
- **Default health path**: `/health` (configurable via `--health-path`, empty to disable)
- **Check interval**: Every 10 seconds
- **Refresh interval**: Pod list refreshed every 30 seconds
- Unhealthy pods are excluded from endpoint selection (except `all-healthy` which reports
  them separately)

---

## Operational Patterns

### Drift Detection (Resource CRDs)

The controller implements GET-first reconciliation with field-level drift detection:

1. **GET** the current state from the REST API
2. **Compare** spec fields against the GET response
3. **Correct** drift via PUT/PATCH only when differences are found

**mergeOnUpdate** (default: `true`): The controller starts with the current API state,
overlays only the fields present in the CR spec, and sends the merged result to PUT. This
means:
- Server-managed fields (`id`, `createdAt`, `updatedAt`) are preserved automatically
- Empty maps `{}` and empty slices `[]` in the spec are skipped (won't overwrite API values)
- Only fields the user explicitly sets can trigger drift

**Comparison handles**: timestamp normalization (RFC3339 variants), nested maps/slices
recursively, and numeric type coercion (JSON `float64` vs Go `int`).

**Drift correction priority**: PATCH (if available) → PUT (with merge) → POST (if
`updateWithPost` configured) → record drift in status but take no action (if no write
method available).

### Read-Only / Observation Mode

Set `spec.readOnly: true` on any Resource CRD to observe without mutating:

```yaml
spec:
  readOnly: true
  externalIDRef: "42"    # Required — tells the controller which resource to GET
```

- Performs GET only — no POST/PUT/PATCH/DELETE
- Sets state to `Observed` or `NotFound`
- Drift detection is disabled (no spec to compare against)
- No finalizer added (nothing to clean up on deletion)
- Useful for monitoring existing API resources without managing their lifecycle

### Pause / Resume

Set `spec.paused: true` to pause synchronization while continuing to monitor:

- **Drift detection continues** — the controller still GETs and compares, so you can see
  if the API has drifted while paused
- **No writes** — POST/PUT/PATCH are skipped
- State shows `Paused` or `Paused (drift detected)`
- Resume by setting `spec.paused: false` — normal sync resumes immediately

For Query and Action CRDs, pausing stops all execution entirely.

### Execution Intervals

Every CRD type supports `spec.executionInterval` to control reconciliation frequency:

| CRD Type | Default Behavior | With `executionInterval` |
|----------|-----------------|------------------------|
| **Resource** | Requeue every 30s | Override the 30s default |
| **Query** | One-shot (execute once) | Periodic re-execution at interval |
| **Action** | One-shot (execute once) | Periodic re-execution at interval |

Setting `executionInterval: 0s` disables periodic reconciliation — the controller only
re-reconciles on spec changes (watch events).

### Deletion Policies (Resource CRDs)

The `spec.onDelete` field controls what happens to the REST API resource when the CR is
deleted from Kubernetes:

| Policy | Behavior |
|--------|----------|
| `Delete` | Sends DELETE to the REST API (default for resources created by the controller) |
| `Orphan` | Leaves the API resource untouched (default for adopted/pre-existing resources) |
| `Restore` | Restores the original state captured when the resource was first adopted |

The controller tracks whether it created the resource (`status.createdByController`) and
captures the original API state on first adoption (`status.originalState`). This enables
safe rollback when managing pre-existing resources.

**Finalizer behavior**: Added only after successful sync (not before), and always removed
even if finalization fails — errors during deletion are logged but never block CR removal.

### Re-Execution on Spec Change (Query/Action CRDs)

Query and Action CRDs track `observedGeneration`. Any change to the CR spec triggers
immediate re-execution, regardless of whether the CRD is in one-shot or periodic mode.
This lets you update query parameters or action inputs and see results immediately.

### TTL Patches

The controller supports temporary spec overrides via annotations:

1. Apply a patch with `<apiGroup>/patch-expires` annotation (RFC3339 timestamp)
2. Save original values in `<apiGroup>/patch-original-state` annotation
3. When the TTL expires, the controller automatically restores the original spec values

This is useful for temporary configuration changes during debugging or maintenance windows.

### Error Handling

The controller distinguishes retryable from non-retryable API errors:

| Error Type | Behavior |
|-----------|----------|
| **5xx / network errors** | Retryable — requeue at standard interval |
| **4xx client errors** | Non-retryable — no requeue, waits for spec change |

The controller avoids returning errors to controller-runtime directly (which would trigger
aggressive exponential backoff). Instead, it sets the status to `Failed` and manages its
own requeue timing.

### Status Fields

Every generated CRD reports structured status with consistent fields:

**Resource CRDs:**

| Field | Description |
|-------|-------------|
| `state` | `Pending`, `Syncing`, `Synced`, `Observed`, `NotFound`, `Failed`, `Paused` |
| `externalID` | ID of the resource in the REST API |
| `driftDetected` | Whether spec differs from API state |
| `driftDetectedCount` | Number of drift state transitions |
| `lastSyncTime` | Last successful POST/PUT/PATCH |
| `lastGetTime` | Last GET request |
| `response` | Single-endpoint response (`success`, `statusCode`, `data`, `lastUpdated`) |
| `responses` | Per-endpoint responses in fan-out mode (`map[url]response`) |
| `createdByController` | Whether the controller created this resource (vs adopted) |
| `originalState` | Captured state at first adoption (for `Restore` policy) |
| `conditions` | `Ready`, `Reconciling`, `Stalled` (kstatus-compatible) |

**Query CRDs** add: `resultCount`, `results`, `lastQueryTime`, `executionCount`,
`nextExecutionTime`

**Action CRDs** add: `httpStatusCode`, `executedAt`, `completedAt`, `executionCount`,
`successCount`, `totalEndpoints`, `nextExecutionTime`

---

## Cross-Resource Observability

### Aggregate CRD (StatusAggregator)

The Aggregate CRD provides a single-pane health view across multiple resources. Enable with
`aggregate: true` in the generator config.

**Selection methods** (can combine both):

```yaml
apiVersion: example.io/v1alpha1
kind: StatusAggregate
metadata:
  name: system-health
spec:
  # Method 1: Explicit references
  resources:
    - kind: Widget
      name: primary-widget
    - kind: WidgetSearchQuery
      name: status-check
      namespace: monitoring

  # Method 2: Dynamic selectors (label matching + name regex)
  resourceSelectors:
    - kind: Widget
      matchLabels:
        env: production
      namePattern: "^prod-.*"

  aggregationStrategy: AllHealthy    # AllHealthy | AnyHealthy | Quorum
  derivedValues:
    - name: total_widgets
      expression: "widgets.size()"
    - name: sync_percentage
      expression: "summary.synced * 100 / summary.total"
    - name: max_retry_count
      expression: "max(widgets.map(r, r.spec.config.retryCount))"
```

**Aggregation strategies:**

| Strategy | Healthy When |
|----------|-------------|
| `AllHealthy` | All resources are in a success state (Synced/Observed/Queried/Completed) |
| `AnyHealthy` | At least one resource is healthy |
| `Quorum` | More than 50% of resources are healthy |

**Status output:**
- `state`: `Healthy`, `Degraded`, `Pending`, `Unknown`, `Paused`
- `summary`: `{total, synced, failed, pending}` counts
- `resources`: per-resource status list (kind, name, state, externalID, message, lastSyncTime)
- `computedValues`: CEL expression results
- `conditions`: `Ready`, `Reconciling`, `Stalled`, `AllHealthy`

The Aggregate controller is **event-driven** — it watches all child resource types and
re-aggregates when any referenced resource changes. No periodic requeue needed.

### Bundle CRD (Inline Composition)

The Bundle CRD creates and manages multiple child CRs as a single unit with dependency
ordering. Enable with `bundle: true` in the generator config.

```yaml
apiVersion: example.io/v1alpha1
kind: InlineCompositionBundle
metadata:
  name: full-setup
spec:
  target:                              # Inherited by all child resources
    helmRelease: my-app
  syncWaves: true                      # Deploy in dependency-wave order
  aggregationStrategy: AllHealthy
  resources:
    - id: backend-config
      kind: Widget
      spec:
        name: "backend"
        state: "running"
        config:
          retryCount: 3

    - id: frontend-config
      kind: Widget
      dependsOn: [backend-config]      # Explicit dependency
      spec:
        name: "frontend"
        config:
          # Variable substitution — references backend's external ID
          timeoutSeconds: "${resources.backend-config.status.externalID}"

    - id: health-check
      kind: WidgetSearchQuery
      dependsOn: [backend-config, frontend-config]
      skipWhen:                         # Conditional — skip if backend failed
        - "resources.backend-config.status.state == 'Failed'"
      readyWhen:                        # Ready condition
        - "resources.health-check.status.resultCount > 0"
      spec:
        category: "active"

  derivedValues:
    - name: all_synced
      expression: "summary.synced == summary.total"
```

**Key features:**
- **Dependency ordering**: DAG resolution via explicit `dependsOn` and auto-detected
  `${resources.<id>...}` references in spec fields
- **Variable substitution**: `${resources.<id>.status.<field>}` resolves cross-resource
  references; `${now()}` and other CEL functions are supported
- **Conditional creation**: `skipWhen` CEL conditions skip resources (skipped resources
  count as ready for downstream dependents)
- **Ready conditions**: `readyWhen` CEL conditions define when a resource is considered
  ready for dependents (beyond just being in Synced state)
- **Sync waves**: Optional wave-based deployment — resources at the same dependency depth
  deploy in parallel
- **Owner references**: Child CRs are garbage-collected when the Bundle is deleted
- **Target inheritance**: Bundle-level `spec.target` is propagated to all children (child
  settings take precedence)
- **Aggregate-compatible health**: `status.aggregatedHealth` mirrors the Aggregate CRD
  format for cross-CRD monitoring

**Bundle status:**
- `state`: `Pending`, `Syncing`, `Synced`, `Failed`, `Paused`
- `summary`: `{total, synced, failed, pending, skipped}`
- `resources`: per-resource status with `id`, `kind`, `state`, `ready`, `skipped`
- `aggregatedHealth`: Aggregate-format health for unified monitoring

### CEL Expressions

Both Aggregate and Bundle CRDs support CEL (Common Expression Language) for computed values
and conditions.

**Available variables:**

| Variable | Type | Available In |
|----------|------|-------------|
| `resources` | `list(map)` (Aggregate) or `map(string, map)` (Bundle conditions) | Both |
| `summary` | `map{total, synced, failed, pending}` | Both |
| `<kind>s` | `list(map)` (e.g., `widgets`, `widgetsearchqueries`) | Both |
| `{kind}_{name}` | `map` (e.g., `widget_primary_widget`) | Aggregate (with individual refs) |

Each resource map contains `kind`, `metadata` (name, namespace, labels, annotations),
`spec` (all fields), and `status` (all fields).

**Built-in functions:**

| Function | Description |
|----------|-------------|
| `sum(list)` | Sum of numeric values |
| `max(list)` | Maximum value |
| `min(list)` | Minimum value |
| `avg(list)` | Average of values |
| `now()` | Current UTC time as RFC3339 string |
| `nowUnix()` | Current Unix timestamp (seconds) |
| `timeSince(timeStr)` | Seconds since given RFC3339 time |
| `timeUntil(timeStr)` | Seconds until given RFC3339 time |
| `addDuration(timeStr, dur)` | Add Go duration to RFC3339 time |
| `durationSeconds(durStr)` | Parse duration string to seconds |
| `parseTimeRFC3339(str)` | Parse RFC3339 to Unix timestamp |

Plus all standard CEL built-ins: `size()`, `filter()`, `map()`, `exists()`, `all()`,
string functions, etc.

**Example expressions:**

```cel
# Percentage of synced resources
summary.synced * 100 / summary.total

# Count resources of a specific kind
widgets.size()

# Aggregate over spec fields
sum(widgets.map(r, r.spec.config.retryCount))
avg(widgets.filter(r, r.status.state == 'Synced').map(r, r.spec.priority))

# Check staleness
timeSince(widget_primary_widget.status.lastSyncTime) < 3600

# Bundle conditions (readyWhen / skipWhen)
resources.backend-config.status.state == 'Synced'
resources.health-check.status.resultCount > 0
```

---

## Observability & Diagnostics

### OpenTelemetry Metrics

The generated operator exports OpenTelemetry metrics configured via environment variables:

| Env Variable | Purpose |
|-------------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint (e.g., `otel-collector:4317`). If unset, telemetry is disabled. |
| `OTEL_SERVICE_NAME` | Override the service name |
| `OTEL_INSECURE` | Set `"true"` to disable TLS for OTLP |

**Resource controller metrics:**

| Metric | Type | Description |
|--------|------|-------------|
| `reconcile_total` | Counter | Total reconciliations by status |
| `reconcile_duration_seconds` | Histogram | Duration of reconciliation |
| `api_call_total` | Counter | REST API calls by method and status code |
| `api_call_duration_seconds` | Histogram | API call duration |
| `drift_detected_total` | Counter | Drift detection state transitions |

**Query controller metrics:** `reconcile_total`, `reconcile_duration_seconds`,
`query_total`, `query_duration_seconds`

**Action controller metrics:** `reconcile_total`, `reconcile_duration_seconds`,
`action_total`, `action_duration_seconds`

**Aggregate and Bundle controllers:** `reconcile_total`, `reconcile_duration_seconds`

### Distributed Tracing

Each controller creates spans for key operations:

- **`Reconcile`** — top-level span for every reconciliation (resource name, namespace)
- **`GET`** — HTTP GET to REST API (`http.url`, `http.status_code`)
- **`SyncToEndpoint`** — per-endpoint sync (`resource.externalID`, `drift.detected`)
- **`Query`** / **`POST`** / **`PUT`** — specific operation spans

The HTTP client is instrumented with `otelhttp.NewTransport` for automatic propagation.

Resource attributes include `k8s.namespace.name` and `k8s.pod.name` (from downward API
env vars `POD_NAMESPACE` and `POD_NAME`).

### Health Probes

The operator exposes standard Kubernetes health probes:
- **Liveness**: `GET /healthz` on port 8081
- **Readiness**: `GET /readyz` on port 8081

### kubectl Plugin

The generator produces a kubectl plugin (`kubectl <app-name>`) with diagnostic subcommands:

**Core commands:**

| Command | Purpose |
|---------|---------|
| `status` | Aggregate health across all CRD types. Supports `--watch` mode (2s polling). Color-coded: green (healthy), red (failed), yellow (pending). |
| `get` | List resources by kind with tabular output |
| `describe` | Detailed resource view with all status fields |

**Diagnostic commands:**

| Command | Purpose |
|---------|---------|
| `drift` | Drift detection report. `--show-diff` shows field-level spec vs actual values. `--kind` filters by type. Identifies which specific endpoints have drift. |
| `diagnose` | Runs 9 checks: resource exists, synced with API, sync state, drift status, sync staleness (warns >2h), conditions, pause status, target pod validation, API latency. |
| `compare` | Multi-endpoint state comparison. Reads `status.responses` from fan-out mode. `--field` for specific field comparison. Reports consistency verdict. |

**Management commands:**

| Command | Purpose |
|---------|---------|
| `create` | Create resources from CLI flags |
| `query` | Execute query CRDs |
| `action` | Execute action CRDs |
| `patch` | Temporary patches with TTL (auto-restored after expiry) |
| `pause` / `unpause` | Pause/resume reconciliation with `--reason` annotation |
| `cleanup` | Remove diagnostic/temporary resources. Filters: `--one-shot`, `--expired`, `--diagnostic` |

**Integration:**

| Command | Purpose |
|---------|---------|
| `nodes` | Discover Kubernetes workloads as Rundeck node source JSON. Filters by type, labels, health, namespace patterns. |

All commands support `--output table|json|yaml|wide` and `--compact` (single-line JSON).

---

## Runtime Configuration Reference

### Operator Binary Flags

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--base-url` | `REST_API_BASE_URL` | — | Static REST API URL |
| `--base-urls` | `REST_API_BASE_URLS` | — | Comma-separated URLs for fan-out |
| `--statefulset-name` | `STATEFULSET_NAME` | — | StatefulSet for pod discovery |
| `--deployment-name` | `DEPLOYMENT_NAME` | — | Deployment for pod discovery |
| `--pod-name` | `TARGET_POD_NAME` | — | Specific pod to target |
| `--helm-release` | `HELM_RELEASE` | — | Helm release auto-detection |
| `--namespace` | `WORKLOAD_NAMESPACE` | auto-detect | Target workload namespace |
| `--service` | `SERVICE_NAME` | — | Headless service name (for DNS mode) |
| `--port` | — | `8080` | REST API port |
| `--base-path` | — | — | Base path prefix (e.g., `/api/v3`) |
| `--scheme` | — | `http` | URL scheme (`http`/`https`) |
| `--strategy` | — | `round-robin` | Endpoint selection strategy |
| `--discovery-mode` | — | auto | `dns`, `pod-ip`, or `service-dns` |
| `--health-path` | — | `/health` | Health check path (empty to disable) |
| `--workload-kind` | — | `auto` | `statefulset`, `deployment`, `daemonset`, `auto` |
| `--watch-labels` | `WATCH_LABELS` | — | Filter CRs by labels (`key=val,key2=val2`) |
| `--watch-namespaces` | `WATCH_NAMESPACES` | — | Filter CRs by namespaces (`ns1,ns2`) |
| `--namespace-scoped` | `NAMESPACE_SCOPED` | `false` | Watch only the operator's own namespace |
| `--leader-elect` | — | `false` | Enable leader election for HA |
| `--metrics-bind-address` | — | `:8080` | Metrics endpoint |
| `--health-probe-bind-address` | — | `:8081` | Health probe endpoint |

### Namespace Resolution Priority

1. `--namespace` flag
2. `WORKLOAD_NAMESPACE` env var
3. `/var/run/secrets/kubernetes.io/serviceaccount/namespace` (downward API)
4. `"default"` fallback

### CR Filtering / Sharding

- **Label-based sharding**: `--watch-labels env=production` — the operator only reconciles
  CRs with matching labels. Deploy multiple operator instances with different label selectors
  for horizontal scaling.
- **Namespace filtering**: `--watch-namespaces ns1,ns2` — restricts to specific namespaces.
- **Namespace-scoped**: `--namespace-scoped` — watches only the operator's own namespace
  (useful for per-namespace isolation).

### Generated Deployment Artifacts

| Artifact | Description |
|----------|-------------|
| **Kustomize manifests** | `config/` — namespace, CRDs, RBAC, manager Deployment with security hardening (`runAsNonRoot`, `SeccompProfile`, `drop ALL`) |
| **Helm chart** | `make helm` generates a chart via helmify; `make helm-install` / `helm-upgrade` |
| **Dockerfile** | Multi-stage: `golang:1.25` → `gcr.io/distroless/static:nonroot` (UID 65532) |
| **Docker Compose** | Dev environment with k3s cluster, auto-applied CRDs, operator container |
| **RBAC** | ClusterRole for CRDs + apps/deployments + apps/statefulsets + pods + services (endpoint discovery) |

---

## Deployment Scenario Examples

### "I have a REST API behind a Deployment and want GitOps-style config management"

**What you get**: Resource CRDs for each API entity. Declare desired state in YAML, commit
to Git, and the operator continuously reconciles against the API.

```yaml
# Configure operator to discover the API Deployment
env:
  - name: DEPLOYMENT_NAME
    value: "my-api"
  - name: WORKLOAD_NAMESPACE
    value: "api-system"
```

```yaml
# Declare resources — the operator ensures they exist in the API
apiVersion: example.io/v1alpha1
kind: Widget
metadata:
  name: production-widget
spec:
  name: "Production Widget"
  state: "running"
  priority: "high"
  config:
    retryCount: 5
    timeoutSeconds: 30
```

The controller GETs the current state, detects drift, and sends PUT/PATCH only when needed.
If someone manually changes the API, the operator corrects it on the next reconciliation
cycle (default: 30s).

### "I want to observe a third-party API without modifying it"

**What you get**: Read-only Resource CRDs that GET and report state, plus Query CRDs for
search/list operations. No writes to the API.

```yaml
# Observe an existing resource without managing it
apiVersion: example.io/v1alpha1
kind: Widget
metadata:
  name: observe-widget-42
spec:
  readOnly: true
  externalIDRef: "42"
  target:
    baseURL: "https://third-party-api.example.com"
```

```yaml
# Periodic query — results appear in status
apiVersion: example.io/v1alpha1
kind: WidgetSearchQuery
metadata:
  name: active-widgets
spec:
  category: "active"
  executionInterval: 5m    # Re-query every 5 minutes
  target:
    baseURL: "https://third-party-api.example.com"
```

Use an Aggregate CRD to monitor overall health of observed resources.

### "I need to compare state across multiple instances"

**What you get**: Per-pod targeting with fan-out and the Aggregate CRD for comparison.

```yaml
# Fan-out: write to all instances, read from all
apiVersion: example.io/v1alpha1
kind: Widget
metadata:
  name: replicated-widget
spec:
  name: "Must Be Consistent"
  target:
    statefulSet: my-api
    # With all-healthy strategy enabled on the operator, this writes to all pods
    # and status.responses contains per-pod results
```

Or create per-pod observers:

```yaml
# Observer for pod-0
apiVersion: example.io/v1alpha1
kind: Widget
metadata:
  name: widget-42-pod-0
  labels:
    comparison-set: widget-42
spec:
  readOnly: true
  externalIDRef: "42"
  target:
    statefulSet: my-api
    podOrdinal: 0
---
# Observer for pod-1
apiVersion: example.io/v1alpha1
kind: Widget
metadata:
  name: widget-42-pod-1
  labels:
    comparison-set: widget-42
spec:
  readOnly: true
  externalIDRef: "42"
  target:
    statefulSet: my-api
    podOrdinal: 1
```

Then use `kubectl <app> compare widget-42-pod-0` or an Aggregate CRD with a selector on
`comparison-set: widget-42` to see differences.

### "I want to execute one-shot operations from CI/CD"

**What you get**: Action CRDs that execute POST/PUT operations and report results in status.

```yaml
# Trigger a reset operation — one-shot, runs once on creation
apiVersion: example.io/v1alpha1
kind: WidgetReset
metadata:
  name: reset-widget-42
  labels:
    ci-run: "build-1234"
spec:
  widgetId: 42
  force: true
  reason: "CI/CD pipeline reset"
  target:
    baseURL: "https://api.example.com"
```

After creation, check `status.state` for `Completed` or `Failed`, and `status.result.data`
for the API response. Clean up with `kubectl <app> cleanup --one-shot` or label selectors.

### "I want unified health monitoring across all managed resources"

**What you get**: The Aggregate CRD for a single health endpoint, plus CEL for business
logic.

```yaml
apiVersion: example.io/v1alpha1
kind: StatusAggregate
metadata:
  name: production-health
spec:
  resourceSelectors:
    - kind: Widget
      matchLabels:
        env: production
    - kind: WidgetSearchQuery
      matchLabels:
        env: production
  aggregationStrategy: AllHealthy
  derivedValues:
    - name: sync_rate
      expression: "summary.synced * 100 / summary.total"
    - name: oldest_sync
      expression: >
        max(widgets.filter(r, r.status.state == 'Synced')
                   .map(r, timeSince(r.status.lastSyncTime)))
```

`status.state` is `Healthy`, `Degraded`, or `Pending`. Use this as a health signal for
external monitoring, ArgoCD health checks, or CI/CD gates.

### "I need to deploy a group of related resources as a unit"

**What you get**: The Bundle CRD for atomic multi-resource deployment with dependency
ordering.

```yaml
apiVersion: example.io/v1alpha1
kind: InlineCompositionBundle
metadata:
  name: full-stack-setup
spec:
  target:
    helmRelease: my-app
  syncWaves: true
  resources:
    - id: backend
      kind: Widget
      spec:
        name: "backend-service"
        state: "running"

    - id: frontend
      kind: Widget
      dependsOn: [backend]
      spec:
        name: "frontend-service"
        config:
          # Auto-resolved: backend's external ID injected here
          timeoutSeconds: "${resources.backend.status.externalID}"

    - id: health-monitor
      kind: WidgetSearchQuery
      dependsOn: [backend, frontend]
      skipWhen:
        - "resources.backend.status.state == 'Failed'"
      spec:
        category: "active"
        executionInterval: 1m
```

The Bundle creates resources in dependency order, substitutes cross-references, and reports
aggregate health. Deleting the Bundle garbage-collects all child resources via owner
references.

---

## Checklist

Before generating, verify your spec follows these rules:

- [ ] Use OpenAPI 3.0 or 3.1 (Swagger 2.0 works but 3.x is preferred)
- [ ] All request/response bodies use `application/json`
- [ ] Schemas are flat — no `oneOf`, `anyOf`, `allOf`
- [ ] No circular `$ref` chains
- [ ] Enums use string values only
- [ ] Path parameters have explicit types with format (`integer` + `int64`)
- [ ] CRUD resources have both collection path (`/things`) and instance path (`/things/{id}`)
- [ ] Instance paths include GET for drift detection
- [ ] State changes (pause/resume/enable/disable) are modeled as schema fields, not separate endpoints
- [ ] Response schemas are fully typed (not empty or `{}`)
- [ ] `operationId` is set on every operation for filtering support
- [ ] Map types use array-of-objects pattern instead of `additionalProperties`
- [ ] Request body and GET response schemas share field names (required for drift detection)
- [ ] `x-k8s-id-field` is set on path params when the name differs from the body ID field
