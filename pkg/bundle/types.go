// Package bundle provides shared types and utilities for Bundle CRD resource management.
// This package is used by both the generated bundle controllers and the cel-test CLI tool.
package bundle

// ResourceSpec represents a parsed Bundle resource specification.
// This is a generic representation that can be used by both controllers and CLI tools.
type ResourceSpec struct {
	// ID is a unique identifier for this resource within the bundle
	ID string
	// Kind specifies the resource kind (e.g., "Pet", "Order")
	Kind string
	// Spec contains the resource specification as a map
	Spec map[string]interface{}
	// DependsOn lists resource IDs that must be synced before this resource
	DependsOn []string
	// ReadyWhen defines CEL conditions for readiness (optional)
	ReadyWhen []string
	// SkipWhen defines CEL conditions for skipping (optional)
	SkipWhen []string
}

// ResourceStatus represents the status of a created/managed resource.
// Used for expression resolution during resource creation.
type ResourceStatus struct {
	// ID is the resource identifier
	ID string
	// Kind is the resource kind
	Kind string
	// Name is the generated CR name in the cluster
	Name string
	// Namespace is the resource namespace
	Namespace string
	// State is the current state (Pending, Synced, Failed, etc.)
	State string
	// ExternalID is the ID from the REST API (if available)
	ExternalID string
	// Message contains status details or error information
	Message string
	// Ready indicates if readyWhen conditions are met
	Ready bool
	// Skipped indicates if skipWhen conditions were met
	Skipped bool
	// Extra holds additional status fields for expression resolution
	Extra map[string]interface{}
}

// ToMap converts ResourceStatus to a map for expression resolution.
func (s *ResourceStatus) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"status": map[string]interface{}{
			"state":      s.State,
			"externalID": s.ExternalID,
			"message":    s.Message,
			"ready":      s.Ready,
			"skipped":    s.Skipped,
		},
	}

	// Merge any extra fields
	if s.Extra != nil {
		for k, v := range s.Extra {
			result[k] = v
		}
	}

	return result
}

// DependencyExtractorOptions configures what sources to extract dependencies from.
type DependencyExtractorOptions struct {
	// IncludeExplicit includes explicit dependsOn declarations
	IncludeExplicit bool
	// IncludeSpecRefs includes ${resources.<id>...} references from spec
	IncludeSpecRefs bool
	// IncludeConditions includes references from readyWhen/skipWhen conditions
	IncludeConditions bool
	// IncludeBareRefs includes bare resources.<id>... patterns (CEL expressions)
	IncludeBareRefs bool
}

// DefaultExtractorOptions returns the default options for dependency extraction.
func DefaultExtractorOptions() DependencyExtractorOptions {
	return DependencyExtractorOptions{
		IncludeExplicit:   true,
		IncludeSpecRefs:   true,
		IncludeConditions: true,
		IncludeBareRefs:   true,
	}
}
