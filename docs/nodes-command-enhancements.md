# kubectl plugin `nodes` Command Enhancements

This document describes proposed enhancements to the `nodes` command for greater control over workload discovery. All filtering options are designed with symmetric include/exclude pairs where applicable.

## Design Principles

1. **Symmetric options**: Every include option has a corresponding exclude option
2. **Additive includes, subtractive excludes**: Includes whitelist, excludes blacklist
3. **Excludes take precedence**: If a workload matches both include and exclude, it is excluded
4. **Sensible defaults**: Without flags, discover all workloads (current behavior)
5. **Composable**: Multiple flags can be combined for precise filtering

## Option Categories

### 1. Workload Type Filtering

Filter by the type of Kubernetes workload.

| Flag | Description | Example |
|------|-------------|---------|
| `--types` | Only include these workload types (comma-separated) | `--types=helm-release,statefulset` |
| `--exclude-types` | Exclude these workload types | `--exclude-types=deployment` |

**Valid types:** `helm-release`, `statefulset`, `deployment`

**Examples:**
```bash
# Only StatefulSets and Helm releases
kubectl petstore nodes --types=statefulset,helm-release

# Everything except Deployments
kubectl petstore nodes --exclude-types=deployment

# Equivalent to above
kubectl petstore nodes --types=statefulset,helm-release
```

**Precedence:** If both are specified, `--exclude-types` removes from the `--types` set.

---

### 2. Label Filtering

Filter workloads by Kubernetes labels.

| Flag | Description | Example |
|------|-------------|---------|
| `--selector`, `-l` | Only include workloads matching label selector | `-l app=petstore` |
| `--exclude-labels` | Exclude workloads matching any of these labels | `--exclude-labels=tier=control-plane` |

**Selector syntax:** Standard Kubernetes label selector (equality, set-based)
- `app=petstore` - equality
- `app!=canary` - inequality
- `environment in (prod,staging)` - set membership
- `!experimental` - label absence

**Examples:**
```bash
# Only workloads with app=petstore
kubectl petstore nodes -l app=petstore

# Exclude control plane components
kubectl petstore nodes --exclude-labels="app.kubernetes.io/component=operator"

# Combine: petstore workloads, but not the operator
kubectl petstore nodes -l app.kubernetes.io/part-of=petstore --exclude-labels="app.kubernetes.io/component=operator"
```

**Precedence:** `--exclude-labels` is applied after `--selector` filtering.

---

### 3. Name Pattern Filtering

Filter workloads by name using glob patterns.

| Flag | Description | Example |
|------|-------------|---------|
| `--name-pattern` | Only include workloads matching glob pattern | `--name-pattern=petstore-*` |
| `--exclude-pattern` | Exclude workloads matching glob pattern | `--exclude-pattern=*-canary` |

**Pattern syntax:** Shell-style glob patterns
- `*` matches any sequence of characters
- `?` matches any single character
- `[abc]` matches any character in the set
- `[a-z]` matches any character in the range

**Examples:**
```bash
# Only workloads starting with "petstore-"
kubectl petstore nodes --name-pattern="petstore-*"

# Exclude canary deployments
kubectl petstore nodes --exclude-pattern="*-canary"

# Include prod/staging, exclude canaries
kubectl petstore nodes --name-pattern="*-prod-*" --name-pattern="*-staging-*" --exclude-pattern="*-canary"
```

**Multiple patterns:** Multiple `--name-pattern` flags are OR'd (include if any match). Multiple `--exclude-pattern` flags are also OR'd (exclude if any match).

**Precedence:** Excludes are applied after includes.

---

### 4. Namespace Filtering

Filter workloads by namespace.

| Flag | Description | Example |
|------|-------------|---------|
| `-n`, `--namespace` | Only discover in this namespace (existing flag) | `-n production` |
| `-A`, `--all-namespaces` | Discover across all namespaces (existing flag) | `-A` |
| `--namespaces` | Only include these namespaces (comma-separated) | `--namespaces=prod,staging` |
| `--exclude-namespaces` | Exclude these namespaces | `--exclude-namespaces=kube-system` |
| `--namespace-pattern` | Only include namespaces matching glob | `--namespace-pattern=prod-*` |
| `--exclude-namespace-pattern` | Exclude namespaces matching glob | `--exclude-namespace-pattern=*-canary` |

**Examples:**
```bash
# All namespaces except system ones
kubectl petstore nodes -A --exclude-namespaces=kube-system,kube-public,kube-node-lease

# Only production namespaces
kubectl petstore nodes -A --namespace-pattern="prod-*"

# Production namespaces, but not canary environments
kubectl petstore nodes -A --namespace-pattern="prod-*" --exclude-namespace-pattern="*-canary"
```

**Interaction with `-n`/`-A`:**
- `-n namespace` is equivalent to `--namespaces=namespace`
- `-A` enables multi-namespace mode; `--namespaces` and `--exclude-namespaces` filter within that
- Without `-A`, only the single namespace from `-n` or kubeconfig default is queried

