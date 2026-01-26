# Options: Native Kubernetes Resource References in Bundle and Aggregate CRDs

## Overview

This document explores options for extending Bundle and StatusAggregate CRDs to reference standard Kubernetes resources (Deployments, StatefulSets, Services, ConfigMaps, etc.) for health/status determination.

### Current Limitations

Both Bundle and StatusAggregate currently only support resources within the generated operator's API group:

```yaml
# Current: Only works with generated CRDs
apiVersion: petstore.example.com/v1alpha1
kind: StatusAggregate
spec:
  selectors:
    - matchKind: Pet        # ✓ Works - same API group
    - matchKind: Deployment # ✗ Fails - different API group
```

### Use Cases

1. **Application Health Aggregation**: Include Deployment/StatefulSet readiness in overall health
2. **Dependency Tracking**: Bundle waits for ConfigMap/Secret before creating API resources
3. **Infrastructure Integration**: Aggregate health of Services, Ingresses, PVCs
4. **Cross-Operator Composition**: Reference CRDs from other operators (Prometheus, Cert-Manager)

---

## Option 1: Extended Selector with API Group

**Approach**: Add `apiGroup` and `apiVersion` fields to selectors for explicit cross-group references.

### Type Changes

```go
// Aggregate selector extension
type ResourceSelector struct {
    // Existing fields
    Kind        string            `json:"kind"`
    NamePattern string            `json:"namePattern,omitempty"`
    MatchLabels map[string]string `json:"matchLabels,omitempty"`

    // New fields for native K8s resources
    // +kubebuilder:validation:Optional
    APIGroup string `json:"apiGroup,omitempty"`  // e.g., "apps", "v1", ""
    // +kubebuilder:validation:Optional
    APIVersion string `json:"apiVersion,omitempty"` // e.g., "v1", "apps/v1"
}

// Explicit reference extension
type ResourceReference struct {
    Kind      string `json:"kind"`
    Name      string `json:"name"`
    Namespace string `json:"namespace,omitempty"`

    // New field
    // +kubebuilder:validation:Optional
    APIGroup string `json:"apiGroup,omitempty"`
}
```

### Example Usage

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: StatusAggregate
metadata:
  name: app-health
spec:
  aggregationStrategy: AllHealthy

  # Explicit references with API group
  resources:
    - kind: Pet
      name: main-pet
      # apiGroup defaults to petstore.example.com (operator's group)

    - kind: Deployment
      name: api-server
      apiGroup: apps  # Core K8s resource

    - kind: Certificate
      name: api-tls
      apiGroup: cert-manager.io  # External operator CRD

  # Selectors with API group
  selectors:
    - kind: Pod
      apiGroup: ""  # Core API group (pods, services, configmaps)
      matchLabels:
        app: petstore

    - kind: StatefulSet
      apiGroup: apps
      namePattern: "petstore-.*"
```

### Health Extraction Logic

```go
// Unified health extraction for any resource
func extractHealthFromResource(obj *unstructured.Unstructured) ResourceHealth {
    gvk := obj.GroupVersionKind()

    switch {
    // Native Kubernetes workloads
    case gvk.Group == "apps" && gvk.Kind == "Deployment":
        return extractDeploymentHealth(obj)
    case gvk.Group == "apps" && gvk.Kind == "StatefulSet":
        return extractStatefulSetHealth(obj)
    case gvk.Group == "apps" && gvk.Kind == "DaemonSet":
        return extractDaemonSetHealth(obj)
    case gvk.Group == "batch" && gvk.Kind == "Job":
        return extractJobHealth(obj)

    // Core resources
    case gvk.Group == "" && gvk.Kind == "Pod":
        return extractPodHealth(obj)
    case gvk.Group == "" && gvk.Kind == "Service":
        return extractServiceHealth(obj)
    case gvk.Group == "" && gvk.Kind == "PersistentVolumeClaim":
        return extractPVCHealth(obj)

    // Generic: Use standard conditions
    default:
        return extractGenericHealth(obj)
    }
}

