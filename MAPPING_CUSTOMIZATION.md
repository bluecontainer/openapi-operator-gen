# Mapping Customization Options

This document outlines current and potential customization options for mapping OpenAPI schemas to Kubernetes CRD definitions.

## Current Options

| Option | Flag | Description |
|--------|------|-------------|
| **Mapping Mode** | `--mapping` | `per-resource` (one CRD per REST resource) or `single-crd` (one CRD for entire API) |
| **API Group** | `--group` | Kubernetes API group (e.g., `myapp.example.com`) |
| **API Version** | `--version` | Kubernetes API version (e.g., `v1alpha1`) |
| **CRD Generation** | `--generate-crds` | Generate CRD YAML directly vs. use controller-gen |

## Potential Customization Options

### 1. Resource Selection/Filtering

```bash
--include-paths="/users,/pets"      # Only generate CRDs for specific paths
--exclude-paths="/internal/*"       # Skip certain paths
--include-tags="public"             # Filter by OpenAPI tags
--exclude-tags="deprecated"
```

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

| Priority | Feature | Rationale |
|----------|---------|-----------|
| **High** | Resource filtering (`--include-paths`, `--exclude-paths`) | Essential for large APIs where only some resources need CRDs |
| **High** | Config file support for complex mappings | Enables reproducible, version-controlled configurations |
| **Medium** | Naming customization (prefix, suffix, overrides) | Helps avoid naming conflicts and match conventions |
| **Medium** | Scope configuration | Some resources are naturally cluster-scoped |
| **Medium** | Field filtering (`--ignore-fields`) | APIs often have internal fields not needed in CRDs |
| **Lower** | Type mapping overrides | Most types are handled automatically |
| **Lower** | Validation customization | OpenAPI validation translates well to kubebuilder markers |

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
