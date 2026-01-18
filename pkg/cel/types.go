package cel

import (
	"encoding/json"
	"strings"
)

// ResourceState represents the sync state of a resource.
type ResourceState string

const (
	// StateSynced indicates a resource is successfully synced with the external API.
	StateSynced ResourceState = "Synced"
	// StateFailed indicates a sync operation failed.
	StateFailed ResourceState = "Failed"
	// StatePending indicates a sync operation is in progress.
	StatePending ResourceState = "Pending"
	// StateQueried indicates a query CRD successfully retrieved data.
	StateQueried ResourceState = "Queried"
	// StateCompleted indicates an action CRD completed successfully.
	StateCompleted ResourceState = "Completed"
	// StateSkipped indicates a resource was skipped (e.g., not found).
	StateSkipped ResourceState = "Skipped"
)

// IsHealthy returns true if the state represents a healthy/successful status.
// Synced, Queried, and Completed are all considered healthy states.
func (s ResourceState) IsHealthy() bool {
	switch s {
	case StateSynced, StateQueried, StateCompleted:
		return true
	default:
		return false
	}
}

// AggregationStrategy defines how resources are aggregated for status calculation.
type AggregationStrategy string

const (
	// StrategyAllHealthy requires all resources to be healthy.
	StrategyAllHealthy AggregationStrategy = "AllHealthy"
	// StrategyAnyHealthy requires at least one resource to be healthy.
	StrategyAnyHealthy AggregationStrategy = "AnyHealthy"
	// StrategyQuorum requires a majority of resources to be healthy.
	StrategyQuorum AggregationStrategy = "Quorum"
)

// EvaluateStrategy determines the overall state based on the aggregation strategy.
// Returns "Synced" if healthy, "Degraded" if not meeting the strategy, or "Pending" if empty.
func EvaluateStrategy(strategy AggregationStrategy, total, synced, failed, pending int64) ResourceState {
	if total == 0 {
		return StatePending
	}

	switch strategy {
	case StrategyAllHealthy:
		if synced == total {
			return StateSynced
		}
		return ResourceState("Degraded")

	case StrategyAnyHealthy:
		if synced > 0 {
			return StateSynced
		}
		return StateFailed

	case StrategyQuorum:
		if synced > total/2 {
			return StateSynced
		}
		return ResourceState("Degraded")

	default:
		// Default to AllHealthy behavior
		if synced == total {
			return StateSynced
		}
		return ResourceState("Degraded")
	}
}

// ResourceData represents a resource in CEL-compatible format.
// This structure can be converted to map[string]any for CEL evaluation.
type ResourceData struct {
	Kind     string         `json:"kind"`
	Metadata ResourceMeta   `json:"metadata"`
	Spec     map[string]any `json:"spec,omitempty"`
	Status   ResourceStatus `json:"status,omitempty"`
}

// ResourceMeta contains resource metadata.
type ResourceMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ResourceStatus contains resource status information.
type ResourceStatus struct {
	State      string `json:"state"`
	ExternalID string `json:"externalID,omitempty"`
	Message    string `json:"message,omitempty"`
}

// ToMap converts ResourceData to a map suitable for CEL evaluation.
func (r *ResourceData) ToMap() map[string]any {
	result := map[string]any{
		"kind": r.Kind,
		"metadata": map[string]any{
			"name":        r.Metadata.Name,
			"namespace":   r.Metadata.Namespace,
			"labels":      r.Metadata.Labels,
			"annotations": r.Metadata.Annotations,
		},
		"status": map[string]any{
			"state":      r.Status.State,
			"externalID": r.Status.ExternalID,
			"message":    r.Status.Message,
		},
	}

	if r.Spec != nil {
		result["spec"] = r.Spec
	}

	return result
}

// ObjectToMap converts any object to a map[string]any via JSON marshaling.
// This is useful for converting Kubernetes resources to CEL-compatible format.
func ObjectToMap(obj any) (map[string]any, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// KindToVariableName converts a Kind name to its CEL variable name.
// For example: "Order" -> "orders", "Pet" -> "pets", "PetFindbystatusQuery" -> "petfindbystatusquerys"
func KindToVariableName(kind string) string {
	return strings.ToLower(kind) + "s"
}

// ComputedValue represents a derived value computed from a CEL expression.
type ComputedValue struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

// DerivedValueDefinition defines a CEL expression to evaluate.
type DerivedValueDefinition struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
}