---

### 5. Health Filtering

Filter workloads by pod health status.

| Flag | Description | Example |
|------|-------------|---------|
| `--healthy-only` | Only include workloads with all pods running | `--healthy-only` |
| `--unhealthy-only` | Only include workloads with some pods not running | `--unhealthy-only` |
| `--min-healthy-percent` | Only include workloads with at least N% healthy pods | `--min-healthy-percent=50` |
| `--max-healthy-percent` | Only include workloads with at most N% healthy pods | `--max-healthy-percent=99` |

**Examples:**
```bash
# Only fully healthy workloads
kubectl petstore nodes --healthy-only

# Only degraded workloads (for alerting/remediation)
kubectl petstore nodes --unhealthy-only

# Workloads with partial availability (50-99% healthy)
kubectl petstore nodes --min-healthy-percent=50 --max-healthy-percent=99
```

**Note:** `--healthy-only` is equivalent to `--min-healthy-percent=100`. `--unhealthy-only` is equivalent to `--max-healthy-percent=99`.

---

### 6. Replica Count Filtering

Filter workloads by replica count.

| Flag | Description | Example |
|------|-------------|---------|
| `--min-replicas` | Only include workloads with at least N replicas | `--min-replicas=2` |
| `--max-replicas` | Only include workloads with at most N replicas | `--max-replicas=10` |

**Examples:**
```bash
# Exclude single-replica workloads (likely not HA)
kubectl petstore nodes --min-replicas=2

# Only small workloads
kubectl petstore nodes --max-replicas=3

# Medium-sized workloads
kubectl petstore nodes --min-replicas=3 --max-replicas=10
```

---

### 7. Operator Exclusion (Convenience)

A convenience flag that combines common patterns to exclude the operator itself.

| Flag | Description | Example |
|------|-------------|---------|
| `--exclude-operator` | Exclude the operator controller-manager | `--exclude-operator` |
| `--include-operator` | Explicitly include the operator (overrides exclude) | `--include-operator` |

**What `--exclude-operator` excludes:**
- Workloads with label `app.kubernetes.io/component=operator`
- Workloads with label `control-plane=controller-manager`
- Workloads with name matching `*-controller-manager`
- Workloads with name matching `*-operator`

**Examples:**
```bash
# Typical usage: exclude the operator
kubectl petstore nodes --exclude-operator

# Discover only the operator (for operator health checks)
kubectl petstore nodes --include-operator --exclude-operator=false
# Or simply:
kubectl petstore nodes -l control-plane=controller-manager
```

**Default:** Consider making `--exclude-operator` default to `true` in the Rundeck plugin configuration.

---

### 8. Tag Injection

Add custom tags to discovered nodes.

| Flag | Description | Example |
|------|-------------|---------|
| `--add-tags` | Add custom tags to all nodes (comma-separated) | `--add-tags=prod,team-platform` |
| `--remove-tags` | Remove auto-generated tags | `--remove-tags=namespace` |
| `--labels-as-tags` | Add specific K8s labels as tags | `--labels-as-tags=environment,tier` |

**Auto-generated tags:** By default, nodes get tags for workload type and namespace (e.g., `helm-release,production`).

**Examples:**
```bash
# Add environment tag
kubectl petstore nodes --add-tags=environment:prod

# Use labels as tags for Rundeck filtering
kubectl petstore nodes --labels-as-tags=app.kubernetes.io/component,tier

# Clean output (no auto-tags, only custom)
kubectl petstore nodes --remove-tags=workload-type,namespace --add-tags=custom-only
```

---

### 9. Attribute Injection

Expose additional data as Rundeck node attributes.

| Flag | Description | Example |
|------|-------------|---------|
| `--label-attributes` | Expose K8s labels as node attributes | `--label-attributes=version,environment` |
| `--annotation-attributes` | Expose K8s annotations as node attributes | `--annotation-attributes=description` |
| `--exclude-attributes` | Remove attributes from output | `--exclude-attributes=podCount` |

**Examples:**
```bash
# Include version label as attribute
kubectl petstore nodes --label-attributes=app.kubernetes.io/version

# Output includes: {"version": "1.2.3", ...}

# Minimal output (only targeting attributes)
kubectl petstore nodes --exclude-attributes=podCount,healthyPods,workloadKind,workloadName
```

---

### 10. Output Control

Control the output format and content.

| Flag | Description | Example |
|------|-------------|---------|
| `--output`, `-o` | Output format | `-o json` (default), `-o yaml`, `-o table`, `-o names` |
| `--compact` | Compact JSON output (no pretty-printing) | `--compact` |
| `--sort-by` | Sort nodes by attribute | `--sort-by=healthyPods` |
| `--reverse` | Reverse sort order | `--reverse` |

**Output formats:**
- `json` - Rundeck resource model JSON (default)
- `yaml` - Rundeck resource model YAML
- `table` - Human-readable table for debugging
- `names` - Just node names, one per line
- `wide` - Table with all attributes

