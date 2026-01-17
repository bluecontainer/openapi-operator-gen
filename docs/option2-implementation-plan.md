# Option 2: Inline Composition CRD Implementation Plan

## Overview

Option 2 (Inline Composition with Embedded Specs) would allow users to create multiple REST API resources from a single Kubernetes CR, with support for dependencies between resources.

### Current vs Option 2

| Aspect | Current (Option 1+4) | Option 2 |
|--------|---------------------|----------|
| Purpose | Observe/aggregate existing resources | Create and manage resources |
| Resource creation | No (read-only aggregation) | Yes (full lifecycle) |
| Dependencies | N/A | CEL expressions between resources |
| Ownership | None | OwnerReferences to bundle CR |
| Deletion | No cascade | Cascade delete child resources |

### Example Option 2 CR

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreBundle
metadata:
  name: full-pet-setup
spec:
  # Embedded resource specs
  resources:
    - id: pet
      kind: Pet
      spec:
        name: "Fido"
        status: available
        photoUrls:
          - "https://example.com/fido.jpg"

    - id: order
      kind: Order
      spec:
        # CEL expression referencing pet's externalID
        petId: ${resources.pet.status.externalID}
        quantity: 1
      # Only create after pet is synced
      dependsOn: [pet]

  # Optional: explicit ordering via waves
  syncWaves:
    enabled: true

status:
  state: Synced  # Pending, Syncing, Synced, Failed
  message: "All resources synced"

  # Per-resource status
  resources:
    - id: pet
      kind: Pet
      name: full-pet-setup-pet  # Generated name
      state: Synced
      externalID: "123"
    - id: order
      kind: Order
      name: full-pet-setup-order
      state: Synced
      externalID: "456"

  # Execution state
  operationState:
    phase: Succeeded
    wave: 1
    startedAt: "2026-01-12T10:00:00Z"
    finishedAt: "2026-01-12T10:00:05Z"
```

---

## Required Changes

### 1. Mapper Changes (`pkg/mapper/resource.go`)

Add new types for bundle definitions:

```go
// BundleDefinition defines an inline composition CRD
type BundleDefinition struct {
    APIGroup   string
    APIVersion string
    Kind       string   // e.g., "PetstoreBundle"
    Plural     string   // e.g., "petstorebundles"

    // Available resource kinds that can be embedded
    ResourceKinds []string  // CRUD CRDs
    QueryKinds    []string  // Query CRDs
    ActionKinds   []string  // Action CRDs
    AllKinds      []string  // All CRDs combined
}

// BundleResourceSpec defines an embedded resource in a bundle
type BundleResourceSpec struct {
    ID        string            // Unique identifier within the bundle
    Kind      string            // Resource kind (e.g., "Pet", "Order")
    Spec      map[string]any    // The resource spec (with CEL expressions)
    DependsOn []string          // IDs of resources this depends on
    ReadyWhen []string          // CEL conditions for readiness
    SkipWhen  []string          // CEL conditions to skip creation
}

// CreateBundleDefinition creates a bundle CRD definition
func CreateBundleDefinition(cfg *config.Config, crds []*CRDDefinition) *BundleDefinition {
    bundle := &BundleDefinition{
        APIGroup:   cfg.APIGroup,
        APIVersion: cfg.APIVersion,
        Kind:       cfg.TitleAppName + "Bundle",
        Plural:     strings.ToLower(cfg.TitleAppName) + "bundles",
    }

    for _, crd := range crds {
        bundle.AllKinds = append(bundle.AllKinds, crd.Kind)
        if crd.IsQuery {
            bundle.QueryKinds = append(bundle.QueryKinds, crd.Kind)
        } else if crd.IsAction {
            bundle.ActionKinds = append(bundle.ActionKinds, crd.Kind)
        } else {
            bundle.ResourceKinds = append(bundle.ResourceKinds, crd.Kind)
        }
    }

    return bundle
}
```

### 2. New Types Template (`pkg/templates/bundle_types.go.tmpl`)

```go
package {{ .APIVersion }}

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
)

