package cmd

import (
	"os"
	"reflect"
	"testing"

	"github.com/bluecontainer/petstore-operator/kubectl-plugin/pkg/client"
)

// =============================================================================
// resolveActionKind tests
// =============================================================================

func TestResolveActionKind(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"singular petuploadimage", "petuploadimageaction", "PetUploadimageAction"},
		{"plural petuploadimage", "petuploadimageactions", "PetUploadimageAction"},
		{"singular usercreatewithlist", "usercreatewithlistaction", "UserCreatewithlistAction"},
		{"plural usercreatewithlist", "usercreatewithlistactions", "UserCreatewithlistAction"},
		{"mixed case", "PetUploadimageAction", "PetUploadimageAction"},
		{"unknown action", "unknownaction", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveActionKind(tt.input)
			if got != tt.expected {
				t.Errorf("resolveActionKind(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// resolveActionPlural tests
// =============================================================================

func TestResolveActionPlural(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"singular petuploadimage", "petuploadimageaction", "petuploadimageactions"},
		{"plural petuploadimage", "petuploadimageactions", "petuploadimageactions"},
		{"singular usercreatewithlist", "usercreatewithlistaction", "usercreatewithlistactions"},
		{"plural usercreatewithlist", "usercreatewithlistactions", "usercreatewithlistactions"},
		{"unknown action", "unknownaction", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveActionPlural(tt.input)
			if got != tt.expected {
				t.Errorf("resolveActionPlural(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// coerceParamValue tests (action version - simpler, no JSON/CSV)
// =============================================================================

func TestCoerceParamValue(t *testing.T) {
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

		// Float values
		{"float", "3.14", float64(3.14)},
		{"negative float", "-1.5", float64(-1.5)},

		// Boolean values
		{"true", "true", true},
		{"false", "false", false},

		// Strings that look like JSON but action coercion doesn't try JSON
		{"json-like string", `{"id":1}`, `{"id":1}`},
		{"csv-like string", "a,b,c", "a,b,c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coerceParamValue(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("coerceParamValue(%q) = %v (%T), want %v (%T)",
					tt.input, got, got, tt.expected, tt.expected)
			}
		})
	}
}

// =============================================================================
// isActionCommonFlag tests
// =============================================================================

func TestIsActionCommonFlag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Known flags
		{"pod", "pod", true},
		{"name", "name", true},
		{"wait", "wait", true},
		{"timeout", "timeout", true},
		{"output", "output", true},
		{"file", "file", true},
		{"dry-run", "dry-run", true},
		{"namespace", "namespace", true},
		{"context", "context", true},
		{"kubeconfig", "kubeconfig", true},

		// Unknown flags (action params)
		{"petId", "petId", false},
		{"data", "data", false},
		{"status", "status", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isActionCommonFlag(tt.input)
			if got != tt.expected {
				t.Errorf("isActionCommonFlag(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// formatActionValue tests
// =============================================================================

func TestFormatActionValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"short string", "hello", "hello"},
		{"long string", "this is a very long string that exceeds fifty characters in total length", "this is a very long string that exceeds fifty c..."},
		{"map", map[string]interface{}{"key": "value"}, "{...}"},
		{"slice", []interface{}{"a", "b"}, "[2 items]"},
		{"empty slice", []interface{}{}, "[0 items]"},
		{"integer", 42, "42"},
		{"bool", false, "false"},
		{"nil", nil, "<nil>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatActionValue(tt.input)
			if got != tt.expected {
				t.Errorf("formatActionValue(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// buildActionCR tests
// =============================================================================

func TestBuildActionCR(t *testing.T) {
	origClient := k8sClient
	k8sClient = &client.Client{}
	k8sClient.SetNamespace("test-ns")
	defer func() { k8sClient = origClient }()

	// Reset pod ordinal
	origOrdinal := actionPodOrdinal
	actionPodOrdinal = -1
	defer func() { actionPodOrdinal = origOrdinal }()

	params := map[string]interface{}{
		"petId": int64(123),
	}

	cr := buildActionCR("PetUploadimageAction", "test-action", params)

	// Verify apiVersion
	if cr.GetAPIVersion() != "petstore.example.com/v1alpha1" {
		t.Errorf("apiVersion = %q, want %q", cr.GetAPIVersion(), "petstore.example.com/v1alpha1")
	}

	// Verify kind
	if cr.GetKind() != "PetUploadimageAction" {
		t.Errorf("kind = %q, want %q", cr.GetKind(), "PetUploadimageAction")
	}

	// Verify name
	if cr.GetName() != "test-action" {
		t.Errorf("name = %q, want %q", cr.GetName(), "test-action")
	}

	// Verify namespace
	if cr.GetNamespace() != "test-ns" {
		t.Errorf("namespace = %q, want %q", cr.GetNamespace(), "test-ns")
	}

	// Verify annotations
	annotations := cr.GetAnnotations()
	if annotations["petstore.example.com/one-shot"] != "true" {
		t.Errorf("one-shot annotation = %q, want %q", annotations["petstore.example.com/one-shot"], "true")
	}
	if annotations["petstore.example.com/created-by"] != "kubectl-plugin" {
		t.Errorf("created-by annotation = %q, want %q", annotations["petstore.example.com/created-by"], "kubectl-plugin")
	}

	// Verify spec contains params (access directly to avoid deep copy issues)
	specVal, ok := cr.Object["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("spec should be a map")
	}
	if specVal["petId"] != int64(123) {
		t.Errorf("spec.petId = %v, want 123", specVal["petId"])
	}

	// Verify no target when pod ordinal is -1
	if _, hasTarget := specVal["target"]; hasTarget {
		t.Error("spec should not have target when podOrdinal is -1")
	}
}

func TestBuildActionCR_WithPodOrdinal(t *testing.T) {
	origClient := k8sClient
	k8sClient = &client.Client{}
	k8sClient.SetNamespace("default")
	defer func() { k8sClient = origClient }()

	origOrdinal := actionPodOrdinal
	actionPodOrdinal = 2
	defer func() { actionPodOrdinal = origOrdinal }()

	params := map[string]interface{}{}
	cr := buildActionCR("PetUploadimageAction", "test-action", params)

	// Access spec directly to avoid deep copy issues with non-JSON types (int vs int64)
	spec, ok := cr.Object["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("spec should be a map")
	}
	target, ok := spec["target"].(map[string]interface{})
	if !ok {
		t.Fatal("spec.target should be a map")
	}
	if target["podOrdinal"] != 2 {
		t.Errorf("spec.target.podOrdinal = %v, want 2", target["podOrdinal"])
	}
}

// =============================================================================
// parseActionParams tests
// =============================================================================

func TestParseActionParams(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]interface{}
	}{
		{
			"key=value syntax",
			[]string{"cmd", "action", "petuploadimageaction", "--petId=123"},
			map[string]interface{}{
				"petId": int64(123),
			},
		},
		{
			"skips common flags",
			[]string{"cmd", "action", "test", "--petId=123", "--dry-run", "--timeout=30s", "--pod=0"},
			map[string]interface{}{
				"petId": int64(123),
			},
		},
		{
			"space-separated key value",
			[]string{"cmd", "action", "test", "--petId", "123"},
			map[string]interface{}{
				"petId": int64(123),
			},
		},
		{
			"no params",
			[]string{"cmd", "action", "test"},
			map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origArgs := os.Args
			os.Args = tt.args
			defer func() { os.Args = origArgs }()

			got := parseActionParams()
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseActionParams() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// =============================================================================
// readFileForUpload tests
// =============================================================================

func TestReadFileForUpload(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.txt"
	os.WriteFile(path, []byte("hello world"), 0644)

	encoded, err := readFileForUpload(path)
	if err != nil {
		t.Fatalf("readFileForUpload() error = %v", err)
	}

	// "hello world" base64 encoded
	expected := "aGVsbG8gd29ybGQ="
	if encoded != expected {
		t.Errorf("readFileForUpload() = %q, want %q", encoded, expected)
	}
}

func TestReadFileForUpload_NotFound(t *testing.T) {
	_, err := readFileForUpload("/nonexistent/file.txt")
	if err == nil {
		t.Error("readFileForUpload() expected error for non-existent file")
	}
}

func TestReadFileForUpload_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/empty.txt"
	os.WriteFile(path, []byte{}, 0644)

	encoded, err := readFileForUpload(path)
	if err != nil {
		t.Fatalf("readFileForUpload() error = %v", err)
	}

	if encoded != "" {
		t.Errorf("readFileForUpload() = %q, want empty string", encoded)
	}
}
