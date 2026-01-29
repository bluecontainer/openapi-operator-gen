package cmd

import (
	"os"
	"reflect"
	"testing"
)

// =============================================================================
// coercePatchParamValue tests
// =============================================================================

func TestCoercePatchParamValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		// String values
		{"plain string", "pending", "pending"},
		{"empty string", "", ""},

		// Integer values
		{"positive integer", "42", int64(42)},
		{"zero", "0", int64(0)},
		{"negative integer", "-5", int64(-5)},

		// Float values
		{"float", "3.14", float64(3.14)},

		// Boolean values
		{"true", "true", true},
		{"false", "false", false},

		// JSON object
		{"json object", `{"id":1,"name":"Dogs"}`, map[string]interface{}{"id": float64(1), "name": "Dogs"}},
		{"nested json", `{"a":{"b":"c"}}`, map[string]interface{}{"a": map[string]interface{}{"b": "c"}}},

		// JSON array
		{"json array", `[1,2,3]`, []interface{}{float64(1), float64(2), float64(3)}},
		{"json array of objects", `[{"id":10,"name":"fluffy"}]`, []interface{}{map[string]interface{}{"id": float64(10), "name": "fluffy"}}},

		// Invalid JSON (falls back)
		{"invalid json object", `{not json}`, "{not json}"},

		// Comma-separated array
		{"comma-separated", "cute,fluffy", []interface{}{"cute", "fluffy"}},
		{"comma-separated with spaces", "a, b, c", []interface{}{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coercePatchParamValue(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("coercePatchParamValue(%q) = %v (%T), want %v (%T)",
					tt.input, got, got, tt.expected, tt.expected)
			}
		})
	}
}

// =============================================================================
// isPatchCommonFlag tests
// =============================================================================

