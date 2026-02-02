# Plan: Managed CR Lifecycle Jobs for Rundeck

## Goal

Add a `--managed-crs=path/to/dir/` CLI flag that reads user-provided CR YAML files and generates per-CR lifecycle Rundeck jobs (apply, get, patch, delete, status) across all three execution modes. Use bundle and aggregate sample CRs as the example.

## Overview

The generator reads CR instance YAML files from a directory, parses each to extract kind/name/YAML, and generates 5 lifecycle jobs per CR in a `jobs/managed/` subdirectory within each Rundeck project.

### Lifecycle Operations

| Job | Command | Tool |
|-----|---------|------|
| **Apply** (create/update) | `kubectl apply -f -` with embedded CR YAML | native kubectl |
| **Get** (read) | `kubectl {plugin} describe {kind} {name}` | kubectl plugin |
| **Patch** (update) | `kubectl patch {kind} {name} --type=merge -p '...'` | native kubectl |
| **Delete** | `kubectl delete {kind} {name}` | native kubectl |
| **Status** (drift) | `kubectl {plugin} describe {kind} {name}` | kubectl plugin |

### Why `kubectl apply` for Apply?

Bundle/aggregate specs are complex (nested arrays, CEL expressions like `${resources.parent-pet.status.externalID}`). The plugin's `create --key=value` pattern can't handle these. `kubectl apply` natively handles any CR structure, is idempotent (creates or updates), and the embedded YAML is safe inside a quoted heredoc (`<<'CRYAML'` prevents shell variable expansion of CEL `${}` expressions).

### Why `kubectl patch` for Patch?

Native `kubectl patch --type=merge` works universally across all CR types. A JSON text option (`{"spec":{"paused":true}}`) is flexible for both simple field changes and complex nested updates, without needing to know the spec structure at generation time.

## Template Data Structure

```go
// RundeckManagedCRInfo holds data for a managed CR lifecycle job
type RundeckManagedCRInfo struct {
    RundeckTemplateData
    Kind      string // e.g., "PetstoreBundle"
    KindLower string // e.g., "petstorebundle"
    CRName    string // e.g., "simple-bundle" (from metadata.name)
    CRYaml    string // Full CR YAML, pre-indented for script heredoc embedding
}
```

## Files to Create

### 1. Managed CR job templates — 5 jobs × 3 execution modes = 15 files

**Script execution** (`pkg/templates/rundeck/`):
- `managed_apply_job.yaml.tmpl`
- `managed_get_job.yaml.tmpl`
- `managed_patch_job.yaml.tmpl`
- `managed_delete_job.yaml.tmpl`
- `managed_status_job.yaml.tmpl`

**Docker execution** (`pkg/templates/rundeck_docker/`):
- Same 5 files

**K8s pod execution** (`pkg/templates/rundeck_k8s/`):
- Same 5 files

### 2. Example managed CR files

`examples/managed-crs/simple-bundle.yaml`:
```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreBundle
metadata:
  name: simple-bundle
spec:
  resources:
    - id: pet-0
      kind: Pet
      spec:
        category: {"id": 1, "name": "Dogs"}
        name: "Fluffy"
        photoUrls: ["https://example.com/photos/fluffy.jpg"]
        status: "available"
    - id: order-1
      kind: Order
      spec:
        complete: false
        petId: 10
        quantity: 2
        status: "placed"
```

`examples/managed-crs/aggregate-overview.yaml`:
```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreAggregate
metadata:
  name: overview
spec:
  resourceSelectors:
    - kind: Order
    - kind: Pet
    - kind: User
  aggregationStrategy: AllHealthy
```

## Template Design

### Apply Job (script mode example)

