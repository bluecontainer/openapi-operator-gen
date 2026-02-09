// Package nodes provides Kubernetes workload discovery for Rundeck node sources.
// It discovers StatefulSets, Deployments, and Helm releases, outputting them
// as Rundeck resource model JSON with attributes that map to kubectl plugin
// targeting flags.
package nodes

import "encoding/json"

// RundeckNode represents a single Rundeck resource model node.
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
	Healthy            string `json:"healthy"`

	// ExtraAttributes holds dynamic attributes from labels/annotations.
	// These are merged into the JSON output at the top level.
	ExtraAttributes map[string]string `json:"-"`
}

// MarshalJSON implements custom JSON marshaling to merge ExtraAttributes.
func (n *RundeckNode) MarshalJSON() ([]byte, error) {
	// Build the base map
	result := map[string]interface{}{
		"nodename":        n.NodeName,
		"hostname":        n.Hostname,
		"tags":            n.Tags,
		"osFamily":        n.OSFamily,
		"node-executor":   n.NodeExecutor,
		"file-copier":     n.FileCopier,
		"targetType":      n.TargetType,
		"targetValue":     n.TargetValue,
		"targetNamespace": n.TargetNamespace,
		"workloadKind":    n.WorkloadKind,
		"workloadName":    n.WorkloadName,
		"podCount":        n.PodCount,
		"healthyPods":     n.HealthyPods,
		"healthy":         n.Healthy,
	}

	// Add optional fields
	if n.Cluster != "" {
		result["cluster"] = n.Cluster
	}
	if n.ClusterURL != "" {
		result["clusterUrl"] = n.ClusterURL
	}
	if n.ClusterTokenSuffix != "" {
		result["clusterTokenSuffix"] = n.ClusterTokenSuffix
	}

	// Merge extra attributes
	for k, v := range n.ExtraAttributes {
		result[k] = v
	}

	return json.Marshal(result)
}

// DiscoverOptions configures workload discovery behavior.
type DiscoverOptions struct {
	// Namespace to discover workloads in. Empty string with AllNamespaces=true
	// discovers across all namespaces.
	Namespace string

	// AllNamespaces discovers workloads across all namespaces.
	AllNamespaces bool

	// LabelSelector filters workloads by label (e.g., "app=myapp").
	LabelSelector string

	// ClusterName is an identifier for multi-cluster setups.
	// If set, it's included in node names and tags.
	ClusterName string

	// ClusterURL is the Kubernetes API server URL.
	// Stored in node attributes for per-node cluster targeting.
	ClusterURL string

	// ClusterTokenSuffix is the Rundeck Key Storage path suffix for this cluster.
	// Jobs use "keys/${node.clusterTokenSuffix}" for dynamic credential selection.
	ClusterTokenSuffix string

	// DefaultTokenSuffix is the default Key Storage path suffix when
	// ClusterTokenSuffix is not set. Defaults to "rundeck/k8s-token".
	DefaultTokenSuffix string

	// Phase 1: Core Filtering Options

	// Types filters to only include these workload types.
	// Valid values: "helm-release", "statefulset", "deployment"
	// Empty means all types.
	Types []string

	// ExcludeTypes excludes these workload types from discovery.
	// Applied after Types filtering.
	ExcludeTypes []string

	// ExcludeLabels excludes workloads matching any of these label selectors.
	// Each entry is a label selector (e.g., "app=operator").
	ExcludeLabels []string

	// ExcludeOperator excludes the operator controller-manager from discovery.
	// This is a convenience flag that excludes workloads matching common operator patterns:
	// - label app.kubernetes.io/component=operator
	// - label control-plane=controller-manager
	// - name matching *-controller-manager or *-operator
	ExcludeOperator bool

	// HealthyOnly includes only workloads where all pods are running.
	HealthyOnly bool

	// UnhealthyOnly includes only workloads where some pods are not running.
	UnhealthyOnly bool

	// Phase 2: Pattern Matching Options

	// NamePatterns filters to only include workloads matching these glob patterns.
	// Uses filepath.Match syntax (e.g., "myapp-*", "*-backend").
	NamePatterns []string

	// ExcludePatterns excludes workloads matching these glob patterns.
	ExcludePatterns []string

	// ExcludeNamespaces excludes workloads in these specific namespaces.
	ExcludeNamespaces []string

	// NamespacePatterns filters to only include workloads in namespaces matching these patterns.
	NamespacePatterns []string

	// ExcludeNamespacePatterns excludes workloads in namespaces matching these patterns.
	ExcludeNamespacePatterns []string

	// Phase 4: Output Customization Options

	// AddTags adds custom static tags to all discovered nodes.
	// Example: ["env:prod", "team:platform"]
	AddTags []string

	// LabelsAsTags converts Kubernetes labels to Rundeck tags.
	// Specify label keys (e.g., "app.kubernetes.io/name", "tier").
	// The label value becomes the tag value.
	LabelsAsTags []string

	// LabelAttributes adds Kubernetes labels as node attributes.
	// Specify label keys to include as node attributes.
	// The attribute name will be "label.<key>" (dots converted to underscores).
	LabelAttributes []string

	// AnnotationAttributes adds Kubernetes annotations as node attributes.
	// Specify annotation keys to include as node attributes.
	// The attribute name will be "annotation.<key>" (dots converted to underscores).
	AnnotationAttributes []string
}

// helmInfo tracks Helm release information during discovery.
// Multiple workloads can belong to a single Helm release.
type helmInfo struct {
	release      string
	namespace    string
	workloadKind string
	workloadName string
	totalPods    int
	healthyPods  int
	// Labels and annotations from the first workload (for tag/attribute extraction)
	labels      map[string]string
	annotations map[string]string
}

// DefaultTokenSuffix is the default Rundeck Key Storage path suffix.
const DefaultTokenSuffix = "rundeck/k8s-token"

// Workload type constants
const (
	TypeHelmRelease = "helm-release"
	TypeStatefulSet = "statefulset"
	TypeDeployment  = "deployment"
)

// ValidTypes returns the list of valid workload types.
func ValidTypes() []string {
	return []string{TypeHelmRelease, TypeStatefulSet, TypeDeployment}
}