func TestIsPatchCommonFlag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Known flags
		{"spec", "spec", true},
		{"ttl", "ttl", true},
		{"restore", "restore", true},
		{"dry-run", "dry-run", true},
		{"output", "output", true},
		{"namespace", "namespace", true},
		{"context", "context", true},
		{"kubeconfig", "kubeconfig", true},

		// Targeting flags (delegated to isTargetingFlag)
		{"target-pod-ordinal", "target-pod-ordinal", true},
		{"target-base-url", "target-base-url", true},
		{"target-base-urls", "target-base-urls", true},
		{"target-statefulset", "target-statefulset", true},
		{"target-deployment", "target-deployment", true},
		{"target-pod", "target-pod", true},
		{"target-helm-release", "target-helm-release", true},
		{"target-namespace", "target-namespace", true},
		{"target-labels", "target-labels", true},

		// Unknown flags (spec fields)
		{"name", "name", false},
		{"status", "status", false},
		{"tags", "tags", false},
		{"category", "category", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPatchCommonFlag(tt.input)
			if got != tt.expected {
				t.Errorf("isPatchCommonFlag(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// formatPatchValue tests
// =============================================================================

func TestFormatPatchValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"nil", nil, "<nil>"},
		{"short string", "hello", `"hello"`},
		{"30 char string", "123456789012345678901234567890", `"123456789012345678901234567890"`},
		{"31 char string", "1234567890123456789012345678901", `"123456789012345678901234567..."`},
		{"long string", "this is a very long string that is definitely longer than thirty characters", `"this is a very long string ..."`},
		{"map", map[string]interface{}{"key": "value"}, "{...}"},
		{"empty map", map[string]interface{}{}, "{...}"},
		{"slice", []interface{}{"a", "b", "c"}, "[3 items]"},
		{"empty slice", []interface{}{}, "[0 items]"},
		{"integer", 42, "42"},
		{"float", 3.14, "3.14"},
		{"bool", true, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPatchValue(tt.input)
			if got != tt.expected {
				t.Errorf("formatPatchValue(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// buildPatchSpec tests
// =============================================================================

func TestBuildPatchSpec_FromJSON(t *testing.T) {
	origSpec := patchSpec
	patchSpec = `{"status":"pending","name":"updated"}`
	origArgs := os.Args
	os.Args = []string{"cmd", "patch", "pet", "fluffy"}
	defer func() {
		patchSpec = origSpec
		os.Args = origArgs
	}()

	result, err := buildPatchSpec()
	if err != nil {
		t.Fatalf("buildPatchSpec() error = %v", err)
	}

	if result["status"] != "pending" {
		t.Errorf("result[status] = %v, want %q", result["status"], "pending")
	}
	if result["name"] != "updated" {
		t.Errorf("result[name] = %v, want %q", result["name"], "updated")
	}
}

func TestBuildPatchSpec_FromFlags(t *testing.T) {
	origSpec := patchSpec
	patchSpec = ""
	origArgs := os.Args
	os.Args = []string{"cmd", "patch", "pet", "fluffy", "--status=pending", "--name=updated"}
	defer func() {
		patchSpec = origSpec
		os.Args = origArgs
	}()

	result, err := buildPatchSpec()
	if err != nil {
		t.Fatalf("buildPatchSpec() error = %v", err)
	}

	if result["status"] != "pending" {
		t.Errorf("result[status] = %v, want %q", result["status"], "pending")
	}
	if result["name"] != "updated" {
		t.Errorf("result[name] = %v, want %q", result["name"], "updated")
	}
}

func TestBuildPatchSpec_FlagsOverrideSpec(t *testing.T) {
	origSpec := patchSpec
	patchSpec = `{"status":"available","name":"original"}`
	origArgs := os.Args
	os.Args = []string{"cmd", "patch", "pet", "fluffy", "--status=pending"}
	defer func() {
		patchSpec = origSpec
		os.Args = origArgs
	}()

	result, err := buildPatchSpec()
	if err != nil {
		t.Fatalf("buildPatchSpec() error = %v", err)
	}

	// Flag should override --spec JSON
	if result["status"] != "pending" {
		t.Errorf("result[status] = %v, want %q (flag should override spec)", result["status"], "pending")
	}
	// Non-overridden value from --spec should remain
	if result["name"] != "original" {
		t.Errorf("result[name] = %v, want %q", result["name"], "original")
	}
}

func TestBuildPatchSpec_InvalidJSON(t *testing.T) {
	origSpec := patchSpec
	patchSpec = `{invalid json}`
	origArgs := os.Args
	os.Args = []string{"cmd", "patch", "pet", "fluffy"}
	defer func() {
		patchSpec = origSpec
		os.Args = origArgs
	}()

	_, err := buildPatchSpec()
	if err == nil {
		t.Error("buildPatchSpec() expected error for invalid JSON")
	}
}

func TestBuildPatchSpec_Empty(t *testing.T) {
	origSpec := patchSpec
	patchSpec = ""
	origArgs := os.Args
	os.Args = []string{"cmd", "patch", "pet", "fluffy"}
	defer func() {
		patchSpec = origSpec
		os.Args = origArgs
	}()

	result, err := buildPatchSpec()
	if err != nil {
		t.Fatalf("buildPatchSpec() error = %v", err)
	}

	if len(result) != 0 {
		t.Errorf("buildPatchSpec() = %v, want empty map", result)
	}
}

// =============================================================================
// parsePatchParams tests
// =============================================================================

func TestParsePatchParams(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]interface{}
	}{
		{
			"key=value syntax",
			[]string{"cmd", "patch", "pet", "fluffy", "--status=pending"},
			map[string]interface{}{
				"status": "pending",
			},
		},
		{
			"skips common flags",
			[]string{"cmd", "patch", "pet", "fluffy", "--status=pending", "--dry-run", "--ttl=1h", "--spec={}", "--output=json"},
			map[string]interface{}{
				"status": "pending",
			},
		},
		{
			"json value",
			[]string{"cmd", "patch", "pet", "fluffy", `--tags=[{"id":1,"name":"cute"}]`},
			map[string]interface{}{
				"tags": []interface{}{map[string]interface{}{"id": float64(1), "name": "cute"}},
			},
		},
		{
			"csv value",
			[]string{"cmd", "patch", "pet", "fluffy", "--tags=cute,fluffy"},
			map[string]interface{}{
				"tags": []interface{}{"cute", "fluffy"},
			},
		},
		{
			"multiple params",
			[]string{"cmd", "patch", "pet", "fluffy", "--status=pending", "--name=updated"},
			map[string]interface{}{
				"status": "pending",
				"name":   "updated",
			},
		},
		{
			"no params",
			[]string{"cmd", "patch", "pet", "fluffy"},
			map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origArgs := os.Args
			os.Args = tt.args
			defer func() { os.Args = origArgs }()

			got := parsePatchParams()
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parsePatchParams() = %v, want %v", got, tt.expected)
			}
		})
	}
}
