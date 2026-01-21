package cel

import (
	"testing"
)

func TestResourceState_IsHealthy(t *testing.T) {
	tests := []struct {
		state ResourceState
		want  bool
	}{
		{StateSynced, true},
		{StateQueried, true},
		{StateCompleted, true},
		{StateFailed, false},
		{StatePending, false},
		{StateSkipped, false},
		{ResourceState("Unknown"), false},
		{ResourceState(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.IsHealthy(); got != tt.want {
				t.Errorf("ResourceState(%q).IsHealthy() = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestResourceStateConstants(t *testing.T) {
	// Verify constant values match expected strings
	tests := []struct {
		state    ResourceState
		expected string
	}{
		{StateSynced, "Synced"},
		{StateFailed, "Failed"},
		{StatePending, "Pending"},
		{StateQueried, "Queried"},
		{StateCompleted, "Completed"},
		{StateSkipped, "Skipped"},
	}

	for _, tt := range tests {
		if string(tt.state) != tt.expected {
			t.Errorf("State constant = %q, want %q", tt.state, tt.expected)
		}
	}
}

func TestAggregationStrategyConstants(t *testing.T) {
	tests := []struct {
		strategy AggregationStrategy
		expected string
	}{
		{StrategyAllHealthy, "AllHealthy"},
		{StrategyAnyHealthy, "AnyHealthy"},
		{StrategyQuorum, "Quorum"},
	}

	for _, tt := range tests {
		if string(tt.strategy) != tt.expected {
			t.Errorf("Strategy constant = %q, want %q", tt.strategy, tt.expected)
		}
	}
}

func TestEvaluateStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy AggregationStrategy
		total    int64
		synced   int64
		failed   int64
		pending  int64
		want     ResourceState
	}{
		// AllHealthy tests
		{
			name:     "AllHealthy - all synced",
			strategy: StrategyAllHealthy,
			total:    5,
			synced:   5,
			failed:   0,
			pending:  0,
			want:     StateSynced,
		},
		{
			name:     "AllHealthy - some failed",
			strategy: StrategyAllHealthy,
			total:    5,
			synced:   3,
			failed:   2,
			pending:  0,
			want:     ResourceState("Degraded"),
		},
		{
			name:     "AllHealthy - some pending",
			strategy: StrategyAllHealthy,
			total:    5,
			synced:   3,
			failed:   0,
			pending:  2,
			want:     ResourceState("Degraded"),
		},
		{
			name:     "AllHealthy - empty",
			strategy: StrategyAllHealthy,
			total:    0,
			synced:   0,
			failed:   0,
			pending:  0,
			want:     StatePending,
		},

		// AnyHealthy tests
		{
			name:     "AnyHealthy - one synced",
			strategy: StrategyAnyHealthy,
			total:    5,
			synced:   1,
			failed:   4,
			pending:  0,
			want:     StateSynced,
		},
		{
			name:     "AnyHealthy - all synced",
			strategy: StrategyAnyHealthy,
			total:    5,
			synced:   5,
			failed:   0,
			pending:  0,
			want:     StateSynced,
		},
		{
			name:     "AnyHealthy - none synced, all failed",
			strategy: StrategyAnyHealthy,
			total:    5,
			synced:   0,
			failed:   5,
			pending:  0,
			want:     StateFailed,
		},
		{
			name:     "AnyHealthy - none synced, some pending",
			strategy: StrategyAnyHealthy,
			total:    5,
			synced:   0,
			failed:   3,
			pending:  2,
			want:     StateFailed,
		},
		{
			name:     "AnyHealthy - empty",
			strategy: StrategyAnyHealthy,
			total:    0,
			synced:   0,
			failed:   0,
			pending:  0,
			want:     StatePending,
		},

		// Quorum tests
		{
			name:     "Quorum - majority synced (3/5)",
			strategy: StrategyQuorum,
			total:    5,
			synced:   3,
			failed:   2,
			pending:  0,
			want:     StateSynced,
		},
		{
			name:     "Quorum - exactly half (2/4) - not enough",
			strategy: StrategyQuorum,
			total:    4,
			synced:   2,
			failed:   2,
			pending:  0,
			want:     ResourceState("Degraded"),
		},
		{
			name:     "Quorum - more than half (3/4)",
			strategy: StrategyQuorum,
			total:    4,
			synced:   3,
			failed:   1,
			pending:  0,
			want:     StateSynced,
		},
		{
			name:     "Quorum - all synced",
			strategy: StrategyQuorum,
			total:    5,
			synced:   5,
			failed:   0,
			pending:  0,
			want:     StateSynced,
		},
		{
			name:     "Quorum - none synced",
			strategy: StrategyQuorum,
			total:    5,
			synced:   0,
			failed:   5,
			pending:  0,
			want:     ResourceState("Degraded"),
		},
		{
			name:     "Quorum - empty",
			strategy: StrategyQuorum,
			total:    0,
			synced:   0,
			failed:   0,
			pending:  0,
			want:     StatePending,
		},

		// Default/unknown strategy tests
		{
			name:     "Unknown strategy - all synced (defaults to AllHealthy)",
			strategy: AggregationStrategy("Unknown"),
			total:    3,
			synced:   3,
			failed:   0,
			pending:  0,
			want:     StateSynced,
		},
		{
			name:     "Unknown strategy - partial sync (defaults to AllHealthy)",
			strategy: AggregationStrategy("Unknown"),
			total:    3,
			synced:   2,
			failed:   1,
			pending:  0,
			want:     ResourceState("Degraded"),
		},
		{
			name:     "Empty strategy - defaults to AllHealthy",
			strategy: AggregationStrategy(""),
			total:    3,
			synced:   3,
			failed:   0,
			pending:  0,
			want:     StateSynced,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateStrategy(tt.strategy, tt.total, tt.synced, tt.failed, tt.pending)
			if got != tt.want {
				t.Errorf("EvaluateStrategy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceData_ToMap(t *testing.T) {
	rd := &ResourceData{
		Kind: "Pet",
		Metadata: ResourceMeta{
			Name:      "fluffy",
			Namespace: "default",
			Labels: map[string]string{
				"app": "petstore",
			},
			Annotations: map[string]string{
				"note": "test annotation",
			},
		},
		Spec: map[string]any{
			"name":   "Fluffy",
			"status": "available",
		},
		Status: ResourceStatus{
			State:      "Synced",
			ExternalID: "pet-123",
			Message:    "Successfully synced",
		},
	}

	result := rd.ToMap()

	// Verify kind
	if result["kind"] != "Pet" {
		t.Errorf("ToMap().kind = %v, want Pet", result["kind"])
	}

	// Verify metadata
	metadata, ok := result["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("ToMap().metadata is not map[string]any")
	}
	if metadata["name"] != "fluffy" {
		t.Errorf("ToMap().metadata.name = %v, want fluffy", metadata["name"])
	}
	if metadata["namespace"] != "default" {
		t.Errorf("ToMap().metadata.namespace = %v, want default", metadata["namespace"])
	}

	// Verify labels
	labels, ok := metadata["labels"].(map[string]string)
	if !ok {
		t.Fatalf("ToMap().metadata.labels is not map[string]string")
	}
	if labels["app"] != "petstore" {
		t.Errorf("ToMap().metadata.labels.app = %v, want petstore", labels["app"])
	}

	// Verify status
	status, ok := result["status"].(map[string]any)
	if !ok {
		t.Fatalf("ToMap().status is not map[string]any")
	}
	if status["state"] != "Synced" {
		t.Errorf("ToMap().status.state = %v, want Synced", status["state"])
	}
	if status["externalID"] != "pet-123" {
		t.Errorf("ToMap().status.externalID = %v, want pet-123", status["externalID"])
	}

	// Verify spec
	spec, ok := result["spec"].(map[string]any)
	if !ok {
		t.Fatalf("ToMap().spec is not map[string]any")
	}
	if spec["name"] != "Fluffy" {
		t.Errorf("ToMap().spec.name = %v, want Fluffy", spec["name"])
	}
}

func TestResourceData_ToMap_NilSpec(t *testing.T) {
	rd := &ResourceData{
		Kind: "Pet",
		Metadata: ResourceMeta{
			Name:      "fluffy",
			Namespace: "default",
		},
		Spec: nil,
		Status: ResourceStatus{
			State: "Synced",
		},
	}

	result := rd.ToMap()

	// Spec should not be present when nil
	if _, ok := result["spec"]; ok {
		t.Error("ToMap() should not include spec when nil")
	}
}

func TestResourceData_ToMap_EmptyValues(t *testing.T) {
	rd := &ResourceData{
		Kind: "",
		Metadata: ResourceMeta{
			Name:      "",
			Namespace: "",
		},
		Status: ResourceStatus{
			State: "",
		},
	}

	result := rd.ToMap()

	// Should still have all keys, just with empty values
	if _, ok := result["kind"]; !ok {
		t.Error("ToMap() should have kind key")
	}
	if _, ok := result["metadata"]; !ok {
		t.Error("ToMap() should have metadata key")
	}
	if _, ok := result["status"]; !ok {
		t.Error("ToMap() should have status key")
	}
}

func TestObjectToMap(t *testing.T) {
	type TestStruct struct {
		Name   string `json:"name"`
		Count  int    `json:"count"`
		Active bool   `json:"active"`
	}

	obj := TestStruct{
		Name:   "test",
		Count:  42,
		Active: true,
	}

	result, err := ObjectToMap(obj)
	if err != nil {
		t.Fatalf("ObjectToMap() error = %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("ObjectToMap().name = %v, want test", result["name"])
	}
	// JSON numbers become float64
	if result["count"] != float64(42) {
		t.Errorf("ObjectToMap().count = %v (%T), want 42", result["count"], result["count"])
	}
	if result["active"] != true {
		t.Errorf("ObjectToMap().active = %v, want true", result["active"])
	}
}

func TestObjectToMap_NestedStruct(t *testing.T) {
	type Inner struct {
		Value string `json:"value"`
	}
	type Outer struct {
		Inner Inner `json:"inner"`
	}

	obj := Outer{
		Inner: Inner{Value: "nested"},
	}

	result, err := ObjectToMap(obj)
	if err != nil {
		t.Fatalf("ObjectToMap() error = %v", err)
	}

	inner, ok := result["inner"].(map[string]any)
	if !ok {
		t.Fatalf("ObjectToMap().inner is not map[string]any: %T", result["inner"])
	}
	if inner["value"] != "nested" {
		t.Errorf("ObjectToMap().inner.value = %v, want nested", inner["value"])
	}
}

func TestObjectToMap_Slice(t *testing.T) {
	obj := struct {
		Items []string `json:"items"`
	}{
		Items: []string{"a", "b", "c"},
	}

	result, err := ObjectToMap(obj)
	if err != nil {
		t.Fatalf("ObjectToMap() error = %v", err)
	}

	items, ok := result["items"].([]any)
	if !ok {
		t.Fatalf("ObjectToMap().items is not []any: %T", result["items"])
	}
	if len(items) != 3 {
		t.Errorf("len(ObjectToMap().items) = %d, want 3", len(items))
	}
}

func TestObjectToMap_InvalidInput(t *testing.T) {
	// Functions can't be marshaled to JSON
	_, err := ObjectToMap(func() {})
	if err == nil {
		t.Error("ObjectToMap() should return error for unmarshallable input")
	}
}

func TestObjectToMap_NilInput(t *testing.T) {
	result, err := ObjectToMap(nil)
	if err != nil {
		t.Fatalf("ObjectToMap() error = %v", err)
	}
	if result != nil {
		t.Errorf("ObjectToMap(nil) = %v, want nil", result)
	}
}

func TestKindToVariableName(t *testing.T) {
	tests := []struct {
		kind string
		want string
	}{
		{"Order", "orders"},
		{"Pet", "pets"},
		{"User", "users"},
		{"PetFindbystatusQuery", "petfindbystatusqueries"}, // Query -> queries
		{"StoreInventoryQuery", "storeinventoryqueries"},   // Query -> queries
		{"OrderAction", "orderactions"},
		{"A", "as"},
		{"", "s"},
		{"UPPERCASE", "uppercases"},
		{"lowercase", "lowercases"},
		{"MixedCase", "mixedcases"},
		// Test proper pluralization rules
		{"Policy", "policies"}, // y -> ies (consonant + y)
		{"Key", "keys"},        // y -> s (vowel + y)
		{"Class", "classes"},   // s -> es
		{"Box", "boxes"},       // x -> es
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			got := KindToVariableName(tt.kind)
			if got != tt.want {
				t.Errorf("KindToVariableName(%q) = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}

func TestComputedValue(t *testing.T) {
	cv := ComputedValue{
		Name:  "total_orders",
		Value: 42,
	}

	if cv.Name != "total_orders" {
		t.Errorf("ComputedValue.Name = %v, want total_orders", cv.Name)
	}
	if cv.Value != 42 {
		t.Errorf("ComputedValue.Value = %v, want 42", cv.Value)
	}
}

func TestDerivedValueDefinition(t *testing.T) {
	dvd := DerivedValueDefinition{
		Name:       "sync_percentage",
		Expression: "double(summary.synced) / double(summary.total) * 100.0",
	}

	if dvd.Name != "sync_percentage" {
		t.Errorf("DerivedValueDefinition.Name = %v, want sync_percentage", dvd.Name)
	}
	if dvd.Expression != "double(summary.synced) / double(summary.total) * 100.0" {
		t.Errorf("DerivedValueDefinition.Expression = %v, want expression", dvd.Expression)
	}
}

func TestResourceData_ToMap_UsableInCEL(t *testing.T) {
	// Test that ToMap output can be used in CEL evaluation
	rd := &ResourceData{
		Kind: "Pet",
		Metadata: ResourceMeta{
			Name:      "fluffy",
			Namespace: "default",
		},
		Status: ResourceStatus{
			State:      "Synced",
			ExternalID: "pet-123",
		},
	}

	env, err := NewEnvironment([]string{"pets"})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	resourceMap := rd.ToMap()
	vars := map[string]any{
		"resources": []map[string]any{resourceMap},
		"summary":   map[string]int64{"total": 1, "synced": 1},
		"pets":      []map[string]any{resourceMap},
	}

	// Test accessing nested values
	tests := []struct {
		expr string
		want any
	}{
		{"resources[0].kind", "Pet"},
		{"resources[0].metadata.name", "fluffy"},
		{"resources[0].status.state", "Synced"},
		{"resources[0].status.externalID", "pet-123"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result := Evaluate(env, tt.expr, vars)
			if result.Error != nil {
				t.Errorf("Evaluate(%q) error = %v", tt.expr, result.Error)
				return
			}
			if result.Value != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v", tt.expr, result.Value, tt.want)
			}
		})
	}
}