```yaml
- name: apply-{{ .KindLower }}-{{ .CRName }}
  group: managed
  description: |-
    Apply managed {{ .Kind }} "{{ .CRName }}" to the cluster.
    Creates the resource if it doesn't exist, updates if it does.
  options:
  - name: namespace
    description: "Kubernetes namespace (overrides metadata.namespace)"
  - name: k8s_token
    secure: true
    ...
  sequence:
    commands:
    - script: |-
        set -e
        cat <<'CRYAML' > /tmp/managed-cr.yaml
{{ .CRYaml }}
        CRYAML
        NS="${RD_OPTION_NAMESPACE}"
        cmd="kubectl apply -f /tmp/managed-cr.yaml"
        [ -n "$NS" ] && cmd="$cmd -n $NS"
        cmd="$cmd --server=\"$RD_GLOBALS_K8S_URL\" --token=\"$RD_OPTION_K8S_TOKEN\" --insecure-skip-tls-verify"
        eval $cmd
```

The `.CRYaml` is pre-indented by 8 spaces in Go code to match the YAML block scalar indentation. The quoted heredoc (`<<'CRYAML'`) prevents shell expansion of `${}` CEL expressions.

### Docker mode difference

```bash
cat <<'CRYAML' > /tmp/managed-cr.yaml
{{ .CRYaml }}
CRYAML
cat /tmp/managed-cr.yaml | docker run -i --rm --network $NET \
    $IMAGE kubectl apply -f - ...
```

### K8s pod mode difference

```bash
cat <<'CRYAML' > /tmp/managed-cr.yaml
{{ .CRYaml }}
CRYAML
cat /tmp/managed-cr.yaml | kubectl --server=... --token=... \
    run $POD --image=$IMAGE --restart=Never --rm -i --quiet \
    -n $PLUGIN_NS --overrides='...' \
    -- kubectl apply -f -
```

The inner `kubectl apply` runs inside the pod with the plugin-runner SA (in-cluster auth). The outer `kubectl run` uses Rundeck's token credentials.

### Get Job

Uses the plugin's `describe` for rich formatted output:
```bash
cmd="kubectl {{ .PluginName }} describe {{ .KindLower }} {{ .CRName }}"
```

Options: namespace, output format (text/json/yaml), k8s_token.

### Patch Job

```bash
cmd="kubectl patch {{ .KindLower }} {{ .CRName }}"
cmd="$cmd --type=merge -p \"$RD_OPTION_PATCH_JSON\""
```

Options: `patch_json` (text type, e.g., `{"spec":{"paused":true}}`), namespace, k8s_token.

### Delete Job

```bash
cmd="kubectl delete {{ .KindLower }} {{ .CRName }}"
```

Options: namespace, k8s_token.

### Status Job

Same as Get but with different description/purpose — focused on status checking:
```bash
cmd="kubectl {{ .PluginName }} describe {{ .KindLower }} {{ .CRName }}"
```

Options: namespace, output format, k8s_token.

## Files to Modify

### 1. `internal/config/config.go`

Add field:
```go
ManagedCRsDir string // Directory containing CR YAML files for managed lifecycle jobs
```

### 2. `internal/config/file.go`

Add to `ConfigFile`:
```go
ManagedCRs string `yaml:"managedCRs,omitempty"` // Path to managed CR YAML directory
```

Add merge logic in `MergeConfigFile()`.

### 3. `cmd/openapi-operator-gen/main.go`

- Add `--managed-crs` flag: `generateCmd.Flags().StringVar(&cfg.ManagedCRsDir, "managed-crs", "", "Directory containing CR YAML files for managed Rundeck lifecycle jobs")`
- Pass `cfg.ManagedCRsDir` to `rundeckGen.GenerateManagedJobs()`
- Update Makefile example target: add `--managed-crs=../managed-crs`

### 4. `pkg/generator/rundeck_project.go`

Add:
- `RundeckManagedCRInfo` struct (see above)
- `parseManagedCRs(dir string) ([]RundeckManagedCRInfo, error)` — reads YAML files, splits multi-doc (`---`), extracts kind/name/full-yaml per document
- `indentString(s string, spaces int) string` — helper to indent CR YAML for embedding
- Add `ManagedApply/Get/Patch/Delete/Status` fields to `rundeckTemplateSet`
- Update `nativeTemplates()`, `dockerTemplates()`, `k8sTemplates()` with new template refs
- Update `generateProject()` to call managed CR generation after operations jobs — creates `jobs/managed/` dir, iterates over parsed CRs, generates 5 jobs each

