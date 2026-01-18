package cel

import (
	"testing"

	"github.com/google/cel-go/cel"
)

func TestNewEnvironment(t *testing.T) {
	tests := []struct {
		name      string
		kindNames []string
		wantErr   bool
	}{
		{
			name:      "empty kind names",
			kindNames: []string{},
			wantErr:   false,
		},
		{
			name:      "single kind name",
			kindNames: []string{"orders"},
			wantErr:   false,
		},
		{
			name:      "multiple kind names",
			kindNames: []string{"orders", "pets", "users"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := NewEnvironment(tt.kindNames)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewEnvironment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && env == nil {
				t.Error("NewEnvironment() returned nil environment")
			}
		})
	}
}

func TestNewEnvironment_StandardVariables(t *testing.T) {
	env, err := NewEnvironment([]string{"pets"})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	// Test that standard variables are available
	tests := []struct {
		name       string
		expression string
		vars       map[string]any
		wantErr    bool
	}{
		{
			name:       "resources variable",
			expression: "size(resources)",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 0},
				"pets":      []map[string]any{},
			},
			wantErr: false,
		},
		{
			name:       "summary variable",
			expression: "summary.total",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 5},
				"pets":      []map[string]any{},
			},
			wantErr: false,
		},
		{
			name:       "kind-specific variable",
			expression: "size(pets)",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 0},
				"pets":      []map[string]any{{"name": "fido"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, issues := env.Compile(tt.expression)
			if issues != nil && issues.Err() != nil {
				if !tt.wantErr {
					t.Errorf("Compile() error = %v", issues.Err())
				}
				return
			}

			prg, err := env.Program(ast)
			if err != nil {
				t.Errorf("Program() error = %v", err)
				return
			}

			_, _, err = prg.Eval(tt.vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("Eval() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewEnvironment_AggregateFunctions(t *testing.T) {
	env, err := NewEnvironment([]string{})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	// Test that aggregate functions are available
	tests := []struct {
		name       string
		expression string
		vars       map[string]any
		want       float64
	}{
		{
			name:       "sum function",
			expression: "sum([1, 2, 3, 4, 5])",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 0},
			},
			want: 15,
		},
		{
			name:       "max function",
			expression: "max([1, 5, 3, 9, 2])",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 0},
			},
			want: 9,
		},
		{
			name:       "min function",
			expression: "min([5, 3, 1, 9, 2])",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 0},
			},
			want: 1,
		},
		{
			name:       "avg function",
			expression: "avg([2, 4, 6, 8])",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 0},
			},
			want: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Evaluate(env, tt.expression, tt.vars)
			if result.Error != nil {
				t.Errorf("Evaluate() error = %v", result.Error)
				return
			}

			got, ok := result.Value.(float64)
			if !ok {
				t.Errorf("Evaluate() result is not float64: %T", result.Value)
				return
			}

			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewEnvironmentWithOptions(t *testing.T) {
	// Test with additional custom variable
	customVar := cel.Variable("customVar", cel.StringType)

	env, err := NewEnvironmentWithOptions([]string{"orders"}, customVar)
	if err != nil {
		t.Fatalf("NewEnvironmentWithOptions() error = %v", err)
	}

	// Verify custom variable is available
	vars := map[string]any{
		"resources": []map[string]any{},
		"summary":   map[string]int64{"total": 0},
		"orders":    []map[string]any{},
		"customVar": "test-value",
	}

	result := Evaluate(env, "customVar", vars)
	if result.Error != nil {
		t.Errorf("Evaluate() error = %v", result.Error)
		return
	}

	if result.Value != "test-value" {
		t.Errorf("Evaluate() = %v, want %v", result.Value, "test-value")
	}
}

func TestNewEnvironmentWithOptions_PreservesStandardFeatures(t *testing.T) {
	env, err := NewEnvironmentWithOptions([]string{"pets"})
	if err != nil {
		t.Fatalf("NewEnvironmentWithOptions() error = %v", err)
	}

	vars := map[string]any{
		"resources": []map[string]any{
			{"name": "pet1"},
			{"name": "pet2"},
		},
		"summary": map[string]int64{"total": 2, "synced": 2},
		"pets":    []map[string]any{},
	}

	// Test standard variables work
	result := Evaluate(env, "size(resources)", vars)
	if result.Error != nil {
		t.Errorf("Evaluate() error = %v", result.Error)
		return
	}

	got, ok := result.Value.(int64)
	if !ok {
		t.Errorf("Evaluate() result is not int64: %T", result.Value)
		return
	}

	if got != 2 {
		t.Errorf("Evaluate() = %v, want %v", got, 2)
	}

	// Test aggregate functions work
	result = Evaluate(env, "sum([1, 2, 3])", vars)
	if result.Error != nil {
		t.Errorf("Evaluate() sum error = %v", result.Error)
	}
}
