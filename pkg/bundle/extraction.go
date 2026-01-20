package bundle

import (
	"encoding/json"
	"strings"
)

// FindResourceReferences finds all resource ID references in a string.
// Supports two patterns:
//   - ${resources.<id>...} - used in spec field interpolation
//   - resources.<id>... - used in CEL expressions (readyWhen, skipWhen)
//
// The includeBare parameter controls whether bare (non-${}) patterns are matched.
func FindResourceReferences(s string, includeBare bool) map[string]bool {
	deps := make(map[string]bool)

	// Pattern 1: ${resources.<id>.anything}
	idx := 0
	for {
		start := strings.Index(s[idx:], "${resources.")
		if start == -1 {
			break
		}
		start += idx + len("${resources.")

		// Find the end of the resource ID (next . or })
		end := start
		for end < len(s) && s[end] != '.' && s[end] != '}' {
			end++
		}

		if end > start {
			resourceID := s[start:end]
			if IsValidResourceID(resourceID) {
				deps[resourceID] = true
			}
		}
		idx = end
	}

	// Pattern 2: resources.<id>.anything (CEL expression without ${})
	// This handles cases like: resources.pet.status.state == 'Synced'
	if includeBare {
		idx = 0
		for {
			start := strings.Index(s[idx:], "resources.")
			if start == -1 {
				break
			}

			// Skip if this is part of ${resources. (already handled above)
			if start > 0 && idx+start > 0 && s[idx+start-1] == '{' {
				idx += start + len("resources.")
				continue
			}

			start += idx + len("resources.")

			// Find the end of the resource ID (next . or non-identifier char)
			end := start
			for end < len(s) && IsIdentChar(s[end]) {
				end++
			}

			if end > start {
				resourceID := s[start:end]
				if IsValidResourceID(resourceID) {
					deps[resourceID] = true
				}
			}
			idx = end
		}
	}

	return deps
}

// ExtractDependenciesFromBytes extracts resource IDs referenced in ${resources.<id>...}
// patterns from raw JSON/YAML bytes.
func ExtractDependenciesFromBytes(data []byte, includeBare bool) []string {
	refs := FindResourceReferences(string(data), includeBare)
	result := make([]string, 0, len(refs))
	for dep := range refs {
		result = append(result, dep)
	}
	return result
}

// ExtractDependenciesFromMap extracts resource IDs referenced in ${resources.<id>...}
// patterns from a map (typically a spec).
func ExtractDependenciesFromMap(data map[string]interface{}, includeBare bool) []string {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	return ExtractDependenciesFromBytes(jsonBytes, includeBare)
}

// ExtractDependenciesFromExpression extracts resource IDs from a CEL expression string.
// Looks for patterns like: resources.<id>.status.field or resources.<id>.spec.field
func ExtractDependenciesFromExpression(expr string) []string {
	refs := FindResourceReferences(expr, true) // Always include bare patterns for CEL
	result := make([]string, 0, len(refs))
	for dep := range refs {
		result = append(result, dep)
	}
	return result
}

// ExtractAllDependencies extracts all dependencies for a resource based on the given options.
func ExtractAllDependencies(res ResourceSpec, opts DependencyExtractorOptions) map[string]bool {
	allDeps := make(map[string]bool)

	// Add explicit dependencies
	if opts.IncludeExplicit {
		for _, dep := range res.DependsOn {
			allDeps[dep] = true
		}
	}

	// Extract implicit dependencies from spec
	if opts.IncludeSpecRefs && res.Spec != nil {
		specJSON, err := json.Marshal(res.Spec)
		if err == nil {
			refs := FindResourceReferences(string(specJSON), opts.IncludeBareRefs)
			for dep := range refs {
				allDeps[dep] = true
			}
		}
	}

	// Extract dependencies from conditions
	if opts.IncludeConditions {
		for _, condition := range res.ReadyWhen {
			refs := FindResourceReferences(condition, true) // Always include bare in CEL
			for dep := range refs {
				allDeps[dep] = true
			}
		}
		for _, condition := range res.SkipWhen {
			refs := FindResourceReferences(condition, true)
			for dep := range refs {
				allDeps[dep] = true
			}
		}
	}

	// Remove self-reference if any
	delete(allDeps, res.ID)

	return allDeps
}