### 5. `pkg/templates/templates.go`

Add 15 new `go:embed` variables (5 templates × 3 modes).

### 6. `Makefile`

Update `example` target to add `--managed-crs=../managed-crs` flag.

## Generated Output

```
{output}/
├── rundeck-project/
│   └── jobs/
│       ├── resources/         # existing CRUD jobs
│       ├── queries/           # existing query jobs
│       ├── actions/           # existing action jobs
│       ├── operations/        # existing ops jobs
│       └── managed/           # NEW: per-CR lifecycle jobs
│           ├── apply-petstorebundle-simple-bundle.yaml
│           ├── get-petstorebundle-simple-bundle.yaml
│           ├── patch-petstorebundle-simple-bundle.yaml
│           ├── delete-petstorebundle-simple-bundle.yaml
│           ├── status-petstorebundle-simple-bundle.yaml
│           ├── apply-petstoreaggregate-overview.yaml
│           ├── get-petstoreaggregate-overview.yaml
│           ├── patch-petstoreaggregate-overview.yaml
│           ├── delete-petstoreaggregate-overview.yaml
│           └── status-petstoreaggregate-overview.yaml
├── rundeck-docker-project/
│   └── jobs/managed/          # Same structure (Docker execution)
└── rundeck-k8s-project/
    └── jobs/managed/          # Same structure (K8s pod execution)
```

## CR YAML Parsing

The `parseManagedCRs()` function:
1. Globs `*.yaml` and `*.yml` from the directory
2. Reads each file and splits on `---` for multi-document YAML
3. For each document, does a minimal YAML parse to extract:
   - `kind` (e.g., "PetstoreBundle")
   - `metadata.name` (e.g., "simple-bundle")
4. Stores the raw YAML text (not the parsed structure) for embedding
5. Pre-indents the YAML for heredoc embedding in the script block
6. Returns `[]RundeckManagedCRInfo` slice

Skips documents that lack `kind` or `metadata.name`. Multi-doc files produce multiple CRs.

## Implementation Order

1. Add `ManagedCRsDir` to config structs and CLI flag
2. Create the 2 example managed CR files in `examples/managed-crs/`
3. Create 5 native script templates in `pkg/templates/rundeck/`
4. Create 5 Docker templates in `pkg/templates/rundeck_docker/`
5. Create 5 K8s templates in `pkg/templates/rundeck_k8s/`
6. Add 15 `go:embed` variables in `templates.go`
7. Add managed template fields to `rundeckTemplateSet` + update 3 factory functions
8. Add `RundeckManagedCRInfo`, `parseManagedCRs()`, `indentString()` to `rundeck_project.go`
9. Add managed CR generation to `generateProject()`
10. Wire up in `main.go` (flag + call)
11. Update `Makefile` example target
12. Build and regenerate: `make build && make example`

## Verification

```bash
make build && make example

# Verify managed jobs generated for all 3 project types
ls examples/generated/rundeck-project/jobs/managed/
ls examples/generated/rundeck-docker-project/jobs/managed/
ls examples/generated/rundeck-k8s-project/jobs/managed/

# Verify job content
cat examples/generated/rundeck-project/jobs/managed/apply-petstorebundle-simple-bundle.yaml
cat examples/generated/rundeck-project/jobs/managed/patch-petstoreaggregate-overview.yaml

# Verify CR YAML is properly embedded (CEL expressions not expanded)
grep '${resources' examples/generated/rundeck-project/jobs/managed/apply-petstorebundle-simple-bundle.yaml

# Run tests
make test

# Full stack test (Docker Compose imports managed jobs automatically via existing job import loop)
```