func extractDeploymentHealth(obj *unstructured.Unstructured) ResourceHealth {
    // Check .status.conditions for Available=True
    conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
    for _, c := range conditions {
        cond := c.(map[string]interface{})
        if cond["type"] == "Available" && cond["status"] == "True" {
            return ResourceHealth{State: "Synced", Ready: true}
        }
    }

    // Check replica counts
    desired, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
    ready, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")

    if ready >= desired && desired > 0 {
        return ResourceHealth{State: "Synced", Ready: true}
    }
    return ResourceHealth{State: "Pending", Ready: false}
}

func extractGenericHealth(obj *unstructured.Unstructured) ResourceHealth {
    // Standard Kubernetes conditions pattern
    conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
    if !found {
        // No conditions - assume healthy if exists
        return ResourceHealth{State: "Synced", Ready: true, Message: "No conditions (existence check only)"}
    }

    // Look for Ready or Available condition
    for _, c := range conditions {
        cond := c.(map[string]interface{})
        condType := cond["type"].(string)
        if (condType == "Ready" || condType == "Available") && cond["status"] == "True" {
            return ResourceHealth{State: "Synced", Ready: true}
        }
    }

    return ResourceHealth{State: "Pending", Ready: false}
}
```

### RBAC Generation

```go
// Generator adds RBAC based on referenced API groups
{{- if .HasNativeK8sReferences }}
// Native Kubernetes resources
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets;daemonsets;replicasets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods;services;configmaps;secrets;persistentvolumeclaims,verbs=get;list;watch
{{- end }}

