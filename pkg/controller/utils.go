// Package controller provides shared utility functions for generated controllers.
package controller

import (
	"encoding/json"
	"reflect"
	"time"
)

// ValuesEqual compares two values for equality, handling special cases like timestamps,
// numeric type mismatches, and nested maps/slices.
// It normalizes RFC 3339 timestamps before comparison to handle format variations
// (e.g., "2026-01-15T10:00:00Z" vs "2026-01-15T10:00:00.000+00:00").
func ValuesEqual(a, b interface{}) bool {
	// Fast path: direct equality
	if reflect.DeepEqual(a, b) {
		return true
	}

	// Handle nil cases
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	// Handle string comparisons (timestamps)
	aStr, aIsStr := a.(string)
	bStr, bIsStr := b.(string)
	if aIsStr && bIsStr {
		// Try parsing as RFC 3339 timestamps
		aTime, aErr := time.Parse(time.RFC3339Nano, aStr)
		bTime, bErr := time.Parse(time.RFC3339Nano, bStr)
		if aErr == nil && bErr == nil {
			// Both are valid timestamps - compare as times
			return aTime.Equal(bTime)
		}
		// Not both timestamps - already compared as strings above
		return false
	}

	// Handle numeric type mismatches (JSON numbers can be float64 or int)
	// After JSON unmarshaling, numbers become float64
	aFloat, aIsFloat := ToFloat64(a)
	bFloat, bIsFloat := ToFloat64(b)
	if aIsFloat && bIsFloat {
		return aFloat == bFloat
	}

	// Handle map comparisons (nested objects)
	aMap, aIsMap := a.(map[string]interface{})
	bMap, bIsMap := b.(map[string]interface{})
	if aIsMap && bIsMap {
		return mapsEqual(aMap, bMap)
	}

	// Handle slice comparisons (arrays)
	aSlice, aIsSlice := toSlice(a)
	bSlice, bIsSlice := toSlice(b)
	if aIsSlice && bIsSlice {
		return slicesEqual(aSlice, bSlice)
	}

	return false
}

// mapsEqual compares two maps recursively for equality.
func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for key, aVal := range a {
		bVal, exists := b[key]
		if !exists {
			return false
		}
		if !ValuesEqual(aVal, bVal) {
			return false
		}
	}
	return true
}

// slicesEqual compares two slices recursively for equality.
func slicesEqual(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !ValuesEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

// toSlice attempts to convert a value to []interface{}.
// Returns the slice and true if conversion was successful.
func toSlice(v interface{}) ([]interface{}, bool) {
	if slice, ok := v.([]interface{}); ok {
		return slice, true
	}
	// Handle typed slices using reflection
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil, false
	}
	result := make([]interface{}, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		result[i] = rv.Index(i).Interface()
	}
	return result, true
}

// ToFloat64 attempts to convert a value to float64.
// Returns the float64 value and true if conversion was successful.
func ToFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// GetExternalIDIfPresent extracts ExternalID from a resource status if the field exists.
// Only CRUD resources have ExternalID; Query and Action CRDs do not.
// This uses reflection to safely access the field without compile-time type dependencies.
func GetExternalIDIfPresent(obj interface{}) string {
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}

	statusField := v.FieldByName("Status")
	if !statusField.IsValid() {
		return ""
	}
	if statusField.Kind() == reflect.Ptr {
		if statusField.IsNil() {
			return ""
		}
		statusField = statusField.Elem()
	}
	if statusField.Kind() != reflect.Struct {
		return ""
	}

	externalIDField := statusField.FieldByName("ExternalID")
	if !externalIDField.IsValid() {
		return ""
	}
	if externalIDField.Kind() == reflect.String {
		return externalIDField.String()
	}
	return ""
}
