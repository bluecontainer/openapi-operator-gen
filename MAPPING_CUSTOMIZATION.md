# Mapping Customization Options

This document outlines current and potential customization options for mapping OpenAPI schemas to Kubernetes CRD definitions.

## Current Options

| Option | Flag | Description |
|--------|------|-------------|
| **Mapping Mode** | `--mapping` | `per-resource` (one CRD per REST resource) or `single-crd` (one CRD for entire API) |
| **API Group** | `--group` | Kubernetes API group (e.g., `myapp.example.com`) |
| **API Version** | `--version` | Kubernetes API version (e.g., `v1alpha1`) |
| **CRD Generation** | `--generate-crds` | Generate CRD YAML directly vs. use controller-gen |
| **Include Paths** | `--include-paths` | Only generate CRDs for paths matching these patterns (comma-separated, glob supported) |
| **Exclude Paths** | `--exclude-paths` | Skip paths matching these patterns (comma-separated, glob supported) |
| **Include Tags** | `--include-tags` | Only include endpoints with these OpenAPI tags (comma-separated) |
| **Exclude Tags** | `--exclude-tags` | Exclude endpoints with these OpenAPI tags (comma-separated) |
| **Include Operations** | `--include-operations` | Only include operations with these operationIds (comma-separated, glob supported) |
| **Exclude Operations** | `--exclude-operations` | Exclude operations with these operationIds (comma-separated, glob supported) |
| **Update With POST** | `--update-with-post` | Use POST for updates when PUT is not available. Value: `*` for all, or comma-separated paths (e.g., `/store/order,/users/*`) |
| **ID Field Merge** | `--id-field-map` | Explicit mapping of path params to body fields (e.g., `orderId=id,petId=id`) |
| **Disable ID Merge** | `--no-id-merge` | Disable automatic merging of path ID parameters with body 'id' fields |

## Resource Selection/Filtering (Implemented)

Filter which OpenAPI paths become CRDs using path patterns, OpenAPI tags, and/or operationIds.

### Path Filtering

```bash
# Include only specific paths
--include-paths="/users,/pets,/orders"

# Exclude paths with wildcards
--exclude-paths="/internal/*,/admin/*"

# Combine include and exclude (exclude takes precedence)
--include-paths="/api/*" --exclude-paths="/api/internal/*"
```

### Pattern Syntax

| Pattern | Description | Example Match |
|---------|-------------|---------------|
| `/users` | Exact match | `/users` only |
| `/users/*` | Prefix match (any depth) | `/users`, `/users/123`, `/users/123/profile` |
| `/users/?` | Single segment wildcard | `/users/123` but not `/users/123/profile` |
| `/api/*/users` | Glob with segment wildcard | `/api/v1/users`, `/api/v2/users` |

### Tag Filtering

Filter by OpenAPI operation tags:

```bash
# Include only endpoints tagged "public" or "v2"
--include-tags="public,v2"

# Exclude deprecated or internal endpoints
--exclude-tags="deprecated,internal"

# Combine with path filtering
--include-paths="/api/*" --exclude-tags="deprecated"
```

Tag matching is case-insensitive. Exclude tags take precedence over include tags.

### OperationId Filtering

Filter by OpenAPI operationId:

```bash
# Include only specific operations
--include-operations="getPetById,createPet,updatePet"

# Include operations matching a pattern
--include-operations="get*,create*"

# Exclude specific operations
--exclude-operations="deletePet,*Deprecated"

# Combine with path and tag filtering
--include-paths="/pet/*" --include-operations="get*" --exclude-tags="deprecated"
```

#### OperationId Pattern Syntax

| Pattern | Description | Example Match |
|---------|-------------|---------------|
| `getPetById` | Exact match | `getPetById` only |
| `get*` | Prefix match | `getPetById`, `getUser`, `getPets` |
| `*Pet` | Suffix match | `createPet`, `updatePet`, `deletePet` |
| `*Pet*` | Contains match | `getPetById`, `updatePetStatus` |
| `get*ById` | Complex glob | `getPetById`, `getUserById` |

OperationId matching is case-sensitive. Exclude patterns take precedence over include patterns.

### Filtering Behavior

- **No filters**: All paths are processed (default)
- **Include only**: Only matching paths/tags/operations are processed
- **Exclude only**: All paths except matching ones are processed
- **Both**: Path must match include AND not match exclude; tags must match include AND not match exclude; at least one operationId must pass
- **Exclude precedence**: If a path/tag/operation matches both include and exclude, it is excluded
- **OperationId logic**: A path is included if ANY of its operationIds passes the operation filter (logical OR)

## ID Field Merging (Implemented)

