package bundle

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ResolveExpressions replaces ${resources.<id>.<path>} expressions in a spec
// with values from the provided status map.
//
// The statusMap keys are resource IDs, and values are maps that can be navigated
// using dot notation (e.g., "status.externalID").
func ResolveExpressions(spec map[string]interface{}, statusMap map[string]map[string]interface{}) (map[string]interface{}, error) {
	if spec == nil {
		return map[string]interface{}{}, nil
	}

	return resolveMap(spec, statusMap)
}

// ResolveExpressionsToBytes is like ResolveExpressions but returns JSON bytes.
func ResolveExpressionsToBytes(spec map[string]interface{}, statusMap map[string]map[string]interface{}) ([]byte, error) {
	resolved, err := ResolveExpressions(spec, statusMap)
	if err != nil {
		return nil, err
	}
	return json.Marshal(resolved)
}

// resolveMap recursively resolves CEL expressions in a map.
func resolveMap(data map[string]interface{}, statusMap map[string]map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range data {
		switch v := value.(type) {
		case string:
			// Check if it's a CEL expression: ${...}
			if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
				expr := v[2 : len(v)-1]
				resolved, err := evaluateSimpleExpression(expr, statusMap)
				if err != nil {
					return nil, fmt.Errorf("failed to evaluate %s: %w", key, err)
				}
				result[key] = resolved
			} else if strings.Contains(v, "${") {
				// Handle embedded expressions within strings
				resolved, err := resolveEmbeddedExpressions(v, statusMap)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve embedded expressions in %s: %w", key, err)
				}
				result[key] = resolved
			} else {
				result[key] = v
			}
		case map[string]interface{}:
			resolved, err := resolveMap(v, statusMap)
			if err != nil {
				return nil, err
			}
			result[key] = resolved
		case []interface{}:
			resolved, err := resolveSlice(v, statusMap)
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

// resolveSlice recursively resolves CEL expressions in a slice.
func resolveSlice(data []interface{}, statusMap map[string]map[string]interface{}) ([]interface{}, error) {
	result := make([]interface{}, len(data))

	for i, value := range data {
		switch v := value.(type) {
		case string:
			if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
				expr := v[2 : len(v)-1]
				resolved, err := evaluateSimpleExpression(expr, statusMap)
				if err != nil {
					return nil, err
				}
				result[i] = resolved
			} else if strings.Contains(v, "${") {
				resolved, err := resolveEmbeddedExpressions(v, statusMap)
				if err != nil {
					return nil, err
				}
				result[i] = resolved
			} else {
				result[i] = v
			}
		case map[string]interface{}:
			resolved, err := resolveMap(v, statusMap)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		case []interface{}:
			resolved, err := resolveSlice(v, statusMap)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		default:
			result[i] = v
		}
	}

	return result, nil
}

// resolveEmbeddedExpressions handles strings like "child-of-${resources.parent.status.externalID}"
func resolveEmbeddedExpressions(s string, statusMap map[string]map[string]interface{}) (string, error) {
	result := s

	for {
		start := strings.Index(result, "${")
		if start == -1 {
			break
		}

		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start

		expr := result[start+2 : end]
		resolved, err := evaluateSimpleExpression(expr, statusMap)
		if err != nil {
			// Leave unresolved expressions as-is for debugging
			break
		}

		// Convert resolved value to string
		var replacement string
		switch v := resolved.(type) {
		case string:
			replacement = v
		case nil:
			replacement = ""
		default:
			valueJSON, _ := json.Marshal(v)
			replacement = string(valueJSON)
		}

		result = result[:start] + replacement + result[end+1:]
	}

	return result, nil
}

// evaluateSimpleExpression evaluates a simple path expression like "resources.pet.status.externalID"
func evaluateSimpleExpression(expr string, statusMap map[string]map[string]interface{}) (interface{}, error) {
	// Handle resources.<id>.status.<field> pattern
	if strings.HasPrefix(expr, "resources.") {
		parts := strings.Split(expr[10:], ".")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid expression: %s", expr)
		}

		resourceID := parts[0]
		resource, ok := statusMap[resourceID]
		if !ok {
			return nil, fmt.Errorf("resource not found: %s", resourceID)
		}

		// Navigate the path
		return navigatePath(resource, parts[1:]), nil
	}

	return nil, fmt.Errorf("unsupported expression: %s", expr)
}

// navigatePath navigates a path through nested maps.
func navigatePath(data map[string]interface{}, parts []string) interface{} {
	var current interface{} = data

	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[part]
	}

	return current
}

// NavigatePath is the exported version of navigatePath for use by consumers.
func NavigatePath(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	return navigatePath(data, parts)
}