{{- range .ExternalAPIGroups }}
// External CRD: {{ . }}
// +kubebuilder:rbac:groups={{ . }},resources=*,verbs=get;list;watch
{{- end }}
```

### Pros
- Clean, explicit API group specification
- Full control over which resources to include
- Works with any Kubernetes resource or CRD
- Type-safe with validation

### Cons
- Verbose for common cases
- User must know API group names
- RBAC must be manually configured or auto-detected

---

## Option 2: Well-Known Resource Shortcuts

**Approach**: Provide shorthand aliases for common Kubernetes resources.

### Type Changes

```go
type ResourceSelector struct {
    // Option A: Use Kind with well-known prefix
    // "k8s:Deployment", "k8s:Pod", "ext:Certificate"
    Kind string `json:"kind"`

    // Option B: Separate field for resource type
    // +kubebuilder:validation:Enum=native;external;generated
    ResourceType string `json:"resourceType,omitempty"`

    // Existing fields...
    NamePattern string            `json:"namePattern,omitempty"`
    MatchLabels map[string]string `json:"matchLabels,omitempty"`
}
```

### Well-Known Mappings

```go
var wellKnownResources = map[string]schema.GroupVersionKind{
    // Workloads
    "k8s:Deployment":  {Group: "apps", Version: "v1", Kind: "Deployment"},
    "k8s:StatefulSet": {Group: "apps", Version: "v1", Kind: "StatefulSet"},
    "k8s:DaemonSet":   {Group: "apps", Version: "v1", Kind: "DaemonSet"},
    "k8s:ReplicaSet":  {Group: "apps", Version: "v1", Kind: "ReplicaSet"},
    "k8s:Job":         {Group: "batch", Version: "v1", Kind: "Job"},
    "k8s:CronJob":     {Group: "batch", Version: "v1", Kind: "CronJob"},

    // Core
    "k8s:Pod":         {Group: "", Version: "v1", Kind: "Pod"},
    "k8s:Service":     {Group: "", Version: "v1", Kind: "Service"},
    "k8s:ConfigMap":   {Group: "", Version: "v1", Kind: "ConfigMap"},
    "k8s:Secret":      {Group: "", Version: "v1", Kind: "Secret"},
    "k8s:PVC":         {Group: "", Version: "v1", Kind: "PersistentVolumeClaim"},

    // Networking
    "k8s:Ingress":     {Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"},
    "k8s:NetworkPolicy": {Group: "networking.k8s.io", Version: "v1", Kind: "NetworkPolicy"},

    // Common external CRDs
    "ext:Certificate":      {Group: "cert-manager.io", Version: "v1", Kind: "Certificate"},
    "ext:Issuer":           {Group: "cert-manager.io", Version: "v1", Kind: "Issuer"},
    "ext:ServiceMonitor":   {Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"},
    "ext:PrometheusRule":   {Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"},
}
```

### Example Usage

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: StatusAggregate
metadata:
  name: app-health
spec:
  resources:
    # Generated CRDs (no prefix needed)
    - kind: Pet
      name: main-pet

    # Native K8s with shorthand
    - kind: k8s:Deployment
      name: api-server

    # Well-known external CRD
    - kind: ext:Certificate
      name: api-tls

  selectors:
    - kind: k8s:Pod
      matchLabels:
        app: petstore
```

### Pros
- Concise syntax for common resources
- No need to remember API groups
- Extensible via configuration

### Cons
- Magic strings may be confusing
- Limited to predefined mappings
- Version pinning issues

---

## Option 3: Typed Native Resource Fields

**Approach**: Add dedicated fields for native Kubernetes resources with full type safety.

### Type Changes

```go
type AggregateSpec struct {
    // Existing: Generated CRD references
    Resources         []ResourceReference  `json:"resources,omitempty"`
    ResourceSelectors []ResourceSelector   `json:"selectors,omitempty"`

    // New: Native Kubernetes resources
    NativeResources struct {
        // Workloads
        Deployments  []NamespacedName `json:"deployments,omitempty"`
        StatefulSets []NamespacedName `json:"statefulSets,omitempty"`
        DaemonSets   []NamespacedName `json:"daemonSets,omitempty"`
        Jobs         []NamespacedName `json:"jobs,omitempty"`

        // Core
        Pods         []NamespacedName `json:"pods,omitempty"`
        Services     []NamespacedName `json:"services,omitempty"`
        ConfigMaps   []NamespacedName `json:"configMaps,omitempty"`
        Secrets      []NamespacedName `json:"secrets,omitempty"`
    } `json:"nativeResources,omitempty"`

    // New: Workload selectors (common pattern)
    WorkloadSelectors []WorkloadSelector `json:"workloadSelectors,omitempty"`
}

type NamespacedName struct {
    Name      string `json:"name"`
    Namespace string `json:"namespace,omitempty"`
}

type WorkloadSelector struct {
    // +kubebuilder:validation:Enum=Deployment;StatefulSet;DaemonSet;Pod
    Kind        string            `json:"kind"`
    MatchLabels map[string]string `json:"matchLabels,omitempty"`
    NamePattern string            `json:"namePattern,omitempty"`
    Namespace   string            `json:"namespace,omitempty"`
}
```

### Example Usage

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: StatusAggregate
metadata:
  name: app-health
spec:
  # Generated CRDs
  resources:
    - kind: Pet
      name: main-pet

  # Native K8s - typed fields
  nativeResources:
    deployments:
      - name: api-server
      - name: worker
        namespace: backend
    statefulSets:
      - name: database
    configMaps:
      - name: app-config

  # Workload selectors
  workloadSelectors:
    - kind: Pod
      matchLabels:
        app: petstore
```

### Pros
- Full IDE autocompletion and validation
- Clear separation of concerns
- Easy to extend with resource-specific options

### Cons
- Verbose type definitions
- New field for each resource type
- Bundle would need similar structure

---

## Option 4: Generic Unstructured References

**Approach**: Use Kubernetes-native object references with dynamic client.

### Type Changes

```go
type AggregateSpec struct {
    // Existing fields...

    // Generic Kubernetes object references
    // +kubebuilder:validation:Optional
    ObjectReferences []corev1.ObjectReference `json:"objectReferences,omitempty"`

    // Generic selectors using GVK
    // +kubebuilder:validation:Optional
    GenericSelectors []GenericSelector `json:"genericSelectors,omitempty"`
}

type GenericSelector struct {
    // Full GVK specification
    APIVersion string `json:"apiVersion"` // e.g., "apps/v1", "v1"
    Kind       string `json:"kind"`       // e.g., "Deployment", "Pod"

    // Selection criteria
    Namespace   string            `json:"namespace,omitempty"`
    MatchLabels map[string]string `json:"matchLabels,omitempty"`
    NamePattern string            `json:"namePattern,omitempty"`
}
```

### Example Usage

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: StatusAggregate
metadata:
  name: app-health
spec:
  # Explicit object references (Kubernetes native type)
  objectReferences:
    - apiVersion: apps/v1
      kind: Deployment
      name: api-server
      namespace: default

    - apiVersion: v1
      kind: Service
      name: api-service

  # Generic selectors
  genericSelectors:
    - apiVersion: apps/v1
      kind: StatefulSet
      matchLabels:
        app: database

    - apiVersion: cert-manager.io/v1
      kind: Certificate
      namePattern: ".*-tls"
```

### Controller Implementation

```go
func (r *AggregateReconciler) fetchGenericResource(ctx context.Context, ref corev1.ObjectReference) (*unstructured.Unstructured, error) {
    gvk := schema.FromAPIVersionAndKind(ref.APIVersion, ref.Kind)

    // Get REST mapping for the GVK
    mapping, err := r.RESTMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
    if err != nil {
        return nil, fmt.Errorf("unknown resource type %v: %w", gvk, err)
    }

    // Use dynamic client
    obj := &unstructured.Unstructured{}
    obj.SetGroupVersionKind(gvk)

    err = r.Client.Get(ctx, types.NamespacedName{
        Name:      ref.Name,
        Namespace: ref.Namespace,
    }, obj)

    return obj, err
}
```

### Pros
- Maximum flexibility
- Uses standard Kubernetes types
- Works with any current or future resource

### Cons
- No compile-time type checking
- Requires dynamic client setup
- More complex watch configuration

---

## Option 5: Condition-Based Health Protocol

**Approach**: Define a standard health protocol that any resource can implement.

### Health Protocol Definition

```go
// Standard condition types for health
const (
    ConditionTypeReady     = "Ready"
    ConditionTypeAvailable = "Available"
    ConditionTypeHealthy   = "Healthy"
    ConditionTypeDegraded  = "Degraded"
)

// Health extraction priority
var healthConditionPriority = []string{
    "Ready",      // Most specific
    "Available",  // Workloads
    "Healthy",    // Custom CRDs
    "Succeeded",  // Jobs
}
```

### Unified Health Extraction

```go
func extractHealth(obj *unstructured.Unstructured) ResourceHealth {
    // 1. Check for standard conditions
    conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
    if found {
        for _, condType := range healthConditionPriority {
            if health := findCondition(conditions, condType); health != nil {
                return *health
            }
        }
    }

    // 2. Check for observedGeneration (indicates controller processed spec)
    observedGen, found, _ := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")
    generation := obj.GetGeneration()
    if found && observedGen < generation {
        return ResourceHealth{State: "Pending", Message: "Waiting for controller"}
    }

    // 3. Resource-specific checks
    return extractResourceSpecificHealth(obj)
}

func extractResourceSpecificHealth(obj *unstructured.Unstructured) ResourceHealth {
    gvk := obj.GroupVersionKind()

    switch gvk.Kind {
    case "Deployment", "StatefulSet", "DaemonSet":
        return extractWorkloadHealth(obj)
    case "Job":
        return extractJobHealth(obj)
    case "Pod":
        return extractPodHealth(obj)
    case "Service":
        return extractServiceHealth(obj)
    case "PersistentVolumeClaim":
        return extractPVCHealth(obj)
    default:
        // Existence = healthy for unknown types
        return ResourceHealth{State: "Synced", Ready: true}
    }
}

func extractWorkloadHealth(obj *unstructured.Unstructured) ResourceHealth {
    desired, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
    if desired == 0 {
        desired = 1 // Default
    }

    ready, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
    available, _, _ := unstructured.NestedInt64(obj.Object, "status", "availableReplicas")

    if ready >= desired {
        return ResourceHealth{
            State:   "Synced",
            Ready:   true,
            Message: fmt.Sprintf("%d/%d replicas ready", ready, desired),
        }
    }

    if available > 0 {
        return ResourceHealth{
            State:   "Degraded",
            Ready:   false,
            Message: fmt.Sprintf("%d/%d replicas ready (%d available)", ready, desired, available),
        }
    }

    return ResourceHealth{
        State:   "Pending",
        Ready:   false,
        Message: fmt.Sprintf("0/%d replicas ready", desired),
    }
}
```

---

## Recommended Approach: Hybrid (Options 1 + 2 + 5)

Combine the best aspects of multiple options:

### Implementation

```go
type ResourceSelector struct {
    // Kind with optional prefix for shortcuts
    // Examples: "Pet", "k8s:Deployment", "apps/Deployment"
    Kind string `json:"kind"`

    // Explicit API group (overrides prefix)
    APIGroup string `json:"apiGroup,omitempty"`

    // Standard selector fields
    NamePattern string            `json:"namePattern,omitempty"`
    MatchLabels map[string]string `json:"matchLabels,omitempty"`
    Namespace   string            `json:"namespace,omitempty"`
}

type ResourceReference struct {
    Kind      string `json:"kind"`
    Name      string `json:"name"`
    APIGroup  string `json:"apiGroup,omitempty"`
    Namespace string `json:"namespace,omitempty"`
}
```

### Kind Resolution Logic

```go
func resolveKind(kind string, defaultGroup string) schema.GroupVersionKind {
    // 1. Check for k8s: prefix
    if strings.HasPrefix(kind, "k8s:") {
        return wellKnownK8s[strings.TrimPrefix(kind, "k8s:")]
    }

    // 2. Check for ext: prefix
    if strings.HasPrefix(kind, "ext:") {
        return wellKnownExternal[strings.TrimPrefix(kind, "ext:")]
    }

    // 3. Check for group/kind format (e.g., "apps/Deployment")
    if strings.Contains(kind, "/") {
        parts := strings.SplitN(kind, "/", 2)
        return schema.GroupVersionKind{Group: parts[0], Kind: parts[1]}
    }

    // 4. Default to operator's API group
    return schema.GroupVersionKind{Group: defaultGroup, Kind: kind}
}
```

### Example Usage

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: StatusAggregate
metadata:
  name: full-stack-health
spec:
  aggregationStrategy: AllHealthy

  resources:
    # Generated CRD (default group)
    - kind: Pet
      name: main-pet

    # Native K8s with shortcut
    - kind: k8s:Deployment
      name: api-server

    # Native K8s with explicit group
    - kind: Deployment
      name: worker
      apiGroup: apps

    # External CRD with shortcut
    - kind: ext:Certificate
      name: api-tls

    # External CRD with explicit group
    - kind: Certificate
      name: backup-tls
      apiGroup: cert-manager.io

  selectors:
    # All pods with label
    - kind: k8s:Pod
      matchLabels:
        app: petstore

    # All StatefulSets matching pattern
    - kind: apps/StatefulSet
      namePattern: "db-.*"

    # All generated Query CRDs
    - kind: PetFindByStatusQuery
      matchLabels:
        environment: production

  derivedValues:
    - name: totalPods
      expression: "size(pods)"
    - name: healthyDeployments
      expression: "deployments.filter(d, d.status.state == 'Synced').size()"
```

### Bundle Support

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: Bundle
metadata:
  name: full-stack
spec:
  resources:
    # Wait for ConfigMap before creating Pet
    - id: config
      kind: k8s:ConfigMap
      waitFor: true  # Don't create, just wait for existence
      spec:
        name: pet-config

    # Create Pet after config exists
    - id: pet
      kind: Pet
      dependsOn: [config]
      spec:
        name: "Buddy"
        configRef: "${resources.config.metadata.name}"
```

### Generator Configuration

```yaml
# openapi-operator-gen.yaml
nativeResources:
  enabled: true

  # Which native resources to support
  include:
    - group: apps
      kinds: [Deployment, StatefulSet, DaemonSet]
    - group: ""
      kinds: [Pod, Service, ConfigMap, Secret, PersistentVolumeClaim]
    - group: batch
      kinds: [Job, CronJob]
    - group: networking.k8s.io
      kinds: [Ingress]

  # External CRDs to support
  external:
    - group: cert-manager.io
      kinds: [Certificate, Issuer, ClusterIssuer]
    - group: monitoring.coreos.com
      kinds: [ServiceMonitor, PrometheusRule]

  # Custom health extractors
  healthExtractors:
    - group: custom.example.com
      kind: MyResource
      readyCondition: "Synchronized"
```

---

## Implementation Considerations

### RBAC Generation

```go
// Generate RBAC based on configuration
func generateRBAC(config NativeResourceConfig) []string {
    var rules []string

    // Group by API group for efficient rules
    byGroup := make(map[string][]string)
    for _, res := range config.Include {
        byGroup[res.Group] = append(byGroup[res.Group], pluralize(res.Kinds)...)
    }

    for group, resources := range byGroup {
        rules = append(rules, fmt.Sprintf(
            "// +kubebuilder:rbac:groups=%s,resources=%s,verbs=get;list;watch",
            group, strings.Join(resources, ";"),
        ))
    }

    return rules
}
```

### Watch Configuration

```go
func (r *AggregateReconciler) SetupWithManager(mgr ctrl.Manager) error {
    builder := ctrl.NewControllerManagedBy(mgr).
        For(&v1alpha1.StatusAggregate{})

    // Watch generated CRDs
    for _, kind := range generatedKinds {
        builder = builder.Watches(/* ... */)
    }

    // Watch native K8s resources
    if r.NativeResourcesEnabled {
        // Deployments
        builder = builder.Watches(
            &appsv1.Deployment{},
            handler.EnqueueRequestsFromMapFunc(r.findAggregatesForDeployment),
        )
        // StatefulSets, Pods, etc...
    }

    return builder.Complete(r)
}
```

### CEL Variable Generation

```go
// Make native resources available in CEL expressions
func buildCELVariables(resources []ResourceStatus) map[string]interface{} {
    vars := make(map[string]interface{})

    // Group by kind (lowercase plural)
    byKind := make(map[string][]interface{})
    for _, r := range resources {
        key := strings.ToLower(r.Kind) + "s"  // deployment -> deployments
        byKind[key] = append(byKind[key], r.ToMap())
    }

    for kind, list := range byKind {
        vars[kind] = list
    }

    // Also provide flat list
    vars["resources"] = resources

    return vars
}
```

---

## Summary

| Option | Flexibility | Usability | Type Safety | Complexity |
|--------|-------------|-----------|-------------|------------|
| 1. Extended Selector | High | Medium | Medium | Low |
| 2. Well-Known Shortcuts | Medium | High | Medium | Low |
| 3. Typed Fields | Low | High | High | Medium |
| 4. Generic Unstructured | Very High | Low | Low | High |
| 5. Condition Protocol | High | Medium | Medium | Medium |
| **Hybrid (Recommended)** | **High** | **High** | **Medium** | **Medium** |

### Recommended Implementation Order

1. **Phase 1**: Add `apiGroup` field to selectors (Option 1)
2. **Phase 2**: Add well-known shortcuts with `k8s:` prefix (Option 2)
3. **Phase 3**: Implement condition-based health extraction (Option 5)
4. **Phase 4**: Add generator configuration for RBAC and watches
5. **Phase 5**: Add CEL variables for native resources