Many REST APIs use path parameters like `/store/order/{orderId}` where the `orderId` value is the same as an `id` field in the request/response body. By default, the generator would create two fields: `OrderId` (from path) and `Id` (from body), leading to redundant data.

ID field merging automatically detects and merges these duplicate fields, resulting in a cleaner CRD with a single `Id` field that is used both in the URL path and the request body.

### Auto-Detection (Default)

The generator automatically detects when a path parameter like `{orderId}` should map to a body field `id`:

- For an `Order` resource, `orderId` is auto-detected to map to `id`
- For a `Pet` resource, `petId` is auto-detected to map to `id`
- Pattern: `{kindName}Id` → `id`

This is enabled by default. The generated CRD will have a single `Id` field instead of both `Id` and `OrderId`.

### Explicit Mapping

Use `--id-field-map` for cases where auto-detection doesn't match your API:

```bash
# Map orderId path param to orderNumber body field
openapi-operator-gen generate \
  --spec api.yaml \
  --id-field-map="orderId=orderNumber,userId=externalId" \
  --group myapp.example.com
```

Multiple mappings can be comma-separated. Each mapping is `pathParam=bodyField`.

### OpenAPI Extension

You can also specify the mapping in your OpenAPI spec using the `x-k8s-id-field` extension:

```yaml
paths:
  /store/order/{orderId}:
    get:
      parameters:
        - name: orderId
          in: path
          required: true
          schema:
            type: integer
            format: int64
          x-k8s-id-field: id  # Maps this path param to body's "id" field
```

### Disabling Auto-Detection

If auto-detection causes issues, disable it with `--no-id-merge`:

```bash
openapi-operator-gen generate \
  --spec api.yaml \
  --no-id-merge \
  --group myapp.example.com
```

With auto-detection disabled:
- Path parameters and body fields are kept as separate fields
- Explicit `--id-field-map` still works
- `x-k8s-id-field` extension still works

### Priority Order

When determining the mapping for a path parameter:

1. **Explicit `--id-field-map`** (highest priority)
2. **`x-k8s-id-field` OpenAPI extension**
3. **Auto-detection** (if `--no-id-merge` is not set)

### Example

Given this OpenAPI spec:

```yaml
paths:
  /store/order/{orderId}:
    get:
      operationId: getOrderById
      parameters:
        - name: orderId
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
                $ref: '#/components/schemas/Order'
components:
  schemas:
    Order:
      properties:
        id:
          type: integer
          format: int64
        petId:
          type: integer
          format: int64
```

**Without ID merging** (using `--no-id-merge`):
```go
type OrderSpec struct {
    OrderId int64 `json:"orderId"`       // From path param
    Id      *int64 `json:"id,omitempty"` // From body schema
    PetId   *int64 `json:"petId,omitempty"`
}
```

**With ID merging** (default):
```go
type OrderSpec struct {
    Id    *int64 `json:"id,omitempty"`    // Merged (used for path AND body)
    PetId *int64 `json:"petId,omitempty"`
}
```

The generated controller automatically uses `spec.Id` to build the URL `/store/order/{orderId}`.

## Potential Customization Options

### 1. Resource Selection/Filtering

✅ **Implemented** - See above for usage details.

### 2. Naming Customization

```bash
--kind-prefix="My"                  # MyUser, MyPet instead of User, Pet
--kind-suffix="Resource"            # UserResource, PetResource
--plural-override="user=people"     # Custom plural forms
--short-names="user=u,usr"          # Custom short names
```

### 3. Field Mapping

```bash
--field-rename="user_name=userName" # Rename specific fields
--ignore-fields="internalId,_links" # Skip certain fields
--required-fields="name,id"         # Force fields to be required
--optional-fields="*"               # Make all fields optional
```

### 4. Type Mapping Overrides

```bash
--type-map="date-time=string"       # Override format mappings
--type-map="uuid=string"            # Custom format handling
--nullable-as-pointer=true          # Use pointers for nullable fields
```

### 5. Validation Customization

```bash
--validation-mode=strict|loose      # How to handle OpenAPI validation rules
--ignore-validation                 # Skip validation markers
--additional-validation="name:MinLength=1"
```

### 6. Scope Configuration

```bash
--scope=Namespaced|Cluster          # CRD scope (default: Namespaced)
--scope-override="Config=Cluster"   # Per-resource scope
```

### 7. Schema Source Selection

```bash
--schema-source=request|response    # Use request body or response schema
--schema-ref="Pet"                  # Use specific component schema
--merge-schemas=true                # Merge request+response schemas
```

### 8. Operation Mapping

