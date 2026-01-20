// Package aggregate provides shared types and utilities for aggregate/bundle resource selection.
// This package is used by both the generated aggregate controllers and the cel-test CLI tool.
package aggregate

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// ResourceReference represents an explicit reference to a specific resource.
// Used in Aggregate/Bundle spec.resources for referencing resources by name.
type ResourceReference struct {
	// Kind is the resource kind (e.g., "Order", "Pet")
	Kind string
	// Name is the resource name
	Name string
	// Namespace is the resource namespace (optional, defaults to aggregate's namespace)
	Namespace string
}

// ResourceSelector represents criteria for selecting multiple resources.
// Used in Aggregate/Bundle spec.resourceSelectors for dynamic selection.
type ResourceSelector struct {
	// Kind is the resource kind to select (required)
	Kind string
	// MatchLabels is a map of label key-value pairs to match (optional)
	MatchLabels map[string]string
	// NamePattern is a regex pattern to match resource names (optional)
	NamePattern string
}

// CompiledSelector is a ResourceSelector with pre-compiled regex for efficient matching.
type CompiledSelector struct {
	ResourceSelector
	// nameRegex is the compiled name pattern (nil if no pattern specified)
	nameRegex *regexp.Regexp
	// labelSelector is the compiled label selector
	labelSelector labels.Selector
}

// CompileSelector compiles a ResourceSelector into a CompiledSelector for efficient matching.
// Returns an error if the NamePattern is an invalid regex.
func CompileSelector(sel ResourceSelector) (*CompiledSelector, error) {
	compiled := &CompiledSelector{
		ResourceSelector: sel,
	}

	// Compile name pattern if specified
	if sel.NamePattern != "" {
		regex, err := regexp.Compile(sel.NamePattern)
		if err != nil {
			return nil, fmt.Errorf("invalid namePattern %q: %w", sel.NamePattern, err)
		}
		compiled.nameRegex = regex
	}

	// Build label selector
	if len(sel.MatchLabels) > 0 {
		compiled.labelSelector = labels.SelectorFromSet(sel.MatchLabels)
	} else {
		compiled.labelSelector = labels.Everything()
	}

	return compiled, nil
}

// Matches checks if a resource matches the selector criteria.
// The resource must match the kind, labels (if specified), and name pattern (if specified).
func (cs *CompiledSelector) Matches(kind, name string, resourceLabels map[string]string) bool {
	// Check kind
	if cs.Kind != kind {
		return false
	}

	// Check labels
	if !cs.labelSelector.Matches(labels.Set(resourceLabels)) {
		return false
	}

	// Check name pattern
	if cs.nameRegex != nil && !cs.nameRegex.MatchString(name) {
		return false
	}

	return true
}

// LabelSelectorString returns the label selector as a string for use in list options.
func (cs *CompiledSelector) LabelSelectorString() string {
	return cs.labelSelector.String()
}

// ParseResourceReference parses a map (from YAML/JSON) into a ResourceReference.
func ParseResourceReference(m map[string]interface{}) ResourceReference {
	ref := ResourceReference{}
	if kind, ok := m["kind"].(string); ok {
		ref.Kind = kind
	}
	if name, ok := m["name"].(string); ok {
		ref.Name = name
	}
	if namespace, ok := m["namespace"].(string); ok {
		ref.Namespace = namespace
	}
	return ref
}

// ParseResourceSelector parses a map (from YAML/JSON) into a ResourceSelector.
func ParseResourceSelector(m map[string]interface{}) ResourceSelector {
	sel := ResourceSelector{}
	if kind, ok := m["kind"].(string); ok {
		sel.Kind = kind
	}
	if matchLabels, ok := m["matchLabels"].(map[string]interface{}); ok {
		sel.MatchLabels = make(map[string]string)
		for k, v := range matchLabels {
			if vs, ok := v.(string); ok {
				sel.MatchLabels[k] = vs
			}
		}
	}
	if namePattern, ok := m["namePattern"].(string); ok {
		sel.NamePattern = namePattern
	}
	return sel
}

// IsValid returns true if the ResourceReference has required fields.
func (ref ResourceReference) IsValid() bool {
	return ref.Kind != "" && ref.Name != ""
}

// IsValid returns true if the ResourceSelector has required fields.
func (sel ResourceSelector) IsValid() bool {
	return sel.Kind != ""
}

// DefaultNamespace returns the namespace to use, defaulting to the provided default if empty.
func DefaultNamespace(namespace, defaultNs string) string {
	if namespace != "" {
		return namespace
	}
	if defaultNs != "" {
		return defaultNs
	}
	return "default"
}

// ResourceKey returns a unique key for a resource based on kind, namespace, and name.
// Used for deduplication when processing both explicit references and selectors.
func ResourceKey(kind, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s", kind, namespace, name)
}

// KindToVariableName converts a Kind name to a CEL variable name (lowercase plural).
// Example: "Order" -> "orders", "Pet" -> "pets"
func KindToVariableName(kind string) string {
	return strings.ToLower(kind) + "s"
}

// KindToResourceName converts a Kind name to a Kubernetes resource name (lowercase plural).
// Example: "Order" -> "orders", "Pet" -> "pets"
// Note: This is a simple pluralization. For irregular plurals, use a proper pluralizer.
func KindToResourceName(kind string) string {
	return strings.ToLower(kind) + "s"
}

// DynamicFetcher provides methods for fetching resources using a dynamic client.
// This is used by cel-test for runtime resource fetching.
type DynamicFetcher struct {
	Client     dynamic.Interface
	APIGroup   string
	APIVersion string
}

// NewDynamicFetcher creates a new DynamicFetcher.
func NewDynamicFetcher(client dynamic.Interface, apiGroup, apiVersion string) *DynamicFetcher {
	return &DynamicFetcher{
		Client:     client,
		APIGroup:   apiGroup,
		APIVersion: apiVersion,
	}
}

// GetResource fetches a specific resource by reference.
func (f *DynamicFetcher) GetResource(ctx context.Context, ref ResourceReference, defaultNamespace string) (*unstructured.Unstructured, error) {
	if !ref.IsValid() {
		return nil, fmt.Errorf("invalid resource reference: kind and name are required")
	}

	namespace := DefaultNamespace(ref.Namespace, defaultNamespace)
	gvr := schema.GroupVersionResource{
		Group:    f.APIGroup,
		Version:  f.APIVersion,
		Resource: KindToResourceName(ref.Kind),
	}

	return f.Client.Resource(gvr).Namespace(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
}

// ListResources lists resources matching a selector.
func (f *DynamicFetcher) ListResources(ctx context.Context, sel *CompiledSelector, namespace string) (*unstructured.UnstructuredList, error) {
	if !sel.IsValid() {
		return nil, fmt.Errorf("invalid resource selector: kind is required")
	}

	gvr := schema.GroupVersionResource{
		Group:    f.APIGroup,
		Version:  f.APIVersion,
		Resource: KindToResourceName(sel.Kind),
	}

	listOpts := metav1.ListOptions{
		LabelSelector: sel.LabelSelectorString(),
	}

	return f.Client.Resource(gvr).Namespace(namespace).List(ctx, listOpts)
}

// FilterByNamePattern filters a list of resources by the selector's name pattern.
// Returns only resources whose names match the pattern (or all if no pattern).
func (f *DynamicFetcher) FilterByNamePattern(list *unstructured.UnstructuredList, sel *CompiledSelector) []unstructured.Unstructured {
	if sel.nameRegex == nil {
		return list.Items
	}

	var filtered []unstructured.Unstructured
	for _, item := range list.Items {
		if sel.nameRegex.MatchString(item.GetName()) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
