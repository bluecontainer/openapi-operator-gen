// Package nodes provides Kubernetes workload discovery for Rundeck node sources.
// It discovers StatefulSets, Deployments, and Helm releases, outputting them
// as Rundeck resource model JSON with attributes that map to kubectl plugin
// targeting flags.
package nodes

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
}

// DefaultTokenSuffix is the default Rundeck Key Storage path suffix.
const DefaultTokenSuffix = "rundeck/k8s-token"
