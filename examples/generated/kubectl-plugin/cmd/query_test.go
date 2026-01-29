package cmd

import (
	"os"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/bluecontainer/petstore-operator/kubectl-plugin/pkg/client"
)

// =============================================================================
// resolveQueryKind tests
// =============================================================================

func TestResolveQueryKind(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"singular petfindbystatus", "petfindbystatusquery", "PetFindbystatusQuery"},
		{"plural petfindbystatus", "petfindbystatusqueries", "PetFindbystatusQuery"},
		{"singular petfindbytags", "petfindbytagsquery", "PetFindbytagsQuery"},
		{"plural petfindbytags", "petfindbytagsqueries", "PetFindbytagsQuery"},
		{"singular storeinventory", "storeinventoryquery", "StoreInventoryQuery"},
		{"plural storeinventory", "storeinventoryqueries", "StoreInventoryQuery"},
		{"singular userlogin", "userloginquery", "UserLoginQuery"},
		{"plural userlogin", "userloginqueries", "UserLoginQuery"},
		{"singular userlogout", "userlogoutquery", "UserLogoutQuery"},
		{"plural userlogout", "userlogoutqueries", "UserLogoutQuery"},
		{"unknown query", "unknownquery", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveQueryKind(tt.input)
			if got != tt.expected {
				t.Errorf("resolveQueryKind(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// coerceQueryParamValue tests
// =============================================================================

func TestCoerceQueryParamValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		// String values
		{"plain string", "available", "available"},
		{"empty string", "", ""},

		// Integer values
		{"positive integer", "10", int64(10)},
		{"zero", "0", int64(0)},
		{"negative integer", "-3", int64(-3)},

		// Float values
		{"float", "2.5", float64(2.5)},

		// Boolean values
		{"true", "true", true},
		{"false", "false", false},

		// Strings that remain strings (simpler coercion - no JSON/CSV)
		{"json-like", `{"key":"val"}`, `{"key":"val"}`},
		{"csv-like", "a,b,c", "a,b,c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coerceQueryParamValue(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("coerceQueryParamValue(%q) = %v (%T), want %v (%T)",
					tt.input, got, got, tt.expected, tt.expected)
			}
		})
	}
}

// =============================================================================
// isCommonFlag tests (query version)
// =============================================================================

