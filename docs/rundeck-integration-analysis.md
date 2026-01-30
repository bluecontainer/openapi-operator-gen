# Rundeck Integration Analysis

Analysis of how to integrate with Rundeck and options for generating a Rundeck project alongside the generated operator.

## What is Rundeck?

Rundeck is an open-source runbook automation platform for executing operational tasks across infrastructure. Its core concepts — **projects** (top-level organizational unit), **jobs** (parameterized workflow sequences), and **nodes** (execution targets) — map well to what openapi-operator-gen already understands about REST APIs.

## Integration Options

There are two distinct integration paths worth considering:

---

## Option A: Generate a Rundeck Project from the OpenAPI Spec

Add a `--rundeck-project` flag to the generator that produces a Rundeck project alongside the operator. This would generate job definitions for each REST API operation (CRUD, queries, actions) that users can import into Rundeck for operational workflows.

### What gets generated

```
output/
└── rundeck-project/
    ├── project.yaml              # Project configuration
    ├── jobs/
    │   ├── pet/
    │   │   ├── create-pet.yaml       # POST /pet
    │   │   ├── get-pet.yaml          # GET /pet/{petId}
    │   │   ├── update-pet.yaml       # PUT /pet
    │   │   ├── delete-pet.yaml       # DELETE /pet/{petId}
    │   │   └── find-by-status.yaml   # GET /pet/findByStatus
    │   ├── store/
    │   │   ├── create-order.yaml
    │   │   └── get-inventory.yaml
    │   └── user/
    │       ├── create-user.yaml
    │       └── login.yaml
    └── acl/
        └── default.aclpolicy    # RBAC policies
```

### How jobs map to the OpenAPI spec

| OpenAPI Classification | Rundeck Job Type | Example |
|---|---|---|
| Resource (CRUD) | One job per HTTP method, grouped by resource | `pet/create-pet.yaml` — POST /pet with spec fields as job options |
| QueryEndpoint (GET-only) | Read-only job with query params as options | `pet/find-by-status.yaml` — GET /pet/findByStatus with `--status` option |
| ActionEndpoint (POST/PUT-only) | One-shot job | `pet/upload-image.yaml` — POST /pet/{petId}/uploadImage |

### Example generated job definition

For `GET /pet/{petId}`:

```yaml
- name: get-pet
  group: pet
  description: |
    Retrieve a Pet resource from the REST API by ID.
    Generated from: GET /pet/{petId}
  loglevel: INFO
  executionEnabled: true
  options:
    - name: petId
      description: ID of the pet to retrieve
      required: true
    - name: base_url
      description: REST API base URL
      required: true
      value: "${globals.api_base_url}"
    - name: output_format
      description: Output format
      enforced: true
      values: [json, yaml, table]
      value: json
  sequence:
    commands:
      - script: |
          #!/bin/bash
          set -e
          RESPONSE=$(curl -sf "${option.base_url}/pet/${option.petId}" \
            -H "Accept: application/json")
          echo "$RESPONSE" | jq .
```

### Value proposition

Operations teams get ready-to-import Rundeck jobs for every API operation without writing them by hand. Jobs include proper parameterization, grouping by resource, and can be used for:
- Manual operational tasks (ad-hoc API calls with an audit trail)
- Scheduled health checks (query endpoints as periodic jobs)
- Incident response runbooks (pre-built jobs for common operations)
- Change management workflows (jobs with approval gates)

---

## Option B: Generate a Kubernetes Operator from Rundeck's API Spec