// BundleResourceSpec defines an embedded resource to create
type BundleResourceSpec struct {
    // ID is a unique identifier for this resource within the bundle
    // Used for referencing in CEL expressions: ${resources.<id>.status.externalID}
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Pattern=^[a-z][a-z0-9-]*$
    ID string `json:"id"`

    // Kind specifies the resource kind (e.g., "Pet", "Order")
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Enum={{ range $i, $k := .AllKinds }}{{ if $i }};{{ end }}{{ $k }}{{ end }}
    Kind string `json:"kind"`

    // Spec contains the resource specification
    // CEL expressions can reference other resources: ${resources.pet.status.externalID}
    // +kubebuilder:validation:Required
    // +kubebuilder:pruning:PreserveUnknownFields
    Spec runtime.RawExtension `json:"spec"`

    // DependsOn lists resource IDs that must be synced before this resource
    // +optional
    DependsOn []string `json:"dependsOn,omitempty"`

    // ReadyWhen defines CEL conditions that must be true for the resource to be considered ready
    // Example: "${pet.status.state == 'Synced'}"
    // +optional
    ReadyWhen []string `json:"readyWhen,omitempty"`

    // SkipWhen defines CEL conditions that, if true, skip creating this resource
    // Example: "${schema.spec.createOrder == false}"
    // +optional
    SkipWhen []string `json:"skipWhen,omitempty"`
}

// BundleResourceStatus represents the status of an embedded resource
type BundleResourceStatus struct {
    // ID is the resource identifier from spec
    ID string `json:"id"`

    // Kind is the resource kind
    Kind string `json:"kind"`

    // Name is the generated CR name
    Name string `json:"name"`

    // State is the current state
    // +kubebuilder:validation:Enum=Pending;Creating;Synced;Failed;Skipped
    State string `json:"state"`

    // ExternalID is the ID from the REST API
    // +optional
    ExternalID string `json:"externalID,omitempty"`

    // Message contains status details
    // +optional
    Message string `json:"message,omitempty"`

    // Ready indicates if readyWhen conditions are met
    Ready bool `json:"ready"`

    // Skipped indicates if skipWhen conditions were met
    Skipped bool `json:"skipped,omitempty"`
}

// OperationState tracks the bundle sync operation
type OperationState struct {
    // Phase is the current operation phase
    // +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed
    Phase string `json:"phase"`

    // Wave is the current sync wave being processed
    // +optional
    Wave int `json:"wave,omitempty"`

    // Message describes the current operation
    // +optional
    Message string `json:"message,omitempty"`

    // StartedAt is when the operation started
    // +optional
    StartedAt *metav1.Time `json:"startedAt,omitempty"`

    // FinishedAt is when the operation finished
    // +optional
    FinishedAt *metav1.Time `json:"finishedAt,omitempty"`

    // RetryCount tracks retry attempts
    // +optional
    RetryCount int `json:"retryCount,omitempty"`
}

// {{ .Kind }}Spec defines the desired state of {{ .Kind }}
type {{ .Kind }}Spec struct {
    // Resources defines the embedded resources to create
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinItems=1
    Resources []BundleResourceSpec `json:"resources"`

    // SyncWaves enables wave-based ordering (resources with dependsOn are auto-ordered)
    // +optional
    SyncWaves bool `json:"syncWaves,omitempty"`

    // Paused stops reconciliation when true
    // +optional
    Paused bool `json:"paused,omitempty"`

    // TargetHelmRelease specifies the Helm release for endpoint discovery (applied to all resources)
    // +optional
    TargetHelmRelease string `json:"targetHelmRelease,omitempty"`

    // TargetNamespace specifies the namespace for the target workload
    // +optional
    TargetNamespace string `json:"targetNamespace,omitempty"`
}