**Examples:**
```bash
# Debug output
kubectl petstore nodes -o table

# Sorted by health (unhealthy first)
kubectl petstore nodes -o table --sort-by=healthyPods

# Just names for scripting
kubectl petstore nodes -o names | xargs -I{} echo "Node: {}"
```

---

## Rundeck Plugin Configuration

All flags should be exposed as plugin configuration properties:

```yaml
providers:
  - name: petstore-k8s-nodes
    service: ResourceModelSource
    plugin-type: script
    script-interpreter: /bin/bash
    script-file: nodes.sh
    resource-format: resourcejson
    config:
      # Existing
      - name: k8s_token
        type: String
        required: true
        renderingOptions:
          valueConversion: "STORAGE_PATH_AUTOMATIC_READ"
      - name: k8s_url
        type: String
        required: true
      - name: namespace
        type: String
        default: "petstore-system"
      - name: execution_mode
        type: Select
        values: "native,docker,kubernetes"
        default: "native"

      # New filtering options
      - name: workload_types
        title: "Workload Types"
        type: String
        description: "Comma-separated: helm-release, statefulset, deployment (empty = all)"
      - name: exclude_types
        title: "Exclude Types"
        type: String
        description: "Workload types to exclude"

      - name: label_selector
        title: "Label Selector"
        type: String
        description: "Kubernetes label selector (e.g., app=petstore)"
      - name: exclude_labels
        title: "Exclude Labels"
        type: String
        description: "Exclude workloads with these labels (comma-separated key=value)"

      - name: name_pattern
        title: "Name Pattern"
        type: String
        description: "Glob pattern for workload names to include"
      - name: exclude_pattern
        title: "Exclude Pattern"
        type: String
        description: "Glob pattern for workload names to exclude"

      - name: exclude_namespaces
        title: "Exclude Namespaces"
        type: String
        description: "Namespaces to exclude (comma-separated)"

      - name: healthy_only
        title: "Healthy Only"
        type: Boolean
        default: "false"
        description: "Only discover workloads with all pods running"

      - name: exclude_operator
        title: "Exclude Operator"
        type: Boolean
        default: "true"
        description: "Exclude the operator controller-manager from discovery"

      - name: additional_tags
        title: "Additional Tags"
        type: String
        description: "Extra tags to add to all nodes (comma-separated)"

      - name: label_attributes
        title: "Label Attributes"
        type: String
        description: "K8s labels to expose as node attributes (comma-separated label keys)"
```

---

## Flag Summary Table

| Category | Include Flag | Exclude Flag | Default Behavior |
|----------|--------------|--------------|------------------|
| Workload Type | `--types` | `--exclude-types` | All types |
| Labels | `--selector`, `-l` | `--exclude-labels` | No label filtering |
| Name | `--name-pattern` | `--exclude-pattern` | All names |
| Namespace | `--namespaces`, `--namespace-pattern` | `--exclude-namespaces`, `--exclude-namespace-pattern` | Single namespace from `-n` |
| Health | `--healthy-only`, `--min-healthy-percent` | `--unhealthy-only`, `--max-healthy-percent` | All health states |
| Replicas | `--min-replicas` | `--max-replicas` | All replica counts |
| Operator | `--include-operator` | `--exclude-operator` | Include operator |
| Tags | `--add-tags`, `--labels-as-tags` | `--remove-tags` | Auto-generated tags |
| Attributes | `--label-attributes`, `--annotation-attributes` | `--exclude-attributes` | Standard attributes |

---

## Implementation Priority

### Phase 1: Core Filtering (High Value)
1. `--types` / `--exclude-types`
2. `--selector` / `--exclude-labels`
3. `--exclude-operator` (default: true in Rundeck plugin)
4. `--healthy-only`

### Phase 2: Pattern Matching
5. `--name-pattern` / `--exclude-pattern`
6. `--exclude-namespaces`
7. `--namespace-pattern` / `--exclude-namespace-pattern`

### Phase 3: Advanced Filtering
8. `--min-replicas` / `--max-replicas`
9. `--min-healthy-percent` / `--max-healthy-percent`

### Phase 4: Output Customization
10. `--add-tags` / `--labels-as-tags`
11. `--label-attributes` / `--annotation-attributes`
12. `-o table` / `-o names` output formats

---

## Example Workflows

### Production-only workloads, excluding operator
```bash
kubectl petstore nodes -A \
  --namespace-pattern="prod-*" \
  --exclude-operator \
  --healthy-only
```

### Canary deployments for monitoring
```bash
kubectl petstore nodes -A \
  --name-pattern="*-canary" \
  --types=deployment \
  --label-attributes=app.kubernetes.io/version
```

### Database StatefulSets with health info
```bash
kubectl petstore nodes \
  --types=statefulset \
  -l app.kubernetes.io/component=database \
  -o table --sort-by=healthyPods
```

### Degraded workloads for alerting
```bash
kubectl petstore nodes -A \
  --unhealthy-only \
  --exclude-operator \
  -o names
```
