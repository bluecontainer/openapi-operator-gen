package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
)

// EvaluationResult contains the result of a CEL expression evaluation.
type EvaluationResult struct {
	Value    any     // The evaluated value
	RawValue ref.Val // The raw CEL value
	Error    error   // Any error that occurred
}

// Evaluate compiles and evaluates a CEL expression against the provided variables.
// The vars map should contain:
//   - "resources": []map[string]any - list of resource data
//   - "summary": map[string]int64 - summary counts
//   - kind-specific lists (e.g., "orders": []map[string]any)
func Evaluate(env *cel.Env, expression string, vars map[string]any) EvaluationResult {
	// Compile the expression
	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return EvaluationResult{Error: fmt.Errorf("failed to compile expression: %w", issues.Err())}
	}

	// Create the program
	prg, err := env.Program(ast)
	if err != nil {
		return EvaluationResult{Error: fmt.Errorf("failed to create program: %w", err)}
	}

	// Evaluate the expression
	result, _, err := prg.Eval(vars)
	if err != nil {
		return EvaluationResult{Error: fmt.Errorf("failed to evaluate expression: %w", err)}
	}

	return EvaluationResult{
		Value:    result.Value(),
		RawValue: result,
	}
}

// EvaluateToString evaluates a CEL expression and returns the result as a string.
func EvaluateToString(env *cel.Env, expression string, vars map[string]any) (string, error) {
	result := Evaluate(env, expression, vars)
	if result.Error != nil {
		return "", result.Error
	}
	return ValueToString(result.RawValue), nil
}

// ValueToString converts a CEL value to a string representation.
func ValueToString(val ref.Val) string {
	if val == nil {
		return "<nil>"
	}

	switch v := val.Value().(type) {
	case bool:
		return fmt.Sprintf("%t", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case uint64:
		return fmt.Sprintf("%d", v)
	case float64:
		// Format without trailing zeros for cleaner output
		if v == float64(int64(v)) {
			return fmt.Sprintf("%.0f", v)
		}
		return fmt.Sprintf("%g", v)
	case string:
		return v
	case []any:
		return fmt.Sprintf("%v", v)
	case map[string]any:
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// BuildVariables constructs a CEL variable map from resources, summary, and kind lists.
// This is a convenience function to build the vars map expected by Evaluate.
func BuildVariables(
	resources []map[string]any,
	summary map[string]int64,
	kindLists map[string][]map[string]any,
) map[string]any {
	vars := map[string]any{
		"resources": resources,
		"summary":   summary,
	}

	// Add kind-specific lists
	for kind, list := range kindLists {
		vars[kind] = list
	}

	return vars
}

// BuildSummary creates a summary map from individual counts.
func BuildSummary(total, synced, failed, pending int64) map[string]int64 {
	return map[string]int64{
		"total":   total,
		"synced":  synced,
		"failed":  failed,
		"pending": pending,
	}
}

// BuildSummaryWithSkipped creates a summary map including a skipped count.
func BuildSummaryWithSkipped(total, synced, failed, pending, skipped int64) map[string]int64 {
	return map[string]int64{
		"total":   total,
		"synced":  synced,
		"failed":  failed,
		"pending": pending,
		"skipped": skipped,
	}
}