func TestIsCommonFlag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Known flags
		{"interval", "interval", true},
		{"name", "name", true},
		{"wait", "wait", true},
		{"timeout", "timeout", true},
		{"output", "output", true},
		{"get", "get", true},
		{"quiet", "quiet", true},
		{"q", "q", true},
		{"dry-run", "dry-run", true},
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

		// Unknown flags (query params)
		{"status", "status", false},
		{"tags", "tags", false},
		{"petId", "petId", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCommonFlag(tt.input)
			if got != tt.expected {
				t.Errorf("isCommonFlag(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// formatValue tests (query version)
// =============================================================================

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"short string", "hello", "hello"},
		{"30 char string", "123456789012345678901234567890", "123456789012345678901234567890"},
		{"31 char string", "1234567890123456789012345678901", "123456789012345678901234567..."},
		{"long string", "this is a very long string that exceeds thirty characters", "this is a very long string ..."},
		{"map", map[string]interface{}{"key": "value"}, "{...}"},
		{"slice", []interface{}{1, 2, 3}, "[3 items]"},
		{"empty slice", []interface{}{}, "[0 items]"},
		{"integer", 42, "42"},
		{"bool", true, "true"},
		{"nil", nil, "<nil>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatValue(tt.input)
			if got != tt.expected {
				t.Errorf("formatValue(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// buildQueryCR tests
// =============================================================================

func TestBuildQueryCR_OneShot(t *testing.T) {
	origClient := k8sClient
	k8sClient = &client.Client{}
	k8sClient.SetNamespace("test-ns")
	defer func() { k8sClient = origClient }()

	resetTargetingFlags()
	defer resetTargetingFlags()

	params := map[string]interface{}{
		"status": "available",
	}

	cr := buildQueryCR("PetFindbystatusQuery", "test-query", params, false)

	// Verify apiVersion
	if cr.GetAPIVersion() != "petstore.example.com/v1alpha1" {
		t.Errorf("apiVersion = %q, want %q", cr.GetAPIVersion(), "petstore.example.com/v1alpha1")
	}

	// Verify kind
	if cr.GetKind() != "PetFindbystatusQuery" {
		t.Errorf("kind = %q, want %q", cr.GetKind(), "PetFindbystatusQuery")
	}

	// Verify name and namespace
	if cr.GetName() != "test-query" {
		t.Errorf("name = %q, want %q", cr.GetName(), "test-query")
	}
	if cr.GetNamespace() != "test-ns" {
		t.Errorf("namespace = %q, want %q", cr.GetNamespace(), "test-ns")
	}

	// Verify one-shot annotations
	annotations := cr.GetAnnotations()
	if annotations["petstore.example.com/one-shot"] != "true" {
		t.Errorf("one-shot annotation = %q, want %q", annotations["petstore.example.com/one-shot"], "true")
	}

	// Verify spec contains query params
	specVal, _, _ := unstructured.NestedMap(cr.Object, "spec")
	if specVal["status"] != "available" {
		t.Errorf("spec.status = %v, want %q", specVal["status"], "available")
	}

	// Verify no executionInterval for one-shot
	if _, hasInterval := specVal["executionInterval"]; hasInterval {
		t.Error("one-shot query should not have executionInterval")
	}
}

func TestBuildQueryCR_Periodic(t *testing.T) {
	origClient := k8sClient
	k8sClient = &client.Client{}
	k8sClient.SetNamespace("default")
	defer func() { k8sClient = origClient }()

	resetTargetingFlags()
	origInterval := queryInterval
	queryInterval = "5m"
	defer func() {
		resetTargetingFlags()
		queryInterval = origInterval
	}()

	params := map[string]interface{}{}
	cr := buildQueryCR("StoreInventoryQuery", "inventory-monitor", params, true)

	// Verify no one-shot annotation for periodic
	annotations := cr.GetAnnotations()
	if annotations["petstore.example.com/one-shot"] == "true" {
		t.Error("periodic query should not have one-shot annotation")
	}

	// Verify executionInterval is set
	specVal, _, _ := unstructured.NestedMap(cr.Object, "spec")
	if specVal["executionInterval"] != "5m" {
		t.Errorf("spec.executionInterval = %v, want %q", specVal["executionInterval"], "5m")
	}
}

func TestBuildQueryCR_WithPodOrdinal(t *testing.T) {
	origClient := k8sClient
	k8sClient = &client.Client{}
	k8sClient.SetNamespace("default")
	defer func() { k8sClient = origClient }()

	resetTargetingFlags()
	targetPodOrdinal = 1
	defer resetTargetingFlags()

	params := map[string]interface{}{}
	cr := buildQueryCR("StoreInventoryQuery", "test-query", params, false)

	// Access spec directly to avoid deep copy issues with non-JSON types (int vs int64)
	spec, ok := cr.Object["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("spec should be a map")
	}
	target, ok := spec["target"].(map[string]interface{})
	if !ok {
		t.Fatal("spec.target should be a map")
	}
	if target["podOrdinal"] != 1 {
		t.Errorf("spec.target.podOrdinal = %v, want 1", target["podOrdinal"])
	}
}

// =============================================================================
// parseQueryParams tests
// =============================================================================

func TestParseQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]interface{}
	}{
		{
			"key=value syntax",
			[]string{"cmd", "query", "petfindbystatusquery", "--status=available"},
			map[string]interface{}{
				"status": "available",
			},
		},
		{
			"skips common flags",
			[]string{"cmd", "query", "test", "--status=available", "--dry-run", "--wait", "--timeout=30s"},
			map[string]interface{}{
				"status": "available",
			},
		},
		{
			"multiple params",
			[]string{"cmd", "query", "test", "--status=available", "--limit=10"},
			map[string]interface{}{
				"status": "available",
				"limit":  int64(10),
			},
		},
		{
			"no params",
			[]string{"cmd", "query", "test"},
			map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origArgs := os.Args
			os.Args = tt.args
			defer func() { os.Args = origArgs }()

			got := parseQueryParams()
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseQueryParams() = %v, want %v", got, tt.expected)
			}
		})
	}
}
