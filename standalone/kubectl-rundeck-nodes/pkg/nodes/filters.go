package nodes

import (
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

// Filter holds pre-compiled filter state for efficient filtering.
type Filter struct {
	opts DiscoverOptions

	// Pre-computed type sets
	includeTypes map[string]bool
	excludeTypes map[string]bool

	// Pre-parsed exclude label selectors
	excludeSelectors []labels.Selector
}

// NewFilter creates a filter from discovery options.
func NewFilter(opts DiscoverOptions) (*Filter, error) {
	f := &Filter{
		opts:         opts,
		includeTypes: make(map[string]bool),
		excludeTypes: make(map[string]bool),
	}

	// Build type filter sets
	for _, t := range opts.Types {
		f.includeTypes[strings.ToLower(t)] = true
	}
	for _, t := range opts.ExcludeTypes {
		f.excludeTypes[strings.ToLower(t)] = true
	}

	// Parse exclude label selectors
	for _, sel := range opts.ExcludeLabels {
		parsed, err := labels.Parse(sel)
		if err != nil {
			return nil, err
		}
		f.excludeSelectors = append(f.excludeSelectors, parsed)
	}

	return f, nil
}

// ShouldIncludeType returns true if the workload type should be included.
func (f *Filter) ShouldIncludeType(workloadType string) bool {
	t := strings.ToLower(workloadType)

	// If types specified, must be in include list
	if len(f.includeTypes) > 0 && !f.includeTypes[t] {
		return false
	}

	// Check exclude list
	if f.excludeTypes[t] {
		return false
	}

	return true
}

// ShouldExcludeByLabels returns true if the workload should be excluded based on labels.
func (f *Filter) ShouldExcludeByLabels(workloadLabels map[string]string) bool {
	if len(f.excludeSelectors) == 0 {
		return false
	}

	labelSet := labels.Set(workloadLabels)
	for _, sel := range f.excludeSelectors {
		if sel.Matches(labelSet) {
			return true
		}
	}

	return false
}

// ShouldExcludeOperator returns true if the workload matches operator patterns.
func (f *Filter) ShouldExcludeOperator(name string, workloadLabels map[string]string) bool {
	if !f.opts.ExcludeOperator {
		return false
	}

	// Check labels
	if workloadLabels["app.kubernetes.io/component"] == "operator" {
		return true
	}
	if workloadLabels["control-plane"] == "controller-manager" {
		return true
	}

	// Check name patterns
	nameLower := strings.ToLower(name)
	if matched, _ := filepath.Match("*-controller-manager", nameLower); matched {
		return true
	}
	if matched, _ := filepath.Match("*-operator", nameLower); matched {
		return true
	}

	return false
}

// ShouldExcludeByHealth returns true if the workload should be excluded based on health.
func (f *Filter) ShouldExcludeByHealth(healthyPods, totalPods int) bool {
	if !f.opts.HealthyOnly {
		return false
	}

	// Exclude if not all pods are healthy
	return healthyPods < totalPods
}

// ShouldIncludeWorkload checks all filters and returns true if the workload should be included.
func (f *Filter) ShouldIncludeWorkload(obj *unstructured.Unstructured, workloadType string, healthyPods, totalPods int) bool {
	name := obj.GetName()
	workloadLabels := obj.GetLabels()

	// Type filter
	if !f.ShouldIncludeType(workloadType) {
		return false
	}

	// Exclude labels
	if f.ShouldExcludeByLabels(workloadLabels) {
		return false
	}

	// Operator exclusion
	if f.ShouldExcludeOperator(name, workloadLabels) {
		return false
	}

	// Health filter
	if f.ShouldExcludeByHealth(healthyPods, totalPods) {
		return false
	}

	return true
}

// ShouldIncludeHelmRelease checks filters for a Helm release node.
func (f *Filter) ShouldIncludeHelmRelease(info *helmInfo) bool {
	// Type filter
	if !f.ShouldIncludeType(TypeHelmRelease) {
		return false
	}

	// Health filter
	if f.ShouldExcludeByHealth(info.healthyPods, info.totalPods) {
		return false
	}

	// Note: We can't easily filter Helm releases by labels since they aggregate
	// multiple workloads. The workload-level filters already apply during discovery.

	return true
}
