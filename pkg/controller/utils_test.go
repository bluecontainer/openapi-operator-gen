package controller

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValuesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        interface{}
		b        interface{}
		expected bool
	}{
		{
			name:     "equal strings",
			a:        "hello",
			b:        "hello",
			expected: true,
		},
		{
			name:     "different strings",
			a:        "hello",
			b:        "world",
			expected: false,
		},
		{
			name:     "equal timestamps different format",
			a:        "2026-01-15T10:00:00Z",
			b:        "2026-01-15T10:00:00.000Z",
			expected: true,
		},
		{
			name:     "equal timestamps with timezone",
			a:        "2026-01-15T10:00:00Z",
			b:        "2026-01-15T10:00:00+00:00",
			expected: true,
		},
		{
			name:     "different timestamps",
			a:        "2026-01-15T10:00:00Z",
			b:        "2026-01-15T11:00:00Z",
			expected: false,
		},
		{
			name:     "equal integers",
			a:        42,
			b:        42,
			expected: true,
		},
		{
			name:     "int vs float64 equal",
			a:        42,
			b:        float64(42),
			expected: true,
		},
		{
			name:     "int32 vs int64 equal",
			a:        int32(42),
			b:        int64(42),
			expected: true,
		},
		{
			name:     "different integers",
			a:        42,
			b:        43,
			expected: false,
		},
		{
			name:     "equal booleans",
			a:        true,
			b:        true,
			expected: true,
		},
		{
			name:     "different booleans",
			a:        true,
			b:        false,
			expected: false,
		},
		{
			name:     "nil values",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "one nil one not",
			a:        nil,
			b:        "hello",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValuesEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("ValuesEqual(%v, %v) = %v, expected %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestValuesEqual_JSONNumber(t *testing.T) {
	// Test with json.Number which is common after JSON unmarshaling
	jsonData := `{"value": 42}`
	var result map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(jsonData))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	jsonNum := result["value"]

	if !ValuesEqual(jsonNum, 42) {
		t.Errorf("ValuesEqual(json.Number(42), 42) should be true")
	}

	if !ValuesEqual(jsonNum, float64(42)) {
		t.Errorf("ValuesEqual(json.Number(42), float64(42)) should be true")
	}

	if ValuesEqual(jsonNum, 43) {
		t.Errorf("ValuesEqual(json.Number(42), 43) should be false")
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
		ok       bool
	}{
		{"float64", float64(3.14), 3.14, true},
		{"float32", float32(3.14), float64(float32(3.14)), true},
		{"int", 42, 42.0, true},
		{"int32", int32(42), 42.0, true},
		{"int64", int64(42), 42.0, true},
		{"string", "42", 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ToFloat64(tt.input)
			if ok != tt.ok {
				t.Errorf("ToFloat64(%v) ok = %v, expected %v", tt.input, ok, tt.ok)
			}
			if ok && result != tt.expected {
				t.Errorf("ToFloat64(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetExternalIDIfPresent(t *testing.T) {
	// Test struct with ExternalID
	type StatusWithID struct {
		ExternalID string
		State      string
	}
	type ResourceWithID struct {
		Status StatusWithID
	}

	resourceWithID := ResourceWithID{
		Status: StatusWithID{
			ExternalID: "ext-123",
			State:      "Synced",
		},
	}

	if id := GetExternalIDIfPresent(&resourceWithID); id != "ext-123" {
		t.Errorf("GetExternalIDIfPresent() = %q, expected %q", id, "ext-123")
	}

	// Test struct without ExternalID
	type StatusWithoutID struct {
		State string
	}
	type ResourceWithoutID struct {
		Status StatusWithoutID
	}

	resourceWithoutID := ResourceWithoutID{
		Status: StatusWithoutID{
			State: "Queried",
		},
	}

	if id := GetExternalIDIfPresent(&resourceWithoutID); id != "" {
		t.Errorf("GetExternalIDIfPresent() = %q, expected empty string", id)
	}

	// Test non-struct
	if id := GetExternalIDIfPresent("not a struct"); id != "" {
		t.Errorf("GetExternalIDIfPresent(string) = %q, expected empty string", id)
	}

	// Test nil
	if id := GetExternalIDIfPresent(nil); id != "" {
		t.Errorf("GetExternalIDIfPresent(nil) = %q, expected empty string", id)
	}
}
