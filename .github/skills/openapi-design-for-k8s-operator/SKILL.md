---
name: openapi-design-for-k8s-operator
description: >
  Guide for designing OpenAPI specifications that produce high-quality Kubernetes operators
  via openapi-operator-gen. Covers endpoint classification, schema constraints, path structure,
  field mapping, and best practices for CRUD resources, query endpoints, and action endpoints.
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