Rundeck itself has a REST API with an existing [Swagger 2.0 spec](https://github.com/rundeck/rundeck-api-specs). You could feed that spec into openapi-operator-gen to produce a Kubernetes operator that manages Rundeck declaratively via CRDs.

### What this produces

| Rundeck Concept | CRD Classification | CRD Kind |
|---|---|---|
| Project (CRUD) | Resource | `RundeckProject` |
| Job definition (CRUD) | Resource | `RundeckJob` |
| Job execution (POST /job/{id}/run) | ActionEndpoint | `RundeckJobExecution` |
| Execution status (GET /execution/{id}) | QueryEndpoint | `RundeckExecutionStatus` |
| Running executions query | QueryEndpoint | `RundeckRunningExecutions` |
| SCM sync action | ActionEndpoint | `RundeckSCMSync` |

### Value proposition

Manage Rundeck itself via Kubernetes GitOps — define projects, jobs, and even trigger executions through Kubernetes CRs. This is more of an "operator for Rundeck" rather than "Rundeck alongside the operator."

This option doesn't require any code changes to openapi-operator-gen — just running the existing tool against Rundeck's API spec.

---

## Option C: Both — Bidirectional Integration

Generate the operator as usual, plus generate Rundeck jobs that interact with the operator's CRDs via `kubectl`. Instead of calling the REST API directly, the Rundeck jobs create/modify Kubernetes CRs:

```yaml
- name: create-pet
  group: pet
  description: Create a Pet CR in Kubernetes
  options:
    - name: name
      required: true
    - name: status
      enforced: true
      values: [available, pending, sold]
  sequence:
    commands:
      - script: |
          #!/bin/bash
          kubectl apply -f - <<EOF
          apiVersion: petstore.example.com/v1alpha1
          kind: Pet
          metadata:
            name: ${option.cr_name:-pet-$(date +%s)}
          spec:
            name: "${option.name}"
            status: "${option.status}"
          EOF
```

This leverages the operator's reconciliation logic while providing a Rundeck UI for teams that prefer it over raw kubectl.

---

## Implementation Effort (Option A)

Following the kubectl plugin as a precedent:

| Component | What to build |
|---|---|
| CLI flag | `--rundeck-project` in `main.go` |
| Config | `GenerateRundeckProject bool` in `config.go` |
| Generator | `pkg/generator/rundeck_project.go` — new file following `kubectl_plugin.go` pattern |
| Templates | `pkg/templates/rundeck/` — project.yaml, job.yaml, acl.aclpolicy templates |
| Embed | Add directives in `templates.go` |
| Wiring | Gate on flag in `main.go`, call generator |

The data needed is already available — CRD definitions contain the paths, methods, parameters, and field types. The mapper's classification (Resource/Query/Action) maps directly to Rundeck job categories.

### Generator architecture

Following the kubectl plugin pattern in `pkg/generator/kubectl_plugin.go`:

```go
type RundeckProjectGenerator struct {
    config *config.Config
}

type RundeckProjectData struct {
    Year             int
    GeneratorVersion string
    APIGroup         string
    ProjectName      string
    APIBaseURL       string
    ResourceKinds    []RundeckJobInfo
    QueryKinds       []RundeckJobInfo
    ActionKinds      []RundeckJobInfo
}

type RundeckJobInfo struct {
    Kind        string
    Path        string
    Method      string
    Description string
    Parameters  []RundeckParameter
}

func (g *RundeckProjectGenerator) Generate(crds []*mapper.CRDDefinition,
    aggregate *mapper.AggregateDefinition,
    bundle *mapper.BundleDefinition) error {

    projectDir := filepath.Join(g.config.OutputDir, "rundeck-project")
    data := g.prepareTemplateData(crds, aggregate, bundle)

    // Generate project.yaml, job files, ACL policies
    // ...
}
```

### Data flow

```
OpenAPI Spec
    ↓
Parser (extract paths, methods, parameters)
    ↓
Mapper (classify as Resource/Query/Action, create CRDDefinitions)
    ↓
RundeckProjectGenerator.prepareTemplateData()
    ├─ Extract API group → project name
    ├─ Categorize CRDs → resource/query/action jobs
    └─ Map operations → job definitions with parameters
    ↓
Template Rendering
    ├─ project.yaml (from RundeckProjectData)
    ├─ job_*.yaml (from RundeckJobInfo per operation)
    └─ acl/*.aclpolicy (from ACL rules)
    ↓
Output Files in ./rundeck-project/
```

---

## Rundeck Background

### Rundeck job definition format (YAML)

Jobs can be defined in YAML, XML, or JSON. YAML is the most common format for jobs-as-code workflows:

```yaml
- name: string                    # Job name
  description: string             # Markdown description
  group: string                   # Folder-like grouping (e.g., "deploy/web")
  loglevel: INFO                  # DEBUG|VERBOSE|INFO|WARN|ERROR
  multipleExecutions: false       # Allow concurrent executions
  timeout: "30m"                  # Max execution time
  retry: 3                        # Retries on failure
  executionEnabled: true
  scheduleEnabled: true

  # Job parameters
  options:
    - name: environment
      description: Target environment
      required: true
      enforced: true              # Must be from values list
      values: [dev, staging, production]
      value: dev                  # Default value
    - name: api_key
      secure: true                # Masked in UI/logs
      storagePath: "keys/api_key" # From key storage

  # Node targeting
  nodefilters:
    dispatch:
      threadcount: 4
      keepgoing: true
    filter: "tags:webserver"

  # Cron scheduling
  schedule:
    crontab: "0 0 6 ? * MON-FRI *"

  # Workflow steps
  sequence:
    keepgoing: false
    strategy: node-first
    commands:
      - exec: echo "Hello"                        # Simple command
      - script: |                                  # Inline script
          #!/bin/bash
          set -e
          ./deploy.sh ${option.version}
      - scripturl: https://example.com/script.sh   # Script from URL
      - jobref:                                     # Call another job
          name: healthcheck
          group: monitoring
      - type: Kubernetes-Create                     # Plugin step
        configuration:
          yaml: |
            apiVersion: apps/v1
            kind: Deployment
            ...

  # Notifications
  notification:
    onsuccess:
      - email:
          recipients: "team@example.com"
    onfailure:
      - urls: "https://hooks.slack.com/services/xxx"
```

### Rundeck + Kubernetes

The [official Kubernetes plugin](https://github.com/rundeck-plugins/kubernetes) provides:
- **Node Source**: Discovers pods as Rundeck nodes (with namespace/label filtering)
- **Node Executor**: Dispatches commands to pods via `kubectl exec`
- **Workflow Steps**: Create/update/delete Deployments, Services, StatefulSets, ConfigMaps, etc.

### Rundeck SCM / Jobs-as-Code

Rundeck has built-in Git SCM integration for a jobs-as-code workflow:
- **SCM Export**: Job changes in Rundeck are committed/pushed to Git
- **SCM Import**: Job definitions pulled from Git into Rundeck
- Supports file path templates: `${job.group}/${job.name}.yaml`

Typical GitOps flow: Dev Rundeck exports to Git branch → code review → Prod Rundeck imports from main branch.

### Rundeck REST API

Rundeck has a comprehensive REST API ([docs](https://docs.rundeck.com/docs/api/)) with an [OpenAPI spec](https://github.com/rundeck/rundeck-api-specs). Key endpoints:

| Endpoint | Description |
|---|---|
| `POST /api/14/job/{id}/run` | Execute a job |
| `GET /api/14/execution/{id}` | Get execution status |
| `GET /api/14/project/{project}/jobs/export` | Export jobs (YAML/XML/JSON) |
| `POST /api/14/project/{project}/jobs/import` | Import job definitions |
| `GET /api/11/project/{project}/export` | Export project archive (ZIP) |

---

## Option C Deep Dive: Execution Mechanism Comparison

Option C generates Rundeck jobs that interact with the operator's CRDs via Kubernetes rather than calling the REST API directly. This leverages the operator's reconciliation logic (drift detection, finalizer cleanup, status reporting) while providing a Rundeck UI for operations teams.

Within Option C there are three ways the generated Rundeck jobs can interact with Kubernetes:

### C1: kubectl with Raw CRs

Rundeck jobs use `kubectl apply -f -` with inline YAML heredocs:

```yaml
- name: create-pet
  group: pet
  description: Create a Pet resource via the Kubernetes operator
  options:
    - name: name
      required: true
    - name: status
      enforced: true
      values: [available, pending, sold]
    - name: cr_name
      description: Name for the Kubernetes CR
  sequence:
    commands:
      - script: |
          #!/bin/bash
          set -e
          CR_NAME="${option.cr_name:-pet-$(date +%s)}"
          kubectl apply -f - <<EOF
          apiVersion: petstore.example.com/v1alpha1
          kind: Pet
          metadata:
            name: $CR_NAME
          spec:
            name: "${option.name}"
            status: "${option.status}"
          EOF
          echo "Waiting for reconciliation..."
          kubectl wait --for=condition=Ready pet/$CR_NAME --timeout=60s
```

**Pros:**
- Only dependency is `kubectl` with a valid kubeconfig — universally available
- Full control over every CR field, including metadata annotations, labels, `target` sub-object
- Can express any CR shape the operator supports (adopt mode, read-only, paused, TTL patches)
- Template generation is straightforward — embed the CRD schema as YAML with `${option.*}` substitution

**Cons:**
- Verbose — each job contains a full YAML manifest with string interpolation, which is fragile
- No schema validation at job execution time — typos in field names silently produce invalid CRs
- No built-in wait-for-ready — needs a separate polling step (`kubectl wait --for=condition=Ready`)
- Nested objects and arrays require JSON-in-YAML escaping (e.g., `category: '{"id":1,"name":"Dogs"}'`), which is error-prone for Rundeck users filling in options
- Every CRUD operation needs its own boilerplate (apply for create/update, delete, get + jq for read)

### C2: kubectl with the Generated Plugin

Rundeck jobs call the generated kubectl plugin commands:

```yaml
- name: create-pet
  group: pet
  description: Create a Pet resource via the Kubernetes operator
  options:
    - name: name
      required: true
    - name: status
      enforced: true
      values: [available, pending, sold]
  sequence:
    commands:
      - exec: >-
          kubectl petstore create pet
          --name="${option.name}"
          --status="${option.status}"
```

```yaml
- name: query-pets-by-status
  group: pet
  description: Find pets by status
  options:
    - name: status
      enforced: true
      values: [available, pending, sold]
  sequence:
    commands:
      - exec: >-
          kubectl petstore query petfindbystatusquery
          --status="${option.status}" -q
```

```yaml
- name: describe-pet
  group: pet
  description: Get detailed information about a pet
  options:
    - name: pet_name
      required: true
  sequence:
    commands:
      - exec: kubectl petstore describe pet "${option.pet_name}" --show-response
```

**Pros:**
- Concise — one-liner per operation, no YAML boilerplate
- Built-in `--wait` and `--timeout` on create/query/action commands — no separate polling step
- `--dry-run` support lets Rundeck users preview what would be created
- Dynamic parameter parsing handles type coercion (strings, ints, booleans, JSON objects)
- Endpoint targeting flags (`--target-helm-release`, `--target-statefulset`, etc.) available directly
- Rundeck job options map 1:1 to plugin flags — clean parameter passing
- `-q` (quiet) mode on queries outputs raw JSON, good for piping to subsequent steps
- The plugin validates resource types (`kubectl petstore create types` lists valid kinds)
- Error messages include hints about JSON syntax for complex fields

**Cons:**
- Requires the kubectl plugin binary installed on the Rundeck server or execution node — adds a build/deploy step
- Plugin binary is architecture-specific (must match the Rundeck server's OS/arch)
- Plugin must be on `$PATH` as `kubectl-petstore` for kubectl to discover it
- Plugin is generated per-operator, so each API gets its own binary — multiple APIs means multiple binaries
- Slightly less flexible than raw CRs for advanced use cases (e.g., setting arbitrary annotations)

### C3: Rundeck Kubernetes Plugin (Workflow Steps)

Rundeck jobs use the native [Kubernetes plugin](https://github.com/rundeck-plugins/kubernetes) workflow step types:

```yaml
- name: create-pet
  group: pet
  description: Create a Pet resource via the Kubernetes operator
  options:
    - name: name
      required: true
    - name: status
      enforced: true
      values: [available, pending, sold]
  sequence:
    commands:
      - nodeStep: false
        type: Kubernetes-Create
        configuration:
          yaml: |
            apiVersion: petstore.example.com/v1alpha1
            kind: Pet
            metadata:
              name: pet-${option.name}
            spec:
              name: "${option.name}"
              status: "${option.status}"
          namespace: petstore-system
          verify_ssl: "true"
```

**Pros:**
- Native Rundeck integration — shows as a proper workflow step in the UI, not a script block
- No kubectl binary needed — the plugin uses the Python Kubernetes SDK directly
- Kubernetes auth configured at the project level (kubeconfig or in-cluster), not per-job
- Visual editing in Rundeck's workflow designer

**Cons:**
- The K8s plugin's typed workflow steps (Kubernetes-Create, Kubernetes-Delete, etc.) are designed for core resources (Deployments, Services, Jobs) — CRDs require the generic create step with raw YAML, which is essentially the same as C1 but in a plugin configuration field instead of a shell heredoc
- No wait-for-condition support — the plugin creates the resource but doesn't poll for `Ready` status
- No CRD-aware validation — the plugin doesn't know the operator's schema
- Requires Python Kubernetes SDK (`pip install kubernetes` v11+) on the Rundeck server
- Multi-cluster support is limited — project-level K8s config, not per-job
- No equivalent to the kubectl plugin's `--dry-run`, `-q`, or `--wait` features
- The plugin hasn't seen active development recently

### Comparison

| Aspect | C1: Raw kubectl | C2: Generated plugin | C3: Rundeck K8s plugin |
|---|---|---|---|
| **Dependencies** | kubectl only | kubectl + plugin binary | Python K8s SDK |
| **Verbosity** | High (full YAML per job) | Low (one-liner flags) | High (YAML in config field) |
| **Wait for ready** | Manual polling step | Built-in `--wait` | Not supported |
| **Schema validation** | None | Type coercion + hints | None |
| **Dry run** | No | Yes (`--dry-run`) | No |
| **Endpoint targeting** | Manual (in CR YAML) | Flags (`--target-*`) | Not applicable |
| **Rundeck UI integration** | Script block | Script block | Native workflow step |
| **Template complexity** | Medium | Low | Medium |
| **Installation burden** | None | Build + install binary | pip install |
| **Flexibility** | Full CR control | Covers common cases | Full CR control |

---

## Recommendation

**Option C with C2 (generated plugin) is the recommended approach.** It utilizes the operator that is also generated, and the plugin produces the most concise jobs with built-in wait/dry-run/quiet support. The flag-based interface maps cleanly to Rundeck job options. The main cost is requiring the plugin binary on the Rundeck server, which is a one-time setup.

C1 (raw kubectl) is the fallback for cases where installing the plugin isn't feasible or where full CR control is needed (annotations, advanced targeting). A practical approach would be to default to C2 and fall back to C1 for edge cases.

C3 (Rundeck K8s plugin) doesn't add meaningful value over C1 for CRD operations — it trades shell scripting for a plugin configuration field, but loses the kubectl plugin's schema awareness and wait semantics.

Option A (direct REST API calls) remains useful as a separate mode for teams that don't have the operator deployed but want Rundeck jobs for the API. This could be exposed as `--rundeck-mode=api` vs `--rundeck-mode=kubectl` (default).
