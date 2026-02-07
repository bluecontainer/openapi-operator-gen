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

---

## Standalone Plugin Architecture

The `nodes` command can be extracted as a standalone kubectl plugin (`kubectl-rundeck-nodes`) that is also used as a library by generated plugins. This provides broader distribution (via Krew) while keeping generated plugins in sync with enhancements.

### Comparison with Existing Plugins

| Plugin | Function | Similarity to `nodes` |
|--------|----------|----------------------|
| [ketall](https://github.com/corneliusweig/ketall) | Lists ALL resources in a cluster | Comprehensive discovery, but outputs raw K8s resources, not Rundeck format |
| [kubectl-tree](https://github.com/ahmetb/kubectl-tree) | Shows object hierarchies via ownerReferences | Relationship visualization, but not workload-focused |
| [lineage](https://krew.sigs.k8s.io/plugins/) | Like tree but understands logical relationships | Better for understanding Helm release relationships |
| [resource-capacity](https://github.com/robscott/kube-capacity) | Shows resource requests/limits/utilization | Health/capacity focus, but for K8s nodes not workloads |
| [kubectl-ansible](https://github.com/moshloop/kubectl-ansible) | Dynamic Ansible inventory from K8s cluster | Inventory export, but for Ansible not Rundeck |

**Gap filled by `nodes`:** No existing plugin provides Rundeck resource model JSON output with workload-centric discovery, targeting attributes, multi-cluster credentials, and integrated health reporting.

### Current Dependencies on Generated Plugin

| Dependency | Location | Extraction Difficulty |
|------------|----------|----------------------|
| `k8sClient.DynamicClient()` | Line 96 | Easy - standard client-go |
| `k8sClient.GetNamespace()` | Line 101 | Easy - kubeconfig loader |
| `{{ .PluginName }}` in `defaultClusterTokenSuffix` | Line 25 | Easy - make configurable via flag |
| `{{ .PluginName }}` in help text | Lines 76-82 | Easy - parameterize or use generic name |

**What's already generic:**
- API calls use only standard `apps/v1` (StatefulSets, Deployments) and `v1` (Pods)
- Output is standard Rundeck resource model JSON format
- All imports are standard k8s client-go packages
- Helm release detection via `app.kubernetes.io/instance` label is universal

### Proposed Library Structure

```
github.com/bluecontainer/kubectl-rundeck-nodes/
├── cmd/kubectl-rundeck-nodes/
│   └── main.go                    # Standalone CLI entrypoint
├── pkg/nodes/
│   ├── discover.go                # Core workload discovery logic
│   ├── types.go                   # RundeckNode struct, Options
│   ├── output.go                  # JSON/table/yaml formatters
│   └── filters.go                 # Type/label/pattern filtering
└── go.mod
```

### Library Interface

```go
// pkg/nodes/types.go
package nodes

type Options struct {
    Namespace           string
    AllNamespaces       bool
    LabelSelector       string
    ClusterName         string
    ClusterURL          string
    DefaultTokenSuffix  string   // Caller provides their default

    // Phase 1 enhancements
    Types               []string // helm-release, statefulset, deployment
    ExcludeTypes        []string
    ExcludeOperator     bool
    HealthyOnly         bool
}

type RundeckNode struct {
    NodeName           string `json:"nodename"`
    Hostname           string `json:"hostname"`
    Tags               string `json:"tags"`
    OSFamily           string `json:"osFamily"`
    NodeExecutor       string `json:"node-executor"`
    FileCopier         string `json:"file-copier"`
    Cluster            string `json:"cluster,omitempty"`
    ClusterURL         string `json:"clusterUrl,omitempty"`
    ClusterTokenSuffix string `json:"clusterTokenSuffix,omitempty"`
    TargetType         string `json:"targetType"`
    TargetValue        string `json:"targetValue"`
    TargetNamespace    string `json:"targetNamespace"`
    WorkloadKind       string `json:"workloadKind"`
    WorkloadName       string `json:"workloadName"`
    PodCount           string `json:"podCount"`
    HealthyPods        string `json:"healthyPods"`
}

// Discover queries K8s and returns Rundeck nodes
func Discover(ctx context.Context, client dynamic.Interface, opts Options) (map[string]*RundeckNode, error)

// Output writes nodes in the requested format
func Output(w io.Writer, nodes map[string]*RundeckNode, format string) error
```

### Standalone Plugin Usage

```go
// cmd/kubectl-rundeck-nodes/main.go
func main() {
    var opts nodes.Options

    cmd := &cobra.Command{
        Use: "kubectl-rundeck-nodes",
        RunE: func(cmd *cobra.Command, args []string) error {
            dynClient := initClient()

            result, err := nodes.Discover(ctx, dynClient, opts)
            if err != nil {
                return err
            }
            return nodes.Output(os.Stdout, result, outputFormat)
        },
    }

    cmd.Flags().StringVar(&opts.DefaultTokenSuffix, "default-token-suffix",
        "rundeck/k8s-token", "Default Rundeck Key Storage path suffix")
    // ... other flags from enhancements
}
```

### Generated Plugin Usage

```go
// Generated nodes_cmd.go (minimal template)
import "github.com/bluecontainer/kubectl-rundeck-nodes/pkg/nodes"

var nodesCmd = nodes.NewCommand(nodes.CommandOptions{
    DefaultTokenSuffix: "project/{{ .PluginName }}-operator/k8s-token",
    GetDynamicClient:   func() dynamic.Interface { return k8sClient.DynamicClient() },
    GetNamespace:       func() string { return k8sClient.GetNamespace() },
})
```

### Benefits

| Aspect | Benefit |
|--------|---------|
| **Single implementation** | Bug fixes and enhancements apply everywhere |
| **Krew distribution** | Standalone plugin installable via `kubectl krew install rundeck-nodes` |
| **Generated plugins** | Import library, just provide config (token suffix, plugin name) |
| **Enhancements** | Phase 1-4 features benefit all users |
| **Testing** | One test suite covers both use cases |
| **Versioning** | Generated plugins can pin to specific library versions |

### Implementation Phases

1. **Extract library** - Move discovery logic to `pkg/nodes/` with configurable options
2. **Create standalone CLI** - Wrap library in `cmd/kubectl-rundeck-nodes/`
3. **Publish to Krew** - Submit to krew-index for distribution
4. **Update generator** - Template imports library instead of embedding implementation
5. **Regenerate examples** - Verify generated plugins still work

---

## Standalone Rundeck Plugin

In addition to the kubectl plugin, a standalone Rundeck ResourceModelSource plugin can be created that works with any Kubernetes cluster without requiring a generated operator.

### Two Standalone Plugins

```
1. kubectl-rundeck-nodes     # kubectl plugin (Go binary)
   └── Discovers K8s workloads, outputs Rundeck JSON

2. rundeck-k8s-nodes         # Rundeck plugin (ZIP)
   └── ResourceModelSource that invokes the kubectl plugin
```

### Comparison: Generated vs Standalone

| Aspect | Generated Plugin | Standalone Plugin |
|--------|------------------|-------------------|
| Name | `petstore-k8s-nodes` | `rundeck-k8s-nodes` |
| Command | `petstore nodes` | `kubectl-rundeck-nodes` |
| Default namespace | Operator namespace | Configurable |
| Token suffix | `project/petstore-operator/k8s-token` | Configurable |
| Binary bundled | In operator container | Separate install or bundled |
| Requires operator | Yes | No |

### Rundeck Plugin Structure

```
rundeck-k8s-nodes/
├── plugin.yaml              # ResourceModelSource plugin definition
├── contents/
│   └── nodes.sh             # Script wrapper for kubectl-rundeck-nodes
├── Makefile                 # Builds the ZIP
└── README.md
```

### plugin.yaml (Standalone)

```yaml
name: rundeck-k8s-nodes
rundeckPluginVersion: 2.0
author: "bluecontainer"
description: "Kubernetes workload discovery for Rundeck node source"
pluginType: script
providers:
  - name: k8s-workload-nodes
    service: ResourceModelSource
    plugin-type: script
    script-interpreter: /bin/bash
    script-file: nodes.sh
    resource-format: resourcejson
    config:
      - name: k8s_token
        type: String
        title: Kubernetes Token
        required: true
        renderingOptions:
          valueConversion: "STORAGE_PATH_AUTOMATIC_READ"
      - name: k8s_url
        type: String
        title: Kubernetes API URL
        required: true
      - name: namespace
        type: String
        title: Namespace
        description: "Namespace to discover workloads (empty = all with -A)"
      - name: label_selector
        type: String
        title: Label Selector
        description: "Filter workloads by labels (e.g., app=myapp)"
      - name: execution_mode
        type: Select
        title: Execution Mode
        values: "native,docker"
        default: "native"
      - name: docker_image
        type: String
        title: Docker Image
        description: "Image containing kubectl-rundeck-nodes"
        default: "bluecontainer/kubectl-rundeck-nodes:latest"
      - name: exclude_operator
        type: Boolean
        title: Exclude Operator
        default: "true"
      - name: cluster_name
        type: String
        title: Cluster Name
        description: "Identifier for multi-cluster setups"
      - name: cluster_token_suffix
        type: String
        title: Cluster Token Suffix
        description: "Key Storage path suffix for this cluster"
      - name: default_token_suffix
        type: String
        title: Default Token Suffix
        description: "Default Key Storage path suffix when not using multi-cluster"
        default: "rundeck/k8s-token"
```

### nodes.sh (Standalone)

```bash
#!/bin/bash
set -e

K8S_TOKEN="${RD_CONFIG_K8S_TOKEN:-}"
K8S_URL="${RD_CONFIG_K8S_URL:-}"
NAMESPACE="${RD_CONFIG_NAMESPACE:-}"
LABEL_SELECTOR="${RD_CONFIG_LABEL_SELECTOR:-}"
EXECUTION_MODE="${RD_CONFIG_EXECUTION_MODE:-native}"
DOCKER_IMAGE="${RD_CONFIG_DOCKER_IMAGE:-bluecontainer/kubectl-rundeck-nodes:latest}"
EXCLUDE_OPERATOR="${RD_CONFIG_EXCLUDE_OPERATOR:-true}"
CLUSTER_NAME="${RD_CONFIG_CLUSTER_NAME:-}"
CLUSTER_TOKEN_SUFFIX="${RD_CONFIG_CLUSTER_TOKEN_SUFFIX:-}"
DEFAULT_TOKEN_SUFFIX="${RD_CONFIG_DEFAULT_TOKEN_SUFFIX:-rundeck/k8s-token}"

if [ -z "$K8S_TOKEN" ]; then
  echo "Error: K8S_TOKEN not provided" >&2
  exit 1
fi

if [ -z "$K8S_URL" ]; then
  echo "Error: K8S_URL not provided" >&2
  exit 1
fi

# Build flags
FLAGS=""
[ -n "$NAMESPACE" ] && FLAGS="$FLAGS -n $NAMESPACE" || FLAGS="$FLAGS -A"
[ -n "$LABEL_SELECTOR" ] && FLAGS="$FLAGS -l $LABEL_SELECTOR"
[ "$EXCLUDE_OPERATOR" = "true" ] && FLAGS="$FLAGS --exclude-operator"
[ -n "$CLUSTER_NAME" ] && FLAGS="$FLAGS --cluster-name=$CLUSTER_NAME"
[ -n "$K8S_URL" ] && FLAGS="$FLAGS --cluster-url=$K8S_URL"
[ -n "$CLUSTER_TOKEN_SUFFIX" ] && FLAGS="$FLAGS --cluster-token-suffix=$CLUSTER_TOKEN_SUFFIX"
[ -n "$DEFAULT_TOKEN_SUFFIX" ] && FLAGS="$FLAGS --default-token-suffix=$DEFAULT_TOKEN_SUFFIX"

case "$EXECUTION_MODE" in
  native)
    kubectl-rundeck-nodes --server="$K8S_URL" --token="$K8S_TOKEN" \
      --insecure-skip-tls-verify $FLAGS
    ;;
  docker)
    docker run --rm -i --network host \
      -e KUBERNETES_SERVICE_HOST="" \
      "$DOCKER_IMAGE" \
      --server="$K8S_URL" --token="$K8S_TOKEN" \
      --insecure-skip-tls-verify $FLAGS
    ;;
  *)
    echo "Error: Unknown execution mode: $EXECUTION_MODE" >&2
    exit 1
    ;;
esac
```

### Distribution Options

| Method | kubectl Plugin | Rundeck Plugin |
|--------|----------------|----------------|
| **Krew** | `kubectl krew install rundeck-nodes` | N/A |
| **GitHub Releases** | Binary downloads | ZIP downloads |
| **Docker Hub** | `bluecontainer/kubectl-rundeck-nodes` | N/A |
| **Rundeck Plugin Repo** | N/A | Upload ZIP |
| **Bundled ZIP** | N/A | Binary included in ZIP |

### Benefits of Standalone Rundeck Plugin

- Works with **any** Kubernetes cluster (no operator needed)
- Install once in Rundeck, use for multiple clusters via node sources
- Community can use it without generating an operator
- All enhancements (filtering, health checks, etc.) available to everyone
- Can be used alongside generated operator-specific plugins

### Repository Structure

Both plugins can live in the same repository:

```
github.com/bluecontainer/kubectl-rundeck-nodes/
├── cmd/kubectl-rundeck-nodes/
│   └── main.go                    # Standalone kubectl plugin CLI
├── pkg/nodes/
│   ├── discover.go                # Core discovery logic
│   ├── types.go                   # RundeckNode struct, Options
│   ├── output.go                  # JSON/table/yaml formatters
│   └── filters.go                 # Type/label/pattern filtering
├── rundeck-plugin/
│   ├── plugin.yaml                # Rundeck ResourceModelSource
│   ├── contents/
│   │   └── nodes.sh               # Script wrapper
│   └── Makefile                   # Builds ZIP
├── Dockerfile                     # Multi-arch image
├── Makefile                       # Build all artifacts
├── .goreleaser.yaml               # Release automation
└── go.mod
```

### Updated Implementation Phases

1. **Extract library** - Move discovery logic to `pkg/nodes/` with configurable options
2. **Create standalone kubectl plugin** - Wrap library in `cmd/kubectl-rundeck-nodes/`
3. **Create standalone Rundeck plugin** - Shell wrapper in `rundeck-plugin/`
4. **Docker image** - Multi-arch image for docker execution mode
5. **Publish kubectl plugin to Krew** - Submit to krew-index
6. **Publish Rundeck plugin** - GitHub releases, optionally Rundeck plugin repo
7. **Update generator** - Template imports library instead of embedding
8. **Regenerate examples** - Verify generated plugins still work