// {{ .Kind }}Status defines the observed state of {{ .Kind }}
type {{ .Kind }}Status struct {
    // State is the aggregated bundle state
    // +kubebuilder:validation:Enum=Pending;Syncing;Synced;Failed;Paused
    State string `json:"state,omitempty"`

    // Message describes the current state
    // +optional
    Message string `json:"message,omitempty"`

    // Resources contains per-resource status
    // +optional
    Resources []BundleResourceStatus `json:"resources,omitempty"`

    // OperationState tracks the current sync operation
    // +optional
    OperationState *OperationState `json:"operationState,omitempty"`

    // Summary contains resource counts
    // +optional
    Summary BundleSummary `json:"summary,omitempty"`

    // ObservedGeneration is the last observed spec generation
    // +optional
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`

    // Conditions represent the latest observations
    // +optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// BundleSummary contains resource counts
type BundleSummary struct {
    Total   int `json:"total"`
    Synced  int `json:"synced"`
    Failed  int `json:"failed"`
    Pending int `json:"pending"`
    Skipped int `json:"skipped"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=bundle
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Total",type=integer,JSONPath=`.status.summary.total`
// +kubebuilder:printcolumn:name="Synced",type=integer,JSONPath=`.status.summary.synced`
// +kubebuilder:printcolumn:name="Failed",type=integer,JSONPath=`.status.summary.failed`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// {{ .Kind }} creates and manages multiple resources as a single unit
type {{ .Kind }} struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   {{ .Kind }}Spec   `json:"spec,omitempty"`
    Status {{ .Kind }}Status `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// {{ .Kind }}List contains a list of {{ .Kind }}
type {{ .Kind }}List struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []{{ .Kind }} `json:"items"`
}

func init() {
    SchemeBuilder.Register(&{{ .Kind }}{}, &{{ .Kind }}List{})
}
```

### 3. New Controller Template (`pkg/templates/bundle_controller.go.tmpl`)

Key controller responsibilities:

```go
package controller

import (
    "context"
    "encoding/json"
    "fmt"
    "sort"
    "strings"
    "time"

    "github.com/google/cel-go/cel"
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/types"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
    "sigs.k8s.io/controller-runtime/pkg/log"

    api "{{ .ModuleName }}/api/{{ .APIVersion }}"
)

const bundleFinalizerName = "{{ .APIGroup }}/bundle-finalizer"

type {{ .Kind }}Reconciler struct {
    client.Client
    Scheme *runtime.Scheme
}

func (r *{{ .Kind }}Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)

    // Fetch the bundle
    var bundle api.{{ .Kind }}
    if err := r.Get(ctx, req.NamespacedName, &bundle); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // Handle deletion
    if !bundle.DeletionTimestamp.IsZero() {
        return r.handleDeletion(ctx, &bundle)
    }

    // Add finalizer
    if !controllerutil.ContainsFinalizer(&bundle, bundleFinalizerName) {
        controllerutil.AddFinalizer(&bundle, bundleFinalizerName)
        if err := r.Update(ctx, &bundle); err != nil {
            return ctrl.Result{}, err
        }
    }

    // Check if paused
    if bundle.Spec.Paused {
        return r.updateStatus(ctx, &bundle, "Paused", "Reconciliation paused")
    }

    // Build dependency graph and determine execution order
    order, err := r.buildExecutionOrder(bundle.Spec.Resources)
    if err != nil {
        return r.updateStatus(ctx, &bundle, "Failed", fmt.Sprintf("Invalid dependencies: %v", err))
    }

    // Process resources in order
    return r.syncResources(ctx, &bundle, order)
}

// buildExecutionOrder builds a DAG and returns topologically sorted resource IDs
func (r *{{ .Kind }}Reconciler) buildExecutionOrder(resources []api.BundleResourceSpec) ([]string, error) {
    // Build adjacency list
    graph := make(map[string][]string)
    inDegree := make(map[string]int)

    for _, res := range resources {
        graph[res.ID] = res.DependsOn
        if _, exists := inDegree[res.ID]; !exists {
            inDegree[res.ID] = 0
        }
        for _, dep := range res.DependsOn {
            inDegree[res.ID]++
        }
    }

    // Kahn's algorithm for topological sort
    var queue []string
    for id, degree := range inDegree {
        if degree == 0 {
            queue = append(queue, id)
        }
    }

    var order []string
    for len(queue) > 0 {
        id := queue[0]
        queue = queue[1:]
        order = append(order, id)

        // Find resources that depend on this one
        for _, res := range resources {
            for _, dep := range res.DependsOn {
                if dep == id {
                    inDegree[res.ID]--
                    if inDegree[res.ID] == 0 {
                        queue = append(queue, res.ID)
                    }
                }
            }
        }
    }

    if len(order) != len(resources) {
        return nil, fmt.Errorf("circular dependency detected")
    }

    return order, nil
}

// syncResources creates/updates resources in dependency order
func (r *{{ .Kind }}Reconciler) syncResources(ctx context.Context, bundle *api.{{ .Kind }}, order []string) (ctrl.Result, error) {
    log := log.FromContext(ctx)

    // Build resource map for quick lookup
    resourceMap := make(map[string]api.BundleResourceSpec)
    for _, res := range bundle.Spec.Resources {
        resourceMap[res.ID] = res
    }

    // Build status map from existing resources
    statusMap := make(map[string]*api.BundleResourceStatus)
    for i := range bundle.Status.Resources {
        statusMap[bundle.Status.Resources[i].ID] = &bundle.Status.Resources[i]
    }

    // Process in order
    var statuses []api.BundleResourceStatus
    allSynced := true
    hasFailed := false

    for _, id := range order {
        res := resourceMap[id]

        // Check skipWhen conditions
        if len(res.SkipWhen) > 0 {
            skip, err := r.evaluateConditions(res.SkipWhen, statusMap)
            if err != nil {
                log.Error(err, "Failed to evaluate skipWhen", "resource", id)
            }
            if skip {
                statuses = append(statuses, api.BundleResourceStatus{
                    ID:      id,
                    Kind:    res.Kind,
                    State:   "Skipped",
                    Skipped: true,
                })
                continue
            }
        }

        // Check if dependencies are ready
        depsReady := r.checkDependenciesReady(res.DependsOn, statusMap)
        if !depsReady {
            statuses = append(statuses, api.BundleResourceStatus{
                ID:      id,
                Kind:    res.Kind,
                State:   "Pending",
                Message: "Waiting for dependencies",
            })
            allSynced = false
            continue
        }

        // Resolve CEL expressions in spec
        resolvedSpec, err := r.resolveExpressions(res.Spec, statusMap)
        if err != nil {
            statuses = append(statuses, api.BundleResourceStatus{
                ID:      id,
                Kind:    res.Kind,
                State:   "Failed",
                Message: fmt.Sprintf("Expression error: %v", err),
            })
            hasFailed = true
            continue
        }

        // Create or update the child resource
        status, err := r.syncChildResource(ctx, bundle, id, res.Kind, resolvedSpec)
        if err != nil {
            statuses = append(statuses, api.BundleResourceStatus{
                ID:      id,
                Kind:    res.Kind,
                State:   "Failed",
                Message: err.Error(),
            })
            hasFailed = true
            continue
        }

        statuses = append(statuses, *status)
        if status.State != "Synced" {
            allSynced = false
        }
    }

    // Update bundle status
    bundle.Status.Resources = statuses
    bundle.Status.Summary = r.calculateSummary(statuses)

    if hasFailed {
        bundle.Status.State = "Failed"
        bundle.Status.Message = fmt.Sprintf("%d resources failed", bundle.Status.Summary.Failed)
    } else if allSynced {
        bundle.Status.State = "Synced"
        bundle.Status.Message = "All resources synced"
    } else {
        bundle.Status.State = "Syncing"
        bundle.Status.Message = fmt.Sprintf("%d/%d resources synced",
            bundle.Status.Summary.Synced, bundle.Status.Summary.Total)
    }

    bundle.Status.ObservedGeneration = bundle.Generation
    if err := r.Status().Update(ctx, bundle); err != nil {
        return ctrl.Result{}, err
    }

    // Requeue if not all synced
    if !allSynced {
        return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
    }

    return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// syncChildResource creates or updates a single child resource
func (r *{{ .Kind }}Reconciler) syncChildResource(
    ctx context.Context,
    bundle *api.{{ .Kind }},
    id string,
    kind string,
    spec []byte,
) (*api.BundleResourceStatus, error) {

    // Generate child resource name
    childName := fmt.Sprintf("%s-%s", bundle.Name, id)

    // Create the appropriate typed resource based on kind
    switch kind {
{{- range .ResourceKinds }}
    case "{{ . }}":
        return r.sync{{ . }}(ctx, bundle, childName, spec)
{{- end }}
{{- range .QueryKinds }}
    case "{{ . }}":
        return r.sync{{ . }}(ctx, bundle, childName, spec)
{{- end }}
{{- range .ActionKinds }}
    case "{{ . }}":
        return r.sync{{ . }}(ctx, bundle, childName, spec)
{{- end }}
    default:
        return nil, fmt.Errorf("unknown kind: %s", kind)
    }
}

// sync{{ .Kind }} creates/updates a {{ .Kind }} resource
{{- range .AllKinds }}
func (r *{{ $.Kind }}Reconciler) sync{{ . }}(
    ctx context.Context,
    bundle *api.{{ $.Kind }},
    name string,
    specData []byte,
) (*api.BundleResourceStatus, error) {

    var child api.{{ . }}
    child.Name = name
    child.Namespace = bundle.Namespace

    // Parse spec
    if err := json.Unmarshal(specData, &child.Spec); err != nil {
        return nil, fmt.Errorf("invalid spec for {{ . }}: %w", err)
    }

    // Set owner reference for garbage collection
    if err := controllerutil.SetControllerReference(bundle, &child, r.Scheme); err != nil {
        return nil, err
    }

    // Create or update
    existing := &api.{{ . }}{}
    err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: bundle.Namespace}, existing)

    if errors.IsNotFound(err) {
        // Create
        if err := r.Create(ctx, &child); err != nil {
            return nil, err
        }
        return &api.BundleResourceStatus{
            ID:    strings.TrimPrefix(name, bundle.Name+"-"),
            Kind:  "{{ . }}",
            Name:  name,
            State: "Pending",
        }, nil
    } else if err != nil {
        return nil, err
    }

    // Update if spec changed
    if !reflect.DeepEqual(existing.Spec, child.Spec) {
        existing.Spec = child.Spec
        if err := r.Update(ctx, existing); err != nil {
            return nil, err
        }
    }

    // Return current status
    return &api.BundleResourceStatus{
        ID:         strings.TrimPrefix(name, bundle.Name+"-"),
        Kind:       "{{ . }}",
        Name:       name,
        State:      existing.Status.State,
        ExternalID: existing.Status.ExternalID,
        Message:    existing.Status.Message,
        Ready:      existing.Status.State == "Synced",
    }, nil
}
{{- end }}

// handleDeletion handles bundle deletion by cleaning up child resources
func (r *{{ .Kind }}Reconciler) handleDeletion(ctx context.Context, bundle *api.{{ .Kind }}) (ctrl.Result, error) {
    if !controllerutil.ContainsFinalizer(bundle, bundleFinalizerName) {
        return ctrl.Result{}, nil
    }

    // Child resources are garbage collected via ownerReferences
    // Just remove the finalizer
    controllerutil.RemoveFinalizer(bundle, bundleFinalizerName)
    if err := r.Update(ctx, bundle); err != nil {
        return ctrl.Result{}, err
    }

    return ctrl.Result{}, nil
}

// resolveExpressions replaces CEL expressions in spec with resolved values
func (r *{{ .Kind }}Reconciler) resolveExpressions(
    spec runtime.RawExtension,
    statusMap map[string]*api.BundleResourceStatus,
) ([]byte, error) {
    // Parse spec as JSON
    var data map[string]interface{}
    if err := json.Unmarshal(spec.Raw, &data); err != nil {
        return nil, err
    }

    // Build CEL environment with resource variables
    env, err := cel.NewEnv(
        cel.Variable("resources", cel.MapType(cel.StringType, cel.DynType)),
    )
    if err != nil {
        return nil, err
    }

    // Build resources map for CEL
    resourcesVar := make(map[string]interface{})
    for id, status := range statusMap {
        resourcesVar[id] = map[string]interface{}{
            "status": map[string]interface{}{
                "state":      status.State,
                "externalID": status.ExternalID,
                "message":    status.Message,
            },
        }
    }

    // Recursively resolve expressions
    resolved, err := r.resolveMap(data, env, map[string]interface{}{
        "resources": resourcesVar,
    })
    if err != nil {
        return nil, err
    }

    return json.Marshal(resolved)
}

// resolveMap recursively resolves CEL expressions in a map
func (r *{{ .Kind }}Reconciler) resolveMap(
    data map[string]interface{},
    env *cel.Env,
    vars map[string]interface{},
) (map[string]interface{}, error) {
    result := make(map[string]interface{})

    for key, value := range data {
        switch v := value.(type) {
        case string:
            // Check if it's a CEL expression: ${...}
            if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
                expr := v[2 : len(v)-1]
                resolved, err := r.evaluateCEL(env, expr, vars)
                if err != nil {
                    return nil, fmt.Errorf("failed to evaluate %s: %w", key, err)
                }
                result[key] = resolved
            } else {
                result[key] = v
            }
        case map[string]interface{}:
            resolved, err := r.resolveMap(v, env, vars)
            if err != nil {
                return nil, err
            }
            result[key] = resolved
        case []interface{}:
            resolved, err := r.resolveSlice(v, env, vars)
            if err != nil {
                return nil, err
            }
            result[key] = resolved
        default:
            result[key] = v
        }
    }

    return result, nil
}

func (r *{{ .Kind }}Reconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&api.{{ .Kind }}{}).
        // Watch owned resources
{{- range .AllKinds }}
        Owns(&api.{{ . }}{}).
{{- end }}
        Complete(r)
}
```

### 4. Generator Changes (`pkg/generator/controller.go`)

Add new generator function:

```go
// GenerateBundleController generates the bundle controller
func (g *ControllerGenerator) GenerateBundleController(bundle *mapper.BundleDefinition) error {
    data := BundleControllerTemplateData{
        Year:             time.Now().Year(),
        GeneratorVersion: g.config.GeneratorVersion,
        APIGroup:         bundle.APIGroup,
        APIVersion:       bundle.APIVersion,
        ModuleName:       g.config.ModuleName,
        Kind:             bundle.Kind,
        KindLower:        strings.ToLower(bundle.Kind),
        Plural:           bundle.Plural,
        ResourceKinds:    bundle.ResourceKinds,
        QueryKinds:       bundle.QueryKinds,
        ActionKinds:      bundle.ActionKinds,
        AllKinds:         bundle.AllKinds,
    }

    tmpl, err := template.New("bundle-controller").Parse(templates.BundleControllerTemplate)
    if err != nil {
        return fmt.Errorf("failed to parse bundle controller template: %w", err)
    }

    controllerPath := filepath.Join(g.config.OutputDir, "internal", "controller",
        fmt.Sprintf("%s_controller.go", strings.ToLower(bundle.Kind)))

    file, err := os.Create(controllerPath)
    if err != nil {
        return fmt.Errorf("failed to create file: %w", err)
    }
    defer file.Close()

    if err := tmpl.Execute(file, data); err != nil {
        return fmt.Errorf("failed to execute template: %w", err)
    }

    return nil
}
```

### 5. CLI Flag Changes (`internal/config/config.go`)

Add new flag:

```go
type Config struct {
    // ... existing fields ...

    // EnableBundle enables Option 2 bundle CRD generation
    EnableBundle bool
}

// In CLI setup
cmd.Flags().BoolVar(&cfg.EnableBundle, "bundle", false,
    "Generate bundle CRD for inline resource composition (Option 2)")
```

### 6. Main Generator Changes

Update main generation flow:

```go
func (g *Generator) Generate() error {
    // ... existing generation ...

    // Generate aggregate CRD if --aggregate flag
    if g.config.EnableAggregate {
        aggregate := mapper.CreateAggregateDefinition(g.config, crds)
        if err := g.generateAggregateCRD(aggregate); err != nil {
            return err
        }
    }

    // Generate bundle CRD if --bundle flag
    if g.config.EnableBundle {
        bundle := mapper.CreateBundleDefinition(g.config, crds)
        if err := g.generateBundleCRD(bundle); err != nil {
            return err
        }
    }

    return nil
}
```

---

## Key Implementation Considerations

### 1. CEL Expression Resolution

Support expressions like `${resources.pet.status.externalID}`:

```go
// Expression patterns to support:
// ${resources.<id>.status.externalID}  - Reference another resource's external ID
// ${resources.<id>.status.state}       - Reference another resource's state
// ${schema.spec.<field>}               - Reference bundle spec fields (Kro-style)
```

### 2. Dependency Graph Validation

Prevent circular dependencies:

```go
func validateDependencies(resources []BundleResourceSpec) error {
    // Build graph
    // Detect cycles using DFS
    // Return error if cycle found
}
```

### 3. Owner References

Ensure proper garbage collection:

```go
// Set owner reference on child resources
controllerutil.SetControllerReference(bundle, child, r.Scheme)
```

### 4. Partial Failure Handling

Handle cases where some resources fail:

- Continue with independent resources
- Track per-resource errors
- Support retry policies

### 5. Update Handling

When bundle spec changes:

- Detect which embedded specs changed
- Update only affected child resources
- Handle dependency order for updates

---

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `pkg/mapper/resource.go` | Modify | Add `BundleDefinition` type and constructor |
| `pkg/templates/bundle_types.go.tmpl` | Create | Bundle CRD types template |
| `pkg/templates/bundle_controller.go.tmpl` | Create | Bundle controller template |
| `pkg/templates/templates.go` | Modify | Add new template exports |
| `pkg/generator/controller.go` | Modify | Add bundle generation functions |
| `pkg/generator/types.go` | Modify | Add bundle types generation |
| `internal/config/config.go` | Modify | Add `--bundle` flag |
| `cmd/openapi-operator-gen/main.go` | Modify | Wire up bundle generation |
| `pkg/templates/main.go.tmpl` | Modify | Register bundle controller |
| `pkg/generator/samples.go` | Modify | Generate bundle example CRs |
| `pkg/templates/example_bundle_cr.yaml.tmpl` | Create | Example bundle CR template |

---

## Estimated Complexity

| Component | Complexity | Effort |
|-----------|------------|--------|
| Types template | Medium | 1-2 days |
| Controller template | High | 3-5 days |
| CEL expression resolution | High | 2-3 days |
| Dependency graph handling | Medium | 1-2 days |
| Generator integration | Low | 1 day |
| Testing | High | 3-5 days |
| **Total** | | **11-18 days** |

---

## Testing Strategy

1. **Unit Tests**
   - Dependency graph building and cycle detection
   - CEL expression resolution
   - Spec merging logic

2. **Integration Tests**
   - Bundle creation with multiple resources
   - Dependency ordering
   - Update propagation
   - Cascade deletion

3. **E2E Tests**
   - Full workflow with real REST API
   - Failure and recovery scenarios

---

## Alternative: Use Kro

Instead of implementing Option 2 from scratch, consider integrating with [Kro](https://kro.run):

**Pros:**
- Battle-tested DAG resolution
- CEL already implemented
- Dynamic CRD generation
- Active community (SIG Cloud Provider)

**Cons:**
- Additional dependency
- May not fit REST API sync model exactly
- Less control over implementation

The recommended approach is to implement a simplified version of Option 2 that:
1. Supports basic dependencies via `dependsOn`
2. Uses CEL for cross-resource references
3. Creates child CRs with ownerReferences
4. Aggregates status from children

This provides the core value without the full complexity of Kro's dynamic CRD generation.