```bash
--reconcile-on=POST,PUT             # Which operations trigger reconcile
--skip-delete-handling              # Don't generate finalizer logic
--custom-action-map="PATCH=Update"  # Override action mapping
```

### 9. Additional Spec Fields

```bash
--add-owner-ref=true                # Add ownerReference field
--add-finalizers=true               # Add finalizers field
--add-annotations=true              # Add annotations field to spec
```

### 10. Config File Support

```yaml
# openapi-operator-gen.yaml
mappings:
  - path: /users
    kind: Account
    plural: accounts
    scope: Cluster
    shortNames: [acc, acct]
    ignoreFields: [internalId]
  - path: /pets
    kind: Pet
    additionalFields:
      - name: owner
        type: string
        required: true
```

## Implementation Priority Recommendation

| Priority | Feature | Rationale | Status |
|----------|---------|-----------|--------|
| **High** | Resource filtering (`--include-paths`, `--exclude-paths`) | Essential for large APIs where only some resources need CRDs | ✅ Implemented |
| **High** | Tag filtering (`--include-tags`, `--exclude-tags`) | Filter by OpenAPI tags for semantic grouping | ✅ Implemented |
| **High** | OperationId filtering (`--include-operations`, `--exclude-operations`) | Filter by operationId for fine-grained control | ✅ Implemented |
| **High** | Config file support for complex mappings | Enables reproducible, version-controlled configurations | Planned |
| **Medium** | Naming customization (prefix, suffix, overrides) | Helps avoid naming conflicts and match conventions | Planned |
| **Medium** | Scope configuration | Some resources are naturally cluster-scoped | Planned |
| **Medium** | Field filtering (`--ignore-fields`) | APIs often have internal fields not needed in CRDs | Planned |
| **Lower** | Type mapping overrides | Most types are handled automatically | Planned |
| **Lower** | Validation customization | OpenAPI validation translates well to kubebuilder markers | Planned |

## Example Use Cases

### Large API with Selective CRD Generation

```bash
openapi-operator-gen generate \
  --spec api.yaml \
  --include-paths="/users,/orders,/products" \
  --exclude-paths="/internal/*,/admin/*" \
  --group myapp.example.com \
  --version v1
```

### Filter by OpenAPI Tags

```bash
# Generate CRDs only for endpoints tagged "public"
openapi-operator-gen generate \
  --spec api.yaml \
  --include-tags="public" \
  --group myapp.example.com \
  --version v1

# Exclude deprecated endpoints
openapi-operator-gen generate \
  --spec api.yaml \
  --exclude-tags="deprecated,internal" \
  --group myapp.example.com \
  --version v1
```

### Combined Path and Tag Filtering

```bash
# Include only /api/* paths that are tagged "v2", excluding deprecated
openapi-operator-gen generate \
  --spec api.yaml \
  --include-paths="/api/*" \
  --include-tags="v2" \
  --exclude-tags="deprecated" \
  --group myapp.example.com \
  --version v1
```

### Filter by OperationId

```bash
# Only include read operations (get*)
openapi-operator-gen generate \
  --spec api.yaml \
  --include-operations="get*,find*,list*" \
  --group myapp.example.com \
  --version v1

# Exclude delete operations
openapi-operator-gen generate \
  --spec api.yaml \
  --exclude-operations="delete*,remove*" \
  --group myapp.example.com \
  --version v1

# Combine all filters: only Pet endpoints with get operations, excluding deprecated
openapi-operator-gen generate \
  --spec api.yaml \
  --include-paths="/pet/*" \
  --include-tags="pet" \
  --include-operations="get*,find*" \
  --exclude-tags="deprecated" \
  --group myapp.example.com \
  --version v1
```

### Custom Naming for Multi-tenant Operator

```bash
openapi-operator-gen generate \
  --spec api.yaml \
  --kind-prefix="Acme" \
  --short-names="user=au,pet=ap" \
  --group acme.example.com \
  --version v1alpha1
```

### Config File for Complex Mapping

```yaml
# openapi-operator-gen.yaml
apiGroup: myapp.example.com
apiVersion: v1
moduleName: github.com/myorg/myapp-operator

defaults:
  scope: Namespaced
  ignoreFields: [_links, _embedded]

mappings:
  - path: /users
    kind: User
    shortNames: [usr]
    requiredFields: [email]

  - path: /configurations
    kind: AppConfig
    scope: Cluster
    plural: appconfigs

  - path: /orders
    kind: Order
    ignoreFields: [internalStatus, auditLog]
    additionalFields:
      - name: priority
        type: integer
        default: 0
```

```bash
openapi-operator-gen generate --config openapi-operator-gen.yaml --spec api.yaml
```
