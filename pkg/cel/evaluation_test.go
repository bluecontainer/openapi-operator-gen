package cel

import (
	"testing"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

func TestEvaluate(t *testing.T) {
	env, err := NewEnvironment([]string{"orders"})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	tests := []struct {
		name       string
		expression string
		vars       map[string]any
		want       any
		wantErr    bool
	}{
		{
			name:       "simple integer expression",
			expression: "1 + 2",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 0},
				"orders":    []map[string]any{},
			},
			want:    int64(3),
			wantErr: false,
		},
		{
			name:       "access summary total",
			expression: "summary.total",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 10, "synced": 5},
				"orders":    []map[string]any{},
			},
			want:    int64(10),
			wantErr: false,
		},
		{
			name:       "conditional expression",
			expression: "summary.synced == summary.total ? 'all synced' : 'partial'",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 5, "synced": 5},
				"orders":    []map[string]any{},
			},
			want:    "all synced",
			wantErr: false,
		},
		{
			name:       "list size",
			expression: "size(resources)",
			vars: map[string]any{
				"resources": []map[string]any{
					{"name": "res1"},
					{"name": "res2"},
				},
				"summary": map[string]int64{"total": 2},
				"orders":  []map[string]any{},
			},
			want:    int64(2),
			wantErr: false,
		},
		{
			name:       "filter resources",
			expression: "resources.filter(r, r.status.state == 'Synced').size()",
			vars: map[string]any{
				"resources": []map[string]any{
					{"name": "res1", "status": map[string]any{"state": "Synced"}},
					{"name": "res2", "status": map[string]any{"state": "Failed"}},
					{"name": "res3", "status": map[string]any{"state": "Synced"}},
				},
				"summary": map[string]int64{"total": 3},
				"orders":  []map[string]any{},
			},
			want:    int64(2),
			wantErr: false,
		},
		{
			name:       "map over resources - size",
			expression: "resources.map(r, r.name).size()",
			vars: map[string]any{
				"resources": []map[string]any{
					{"name": "res1"},
					{"name": "res2"},
				},
				"summary": map[string]int64{"total": 2},
				"orders":  []map[string]any{},
			},
			want:    int64(2),
			wantErr: false,
		},
		{
			name:       "invalid expression - compile error",
			expression: "invalid syntax !!!",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 0},
				"orders":    []map[string]any{},
			},
			wantErr: true,
		},
		{
			name:       "invalid expression - undefined variable",
			expression: "undefinedVar",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 0},
				"orders":    []map[string]any{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Evaluate(env, tt.expression, tt.vars)

			if (result.Error != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", result.Error, tt.wantErr)
				return
			}

			if !tt.wantErr && result.Value != tt.want {
				t.Errorf("Evaluate() = %v (%T), want %v (%T)", result.Value, result.Value, tt.want, tt.want)
			}
		})
	}
}

func TestEvaluate_RawValue(t *testing.T) {
	env, err := NewEnvironment([]string{})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	vars := map[string]any{
		"resources": []map[string]any{},
		"summary":   map[string]int64{"total": 0},
	}

	result := Evaluate(env, "42", vars)
	if result.Error != nil {
		t.Fatalf("Evaluate() error = %v", result.Error)
	}

	if result.RawValue == nil {
		t.Error("Evaluate() RawValue is nil")
	}
}

