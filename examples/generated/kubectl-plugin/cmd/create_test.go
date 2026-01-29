package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/bluecontainer/petstore-operator/kubectl-plugin/pkg/client"
)

// =============================================================================
// resolveResourceKind tests
// =============================================================================

func TestResolveResourceKind(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"singular lowercase", "pet", "Pet"},
		{"plural lowercase", "pets", "Pet"},
		{"singular order", "order", "Order"},
		{"plural order", "orders", "Order"},
		{"singular user", "user", "User"},
		{"plural user", "users", "User"},
		{"mixed case", "Pet", "Pet"},
		{"uppercase", "PET", "Pet"},
		{"unknown kind", "widget", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveResourceKind(tt.input)
			if got != tt.expected {
				t.Errorf("resolveResourceKind(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// resolveResourcePlural tests
// =============================================================================

func TestResolveResourcePlural(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"singular pet", "pet", "pets"},
		{"plural pet", "pets", "pets"},
		{"singular order", "order", "orders"},
		{"plural order", "orders", "orders"},
		{"singular user", "user", "users"},
		{"plural user", "users", "users"},
		{"unknown kind", "widget", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveResourcePlural(tt.input)
			if got != tt.expected {
				t.Errorf("resolveResourcePlural(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// coerceCreateParamValue tests
// =============================================================================

func TestCoerceCreateParamValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		// String values
		{"plain string", "hello", "hello"},
		{"string with spaces", "hello world", "hello world"},
		{"empty string", "", ""},

		// Integer values
		{"positive integer", "42", int64(42)},
		{"zero", "0", int64(0)},
		{"negative integer", "-5", int64(-5)},
		{"large integer", "9999999", int64(9999999)},

		// Float values
		{"float", "3.14", float64(3.14)},
		{"negative float", "-1.5", float64(-1.5)},

		// Boolean values
		{"true", "true", true},
		{"false", "false", false},
		{"True", "True", true},
		{"FALSE", "FALSE", false},

		// JSON object
		{"json object", `{"id":1,"name":"Dogs"}`, map[string]interface{}{"id": float64(1), "name": "Dogs"}},
		{"nested json", `{"a":{"b":"c"}}`, map[string]interface{}{"a": map[string]interface{}{"b": "c"}}},

		// JSON array
		{"json array", `[1,2,3]`, []interface{}{float64(1), float64(2), float64(3)}},
		{"json array of objects", `[{"id":1}]`, []interface{}{map[string]interface{}{"id": float64(1)}}},

		// Invalid JSON (falls back)
		{"invalid json object", `{not json}`, "{not json}"},
		{"invalid json array", `[not json`, "[not json"},

		// Comma-separated array
		{"comma-separated", "a,b,c", []interface{}{"a", "b", "c"}},
		{"comma-separated with spaces", "a, b, c", []interface{}{"a", "b", "c"}},
		{"two items", "foo,bar", []interface{}{"foo", "bar"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coerceCreateParamValue(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("coerceCreateParamValue(%q) = %v (%T), want %v (%T)",
					tt.input, got, got, tt.expected, tt.expected)
			}
		})
	}
}

// =============================================================================
// isCreateCommonFlag tests
// =============================================================================

func TestIsCreateCommonFlag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Known flags
		{"cr-name", "cr-name", true},
		{"no-wait", "no-wait", true},
		{"wait", "wait", true},
		{"timeout", "timeout", true},
		{"output", "output", true},
		{"from-file", "from-file", true},
		{"dry-run", "dry-run", true},
		{"namespace", "namespace", true},
		{"context", "context", true},
		{"kubeconfig", "kubeconfig", true},

		// Unknown flags (spec fields)
		{"name", "name", false},
		{"status", "status", false},
		{"tags", "tags", false},
		{"category", "category", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCreateCommonFlag(tt.input)
			if got != tt.expected {
				t.Errorf("isCreateCommonFlag(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// formatCreateValue tests
// =============================================================================

func TestFormatCreateValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"short string", "hello", "hello"},
		{"long string", "this is a very long string that exceeds fifty characters in total length", "this is a very long string that exceeds fifty c..."},
		{"exactly 50 chars", "12345678901234567890123456789012345678901234567890", "12345678901234567890123456789012345678901234567890"},
		{"51 chars", "123456789012345678901234567890123456789012345678901", "12345678901234567890123456789012345678901234567..."},
		{"map", map[string]interface{}{"key": "value"}, "{...}"},
		{"empty map", map[string]interface{}{}, "{...}"},
		{"slice", []interface{}{"a", "b", "c"}, "[3 items]"},
		{"empty slice", []interface{}{}, "[0 items]"},
		{"integer", 42, "42"},
		{"float", 3.14, "3.14"},
		{"bool", true, "true"},
		{"nil", nil, "<nil>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCreateValue(tt.input)
			if got != tt.expected {
				t.Errorf("formatCreateValue(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// buildResourceCR tests
// =============================================================================

func TestBuildResourceCR(t *testing.T) {
	// Set up minimal k8sClient for namespace resolution
	origClient := k8sClient
	k8sClient = &client.Client{}
	k8sClient.SetNamespace("test-ns")
	defer func() { k8sClient = origClient }()

	spec := map[string]interface{}{
		"name":   "fluffy",
		"status": "available",
	}

	cr := buildResourceCR("Pet", "my-pet", spec)

	// Verify apiVersion
	if cr.GetAPIVersion() != "petstore.example.com/v1alpha1" {
		t.Errorf("apiVersion = %q, want %q", cr.GetAPIVersion(), "petstore.example.com/v1alpha1")
	}

	// Verify kind
	if cr.GetKind() != "Pet" {
		t.Errorf("kind = %q, want %q", cr.GetKind(), "Pet")
	}

	// Verify name
	if cr.GetName() != "my-pet" {
		t.Errorf("name = %q, want %q", cr.GetName(), "my-pet")
	}

	// Verify namespace
	if cr.GetNamespace() != "test-ns" {
		t.Errorf("namespace = %q, want %q", cr.GetNamespace(), "test-ns")
	}

	// Verify annotations
	annotations := cr.GetAnnotations()
	if annotations["petstore.example.com/created-by"] != "kubectl-plugin" {
		t.Errorf("created-by annotation = %q, want %q", annotations["petstore.example.com/created-by"], "kubectl-plugin")
	}

	// Verify spec
	specVal, _, _ := unstructured.NestedMap(cr.Object, "spec")
	if specVal["name"] != "fluffy" {
		t.Errorf("spec.name = %v, want %q", specVal["name"], "fluffy")
	}
	if specVal["status"] != "available" {
		t.Errorf("spec.status = %v, want %q", specVal["status"], "available")
	}
}

func TestBuildResourceCR_EmptySpec(t *testing.T) {
	origClient := k8sClient
	k8sClient = &client.Client{}
	k8sClient.SetNamespace("default")
	defer func() { k8sClient = origClient }()

	spec := map[string]interface{}{}
	cr := buildResourceCR("Order", "test-order", spec)

	if cr.GetKind() != "Order" {
		t.Errorf("kind = %q, want %q", cr.GetKind(), "Order")
	}

	specVal, _, _ := unstructured.NestedMap(cr.Object, "spec")
	if len(specVal) != 0 {
		t.Errorf("spec should be empty, got %v", specVal)
	}
}

// =============================================================================
// loadSpecFromFile tests
// =============================================================================

func TestLoadSpecFromFile_YAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")
	content := `name: fluffy
status: available
tags:
  - cute
  - fluffy
`
	os.WriteFile(path, []byte(content), 0644)

	spec, err := loadSpecFromFile(path)
	if err != nil {
		t.Fatalf("loadSpecFromFile() error = %v", err)
	}

	if spec["name"] != "fluffy" {
		t.Errorf("spec[name] = %v, want %q", spec["name"], "fluffy")
	}
	if spec["status"] != "available" {
		t.Errorf("spec[status] = %v, want %q", spec["status"], "available")
	}

	tags, ok := spec["tags"].([]interface{})
	if !ok || len(tags) != 2 {
		t.Errorf("spec[tags] = %v, want [cute fluffy]", spec["tags"])
	}
}

func TestLoadSpecFromFile_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.json")
	content := `{"name": "fluffy", "id": 123}`
	os.WriteFile(path, []byte(content), 0644)

	spec, err := loadSpecFromFile(path)
	if err != nil {
		t.Fatalf("loadSpecFromFile() error = %v", err)
	}

	if spec["name"] != "fluffy" {
		t.Errorf("spec[name] = %v, want %q", spec["name"], "fluffy")
	}
	// JSON numbers unmarshal as float64 via YAML decoder
	if id, ok := spec["id"].(int64); ok {
		if id != 123 {
			t.Errorf("spec[id] = %v, want 123", id)
		}
	} else if id, ok := spec["id"].(float64); ok {
		if id != 123 {
			t.Errorf("spec[id] = %v, want 123", id)
		}
	}
}

func TestLoadSpecFromFile_NotFound(t *testing.T) {
	_, err := loadSpecFromFile("/nonexistent/path/file.yaml")
	if err == nil {
		t.Error("loadSpecFromFile() expected error for non-existent file")
	}
}

func TestLoadSpecFromFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	// Write something that won't unmarshal as a map
	os.WriteFile(path, []byte("- just\n- a\n- list\n"), 0644)

	_, err := loadSpecFromFile(path)
	if err == nil {
		t.Error("loadSpecFromFile() expected error for non-map YAML")
	}
}

// =============================================================================
// parseCreateParams tests (requires os.Args manipulation)
// =============================================================================

func TestParseCreateParams(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]interface{}
	}{
		{
			"key=value syntax",
			[]string{"cmd", "create", "pet", "--name=fluffy", "--status=available"},
			map[string]interface{}{
				"name":   "fluffy",
				"status": "available",
			},
		},
		{
			"skips common flags",
			[]string{"cmd", "create", "pet", "--name=fluffy", "--dry-run", "--timeout=30s", "--output=json"},
			map[string]interface{}{
				"name": "fluffy",
			},
		},
		{
			"integer coercion",
			[]string{"cmd", "create", "pet", "--id=42"},
			map[string]interface{}{
				"id": int64(42),
			},
		},
		{
			"json object coercion",
			[]string{"cmd", "create", "pet", `--category={"id":1,"name":"Dogs"}`},
			map[string]interface{}{
				"category": map[string]interface{}{"id": float64(1), "name": "Dogs"},
			},
		},
		{
			"no params",
			[]string{"cmd", "create", "pet"},
			map[string]interface{}{},
		},
		{
			"key value space syntax",
			[]string{"cmd", "create", "pet", "--name", "fluffy"},
			map[string]interface{}{
				"name": "fluffy",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origArgs := os.Args
			os.Args = tt.args
			defer func() { os.Args = origArgs }()

			got := parseCreateParams()
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseCreateParams() = %v, want %v", got, tt.expected)
			}
		})
	}
}
