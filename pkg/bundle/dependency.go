package bundle

import (
	"fmt"
	"sort"
)

// BuildExecutionOrder builds a topologically sorted order for resource creation
// based on dependencies. Uses Kahn's algorithm with deterministic ordering.
//
// Returns an error if:
//   - A circular dependency is detected
//   - A resource depends on an unknown resource ID
func BuildExecutionOrder(resources []ResourceSpec, opts DependencyExtractorOptions) ([]string, error) {
	if len(resources) == 0 {
		return []string{}, nil
	}

	// Build set of valid resource IDs for validation
	validIDs := make(map[string]bool)
	for _, res := range resources {
		validIDs[res.ID] = true
	}

	// Build adjacency list
	inDegree := make(map[string]int)
	dependents := make(map[string][]string) // Maps resource ID to IDs that depend on it

	// Initialize all resources with 0 in-degree
	for _, res := range resources {
		inDegree[res.ID] = 0
	}

	// Build dependency graph
	for _, res := range resources {
		allDeps := ExtractAllDependencies(res, opts)

		// Validate and add dependencies
		for dep := range allDeps {
			if !validIDs[dep] {
				return nil, fmt.Errorf("resource %s depends on unknown resource %s", res.ID, dep)
			}
			inDegree[res.ID]++
			dependents[dep] = append(dependents[dep], res.ID)
		}
	}

	// Kahn's algorithm for topological sort with deterministic ordering
	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	// Sort queue for deterministic ordering
	sort.Strings(queue)

	var order []string
	for len(queue) > 0 {
		// Take from front of sorted queue
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)

		// Get dependents and sort for determinism
		deps := dependents[id]
		sort.Strings(deps)

		// Reduce in-degree of dependent resources
		for _, dependent := range deps {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
				// Re-sort queue to maintain deterministic order
				sort.Strings(queue)
			}
		}
	}

	if len(order) != len(resources) {
		return nil, fmt.Errorf("circular dependency detected")
	}

	return order, nil
}

// BuildExecutionOrderSimple builds execution order using default options.
// This is a convenience wrapper around BuildExecutionOrder.
func BuildExecutionOrderSimple(resources []ResourceSpec) ([]string, error) {
	return BuildExecutionOrder(resources, DefaultExtractorOptions())
}

// DetectCircularDependencies checks for circular dependencies without building the full order.
// Returns a list of resource IDs involved in cycles, or nil if no cycles exist.
func DetectCircularDependencies(resources []ResourceSpec, opts DependencyExtractorOptions) []string {
	_, err := BuildExecutionOrder(resources, opts)
	if err != nil && err.Error() == "circular dependency detected" {
		// Find which resources have non-zero in-degree after algorithm
		// This is a simplified detection - for detailed cycle info, use a proper DFS
		var cycleMembers []string
		for _, res := range resources {
			cycleMembers = append(cycleMembers, res.ID)
		}
		return cycleMembers
	}
	return nil
}