func TestEvaluateToString(t *testing.T) {
	env, err := NewEnvironment([]string{})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	tests := []struct {
		name       string
		expression string
		vars       map[string]any
		want       string
		wantErr    bool
	}{
		{
			name:       "integer to string",
			expression: "42",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 0},
			},
			want:    "42",
			wantErr: false,
		},
		{
			name:       "float to string",
			expression: "3.14159",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 0},
			},
			want:    "3.14159",
			wantErr: false,
		},
		{
			name:       "boolean to string",
			expression: "true",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 0},
			},
			want:    "true",
			wantErr: false,
		},
		{
			name:       "string literal",
			expression: "'hello world'",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 0},
			},
			want:    "hello world",
			wantErr: false,
		},
		{
			name:       "summary expression",
			expression: "summary.total + summary.synced",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 10, "synced": 5},
			},
			want:    "15",
			wantErr: false,
		},
		{
			name:       "invalid expression returns error",
			expression: "invalid!!!",
			vars: map[string]any{
				"resources": []map[string]any{},
				"summary":   map[string]int64{"total": 0},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvaluateToString(env, tt.expression, tt.vars)

			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateToString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("EvaluateToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValueToString(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want string
	}{
		{
			name: "nil value",
			val:  nil,
			want: "<nil>",
		},
		{
			name: "boolean true",
			val:  true,
			want: "true",
		},
		{
			name: "boolean false",
			val:  false,
			want: "false",
		},
		{
			name: "int64 positive",
			val:  int64(42),
			want: "42",
		},
		{
			name: "int64 negative",
			val:  int64(-100),
			want: "-100",
		},
		{
			name: "uint64",
			val:  uint64(99),
			want: "99",
		},
		{
			name: "float64 whole number",
			val:  float64(100),
			want: "100",
		},
		{
			name: "float64 decimal",
			val:  float64(3.14159),
			want: "3.14159",
		},
		{
			name: "string",
			val:  "hello",
			want: "hello",
		},
		{
			name: "slice",
			val:  []any{1, 2, 3},
			want: "[1 2 3]",
		},
		{
			name: "map",
			val:  map[string]any{"key": "value"},
			want: "map[key:value]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var celVal ref.Val
			if tt.val == nil {
				got := ValueToString(nil)
				if got != tt.want {
					t.Errorf("ValueToString() = %v, want %v", got, tt.want)
				}
				return
			}

			// Create a CEL value from the native value
			switch v := tt.val.(type) {
			case bool:
				celVal = types.Bool(v)
			case int64:
				celVal = types.Int(v)
			case uint64:
				celVal = types.Uint(v)
			case float64:
				celVal = types.Double(v)
			case string:
				celVal = types.String(v)
			case []any:
				// For slices, we need to use the DefaultTypeAdapter
				celVal = types.DefaultTypeAdapter.NativeToValue(v)
			case map[string]any:
				celVal = types.DefaultTypeAdapter.NativeToValue(v)
			default:
				t.Fatalf("unsupported type: %T", v)
			}

			got := ValueToString(celVal)
			if got != tt.want {
				t.Errorf("ValueToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildVariables(t *testing.T) {
	resources := []map[string]any{
		{"name": "res1", "kind": "Order"},
		{"name": "res2", "kind": "Pet"},
	}

	summary := map[string]int64{
		"total":   10,
		"synced":  5,
		"failed":  2,
		"pending": 3,
	}

	kindLists := map[string][]map[string]any{
		"orders": {{"name": "order1"}},
		"pets":   {{"name": "pet1"}, {"name": "pet2"}},
	}

	vars := BuildVariables(resources, summary, kindLists)

	// Verify resources
	gotResources, ok := vars["resources"].([]map[string]any)
	if !ok {
		t.Fatalf("vars[resources] is not []map[string]any: %T", vars["resources"])
	}
	if len(gotResources) != 2 {
		t.Errorf("len(vars[resources]) = %d, want 2", len(gotResources))
	}

	// Verify summary
	gotSummary, ok := vars["summary"].(map[string]int64)
	if !ok {
		t.Fatalf("vars[summary] is not map[string]int64: %T", vars["summary"])
	}
	if gotSummary["total"] != 10 {
		t.Errorf("vars[summary][total] = %d, want 10", gotSummary["total"])
	}

	// Verify kind lists
	gotOrders, ok := vars["orders"].([]map[string]any)
	if !ok {
		t.Fatalf("vars[orders] is not []map[string]any: %T", vars["orders"])
	}
	if len(gotOrders) != 1 {
		t.Errorf("len(vars[orders]) = %d, want 1", len(gotOrders))
	}

	gotPets, ok := vars["pets"].([]map[string]any)
	if !ok {
		t.Fatalf("vars[pets] is not []map[string]any: %T", vars["pets"])
	}
	if len(gotPets) != 2 {
		t.Errorf("len(vars[pets]) = %d, want 2", len(gotPets))
	}
}

func TestBuildVariables_EmptyInputs(t *testing.T) {
	vars := BuildVariables(nil, nil, nil)

	if _, ok := vars["resources"]; !ok {
		t.Error("vars should have 'resources' key")
	}
	if _, ok := vars["summary"]; !ok {
		t.Error("vars should have 'summary' key")
	}
}

func TestBuildSummary(t *testing.T) {
	summary := BuildSummary(10, 5, 3, 2)

	if summary["total"] != 10 {
		t.Errorf("summary[total] = %d, want 10", summary["total"])
	}
	if summary["synced"] != 5 {
		t.Errorf("summary[synced] = %d, want 5", summary["synced"])
	}
	if summary["failed"] != 3 {
		t.Errorf("summary[failed] = %d, want 3", summary["failed"])
	}
	if summary["pending"] != 2 {
		t.Errorf("summary[pending] = %d, want 2", summary["pending"])
	}

	// Verify skipped is not present in basic summary
	if _, ok := summary["skipped"]; ok {
		t.Error("basic summary should not have 'skipped' key")
	}
}

func TestBuildSummaryWithSkipped(t *testing.T) {
	summary := BuildSummaryWithSkipped(10, 5, 2, 2, 1)

	if summary["total"] != 10 {
		t.Errorf("summary[total] = %d, want 10", summary["total"])
	}
	if summary["synced"] != 5 {
		t.Errorf("summary[synced] = %d, want 5", summary["synced"])
	}
	if summary["failed"] != 2 {
		t.Errorf("summary[failed] = %d, want 2", summary["failed"])
	}
	if summary["pending"] != 2 {
		t.Errorf("summary[pending] = %d, want 2", summary["pending"])
	}
	if summary["skipped"] != 1 {
		t.Errorf("summary[skipped] = %d, want 1", summary["skipped"])
	}
}

func TestBuildSummary_ZeroValues(t *testing.T) {
	summary := BuildSummary(0, 0, 0, 0)

	if summary["total"] != 0 {
		t.Errorf("summary[total] = %d, want 0", summary["total"])
	}
	if summary["synced"] != 0 {
		t.Errorf("summary[synced] = %d, want 0", summary["synced"])
	}
	if summary["failed"] != 0 {
		t.Errorf("summary[failed] = %d, want 0", summary["failed"])
	}
	if summary["pending"] != 0 {
		t.Errorf("summary[pending] = %d, want 0", summary["pending"])
	}
}

func TestResourceKey(t *testing.T) {
	tests := []struct {
		kind string
		name string
		want string
	}{
		{"Pet", "fluffy", "pet_fluffy"},
		{"Order", "sample", "order_sample"},
		{"StoreInventoryQuery", "test", "storeinventoryquery_test"},
		{"UPPERCASE", "name", "uppercase_name"},
		{"mixedCase", "item", "mixedcase_item"},
		// Names with hyphens should have them replaced with underscores
		{"Order", "order-001", "order_order_001"},
		{"Pet", "my-pet-name", "pet_my_pet_name"},
		{"StoreInventoryQuery", "store-query-1", "storeinventoryquery_store_query_1"},
	}

	for _, tt := range tests {
		t.Run(tt.kind+"-"+tt.name, func(t *testing.T) {
			got := ResourceKey(tt.kind, tt.name)
			if got != tt.want {
				t.Errorf("ResourceKey(%q, %q) = %q, want %q", tt.kind, tt.name, got, tt.want)
			}
		})
	}
}

func TestResourceKeyFromData(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want string
	}{
		{
			name: "valid data",
			data: map[string]any{
				"kind":     "Pet",
				"metadata": map[string]any{"name": "fluffy"},
			},
			want: "pet_fluffy",
		},
		{
			name: "missing kind",
			data: map[string]any{
				"metadata": map[string]any{"name": "fluffy"},
			},
			want: "",
		},
		{
			name: "missing metadata",
			data: map[string]any{
				"kind": "Pet",
			},
			want: "",
		},
		{
			name: "missing name in metadata",
			data: map[string]any{
				"kind":     "Pet",
				"metadata": map[string]any{},
			},
			want: "",
		},
		{
			name: "empty kind",
			data: map[string]any{
				"kind":     "",
				"metadata": map[string]any{"name": "fluffy"},
			},
			want: "",
		},
		{
			name: "empty name",
			data: map[string]any{
				"kind":     "Pet",
				"metadata": map[string]any{"name": ""},
			},
			want: "",
		},
		{
			name: "kind is not string",
			data: map[string]any{
				"kind":     123,
				"metadata": map[string]any{"name": "fluffy"},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResourceKeyFromData(tt.data)
			if got != tt.want {
				t.Errorf("ResourceKeyFromData() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCollectResourceKeys(t *testing.T) {
	resources := []map[string]any{
		{
			"kind":     "Pet",
			"metadata": map[string]any{"name": "fluffy"},
		},
		{
			"kind":     "Pet",
			"metadata": map[string]any{"name": "buddy"},
		},
		{
			"kind":     "Order",
			"metadata": map[string]any{"name": "sample"},
		},
		{
			"kind":     "Pet",
			"metadata": map[string]any{"name": "fluffy"}, // Duplicate should be ignored
		},
		{
			"kind": "Invalid", // Missing metadata should be ignored
		},
	}

	keys := CollectResourceKeys(resources)

	// Should have 3 unique keys
	if len(keys) != 3 {
		t.Errorf("CollectResourceKeys() returned %d keys, want 3", len(keys))
	}

	// Check that expected keys are present
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}

	expectedKeys := []string{"pet_fluffy", "pet_buddy", "order_sample"}
	for _, expected := range expectedKeys {
		if !keySet[expected] {
			t.Errorf("CollectResourceKeys() missing expected key %q", expected)
		}
	}
}

func TestCollectResourceKeys_Empty(t *testing.T) {
	keys := CollectResourceKeys(nil)
	if len(keys) != 0 {
		t.Errorf("CollectResourceKeys(nil) returned %d keys, want 0", len(keys))
	}

	keys = CollectResourceKeys([]map[string]any{})
	if len(keys) != 0 {
		t.Errorf("CollectResourceKeys([]) returned %d keys, want 0", len(keys))
	}
}

func TestBuildVariablesWithResources(t *testing.T) {
	resources := []map[string]any{
		{
			"kind":     "Pet",
			"metadata": map[string]any{"name": "fluffy", "namespace": "default"},
			"status":   map[string]any{"state": "Synced"},
		},
		{
			"kind":     "Order",
			"metadata": map[string]any{"name": "sample", "namespace": "default"},
			"status":   map[string]any{"state": "Failed"},
		},
	}

	summary := map[string]int64{"total": 2, "synced": 1, "failed": 1, "pending": 0}

	kindLists := map[string][]map[string]any{
		"pets":   {resources[0]},
		"orders": {resources[1]},
	}

	vars := BuildVariablesWithResources(resources, summary, kindLists)

	// Verify standard variables are present
	if _, ok := vars["resources"]; !ok {
		t.Error("vars missing 'resources'")
	}
	if _, ok := vars["summary"]; !ok {
		t.Error("vars missing 'summary'")
	}
	if _, ok := vars["pets"]; !ok {
		t.Error("vars missing 'pets'")
	}
	if _, ok := vars["orders"]; !ok {
		t.Error("vars missing 'orders'")
	}

	// Verify resource-specific variables are present
	petFluffy, ok := vars["pet_fluffy"].(map[string]any)
	if !ok {
		t.Error("vars missing 'pet_fluffy' or wrong type")
	} else {
		if petFluffy["kind"] != "Pet" {
			t.Errorf("pet_fluffy.kind = %v, want 'Pet'", petFluffy["kind"])
		}
	}

	orderSample, ok := vars["order_sample"].(map[string]any)
	if !ok {
		t.Error("vars missing 'order_sample' or wrong type")
	} else {
		if orderSample["kind"] != "Order" {
			t.Errorf("order_sample.kind = %v, want 'Order'", orderSample["kind"])
		}
	}
}

func TestNewEnvironmentWithResources(t *testing.T) {
	kindNames := []string{"pets", "orders"}
	resourceKeys := []string{"pet_fluffy", "order_sample"}

	env, err := NewEnvironmentWithResources(kindNames, resourceKeys)
	if err != nil {
		t.Fatalf("NewEnvironmentWithResources() error = %v", err)
	}

	// Test that the environment can compile expressions using resource-specific variables
	tests := []struct {
		name       string
		expression string
		vars       map[string]any
		want       any
	}{
		{
			name:       "access pet_fluffy status",
			expression: `pet_fluffy.status.state`,
			want:       "Synced",
		},
		{
			name:       "access order_sample status",
			expression: `order_sample.status.state`,
			want:       "Failed",
		},
		{
			name:       "compare resource-specific values",
			expression: `pet_fluffy.status.state == "Synced"`,
			want:       true,
		},
	}

	baseVars := map[string]any{
		"resources": []map[string]any{},
		"summary":   map[string]int64{"total": 2, "synced": 1, "failed": 1},
		"pets":      []map[string]any{},
		"orders":    []map[string]any{},
		"pet_fluffy": map[string]any{
			"kind":     "Pet",
			"metadata": map[string]any{"name": "fluffy"},
			"status":   map[string]any{"state": "Synced"},
		},
		"order_sample": map[string]any{
			"kind":     "Order",
			"metadata": map[string]any{"name": "sample"},
			"status":   map[string]any{"state": "Failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Evaluate(env, tt.expression, baseVars)
			if result.Error != nil {
				t.Errorf("Evaluate() error = %v", result.Error)
				return
			}
			if result.Value != tt.want {
				t.Errorf("Evaluate() = %v, want %v", result.Value, tt.want)
			}
		})
	}
}

func TestEvaluate_ComplexExpressions(t *testing.T) {
	env, err := NewEnvironment([]string{"orders", "pets"})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	vars := map[string]any{
		"resources": []map[string]any{
			{"kind": "Order", "metadata": map[string]any{"name": "order1"}, "status": map[string]any{"state": "Synced"}},
			{"kind": "Order", "metadata": map[string]any{"name": "order2"}, "status": map[string]any{"state": "Failed"}},
			{"kind": "Pet", "metadata": map[string]any{"name": "pet1"}, "status": map[string]any{"state": "Synced"}},
		},
		"summary": map[string]int64{"total": 3, "synced": 2, "failed": 1, "pending": 0},
		"orders": []map[string]any{
			{"kind": "Order", "metadata": map[string]any{"name": "order1"}, "status": map[string]any{"state": "Synced"}},
			{"kind": "Order", "metadata": map[string]any{"name": "order2"}, "status": map[string]any{"state": "Failed"}},
		},
		"pets": []map[string]any{
			{"kind": "Pet", "metadata": map[string]any{"name": "pet1"}, "status": map[string]any{"state": "Synced"}},
		},
	}

	tests := []struct {
		name       string
		expression string
		want       any
	}{
		{
			name:       "count synced resources",
			expression: "resources.filter(r, r.status.state == 'Synced').size()",
			want:       int64(2),
		},
		{
			name:       "check if all synced",
			expression: "summary.synced == summary.total",
			want:       false,
		},
		{
			name:       "calculate sync percentage",
			expression: "double(summary.synced) / double(summary.total) * 100.0",
			want:       float64(200.0 / 3.0), // ~66.67
		},
		{
			name:       "use kind-specific list",
			expression: "size(orders)",
			want:       int64(2),
		},
		{
			name:       "filter by kind",
			expression: "resources.filter(r, r.kind == 'Pet').size()",
			want:       int64(1),
		},
		{
			name:       "exists check",
			expression: "resources.exists(r, r.status.state == 'Failed')",
			want:       true,
		},
		{
			name:       "all check - false case",
			expression: "resources.all(r, r.status.state == 'Synced')",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Evaluate(env, tt.expression, vars)
			if result.Error != nil {
				t.Errorf("Evaluate() error = %v", result.Error)
				return
			}

			// Handle float comparison with tolerance
			if wantFloat, ok := tt.want.(float64); ok {
				gotFloat, ok := result.Value.(float64)
				if !ok {
					t.Errorf("Evaluate() result is not float64: %T", result.Value)
					return
				}
				diff := gotFloat - wantFloat
				if diff < -0.001 || diff > 0.001 {
					t.Errorf("Evaluate() = %v, want %v", gotFloat, wantFloat)
				}
			} else if result.Value != tt.want {
				t.Errorf("Evaluate() = %v (%T), want %v (%T)", result.Value, result.Value, tt.want, tt.want)
			}
		})
	}
}
