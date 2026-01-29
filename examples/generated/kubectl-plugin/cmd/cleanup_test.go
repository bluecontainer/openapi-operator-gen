package cmd

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// =============================================================================
// matchesLabelSelector tests
// =============================================================================

func TestMatchesLabelSelector(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		selector string
		expected bool
	}{
		{
			"matching label",
			map[string]string{"app": "petstore", "env": "dev"},
			"app=petstore",
			true,
		},
		{
			"non-matching value",
			map[string]string{"app": "petstore"},
			"app=other",
			false,
		},
		{
			"missing key",
			map[string]string{"app": "petstore"},
			"env=dev",
			false,
		},
		{
			"empty labels",
			map[string]string{},
			"app=petstore",
			false,
		},
		{
			"nil labels",
			nil,
			"app=petstore",
			false,
		},
		{
			"invalid selector - no equals",
			map[string]string{"app": "petstore"},
			"app",
			false,
		},
		{
			"empty selector",
			map[string]string{"app": "petstore"},
			"",
			false,
		},
		{
			"selector with value containing equals",
			map[string]string{"config": "a=b"},
			"config=a=b",
			true,
		},
		{
			"exact match session label",
			map[string]string{"diagnostic-session": "session-123"},
			"diagnostic-session=session-123",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesLabelSelector(tt.labels, tt.selector)
			if got != tt.expected {
				t.Errorf("matchesLabelSelector(%v, %q) = %v, want %v",
					tt.labels, tt.selector, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// checkResourceForCleanup tests
// =============================================================================

func TestCheckResourceForCleanup_OneShot(t *testing.T) {
	origOneShot := cleanupOneShot
	origExpired := cleanupExpired
	origDiagnostic := cleanupDiagnostic
	cleanupOneShot = true
	cleanupExpired = false
	cleanupDiagnostic = false
	defer func() {
		cleanupOneShot = origOneShot
		cleanupExpired = origExpired
		cleanupDiagnostic = origDiagnostic
	}()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "petstore.example.com/v1alpha1",
			"kind":       "PetFindbystatusQuery",
			"metadata": map[string]interface{}{
				"name":      "test-query",
				"namespace": "default",
				"annotations": map[string]interface{}{
					"petstore.example.com/one-shot": "true",
				},
			},
		},
	}

	target := checkResourceForCleanup(obj, "PetFindbystatusQuery")
	if target == nil {
		t.Fatal("checkResourceForCleanup() returned nil, expected cleanup target")
	}

	if target.Kind != "PetFindbystatusQuery" {
		t.Errorf("target.Kind = %q, want %q", target.Kind, "PetFindbystatusQuery")
	}
	if target.Name != "test-query" {
		t.Errorf("target.Name = %q, want %q", target.Name, "test-query")
	}
	if target.Namespace != "default" {
		t.Errorf("target.Namespace = %q, want %q", target.Namespace, "default")
	}
	if target.Reason != "one-shot" {
		t.Errorf("target.Reason = %q, want %q", target.Reason, "one-shot")
	}
	if target.Action != CleanupActionDelete {
		t.Errorf("target.Action = %q, want %q", target.Action, CleanupActionDelete)
	}
}

func TestCheckResourceForCleanup_ExpiredTTL(t *testing.T) {
	origOneShot := cleanupOneShot
	origExpired := cleanupExpired
	origDiagnostic := cleanupDiagnostic
	cleanupOneShot = false
	cleanupExpired = true
	cleanupDiagnostic = false
	defer func() {
		cleanupOneShot = origOneShot
		cleanupExpired = origExpired
		cleanupDiagnostic = origDiagnostic
	}()

	// Use a time in the past for the expiry
	pastTime := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	originalState := `{"status":"available"}`

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "petstore.example.com/v1alpha1",
			"kind":       "Pet",
			"metadata": map[string]interface{}{
				"name":      "fluffy",
				"namespace": "default",
				"annotations": map[string]interface{}{
					"petstore.example.com/patch-expires":       pastTime,
					"petstore.example.com/patch-original-state": originalState,
				},
			},
		},
	}

	target := checkResourceForCleanup(obj, "Pet")
	if target == nil {
		t.Fatal("checkResourceForCleanup() returned nil, expected cleanup target for expired TTL")
	}

	if target.Action != CleanupActionRestore {
		t.Errorf("target.Action = %q, want %q", target.Action, CleanupActionRestore)
	}
	if target.Reason != "expired TTL" {
		t.Errorf("target.Reason = %q, want %q", target.Reason, "expired TTL")
	}
	if target.OriginalState != originalState {
		t.Errorf("target.OriginalState = %q, want %q", target.OriginalState, originalState)
	}
}

func TestCheckResourceForCleanup_NotExpiredTTL(t *testing.T) {
	origOneShot := cleanupOneShot
	origExpired := cleanupExpired
	origDiagnostic := cleanupDiagnostic
	cleanupOneShot = false
	cleanupExpired = true
	cleanupDiagnostic = false
	defer func() {
		cleanupOneShot = origOneShot
		cleanupExpired = origExpired
		cleanupDiagnostic = origDiagnostic
	}()

	// Use a time in the future for the expiry
	futureTime := time.Now().Add(1 * time.Hour).Format(time.RFC3339)

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "petstore.example.com/v1alpha1",
			"kind":       "Pet",
			"metadata": map[string]interface{}{
				"name":      "fluffy",
				"namespace": "default",
				"annotations": map[string]interface{}{
					"petstore.example.com/patch-expires": futureTime,
				},
			},
		},
	}

	target := checkResourceForCleanup(obj, "Pet")
	if target != nil {
		t.Error("checkResourceForCleanup() should return nil for non-expired TTL")
	}
}

func TestCheckResourceForCleanup_Diagnostic(t *testing.T) {
	origOneShot := cleanupOneShot
	origExpired := cleanupExpired
	origDiagnostic := cleanupDiagnostic
	cleanupOneShot = false
	cleanupExpired = false
	cleanupDiagnostic = true
	defer func() {
		cleanupOneShot = origOneShot
		cleanupExpired = origExpired
		cleanupDiagnostic = origDiagnostic
	}()

	// Test with purpose=diagnostic
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "petstore.example.com/v1alpha1",
			"kind":       "Pet",
			"metadata": map[string]interface{}{
				"name":      "diag-pet",
				"namespace": "default",
				"annotations": map[string]interface{}{
					"petstore.example.com/purpose": "diagnostic",
				},
			},
		},
	}

	target := checkResourceForCleanup(obj, "Pet")
	if target == nil {
		t.Fatal("checkResourceForCleanup() returned nil, expected cleanup target for diagnostic")
	}

	if target.Reason != "diagnostic" {
		t.Errorf("target.Reason = %q, want %q", target.Reason, "diagnostic")
	}
	if target.Action != CleanupActionDelete {
		t.Errorf("target.Action = %q, want %q", target.Action, CleanupActionDelete)
	}
}

func TestCheckResourceForCleanup_DiagnosticCreatedBy(t *testing.T) {
	origOneShot := cleanupOneShot
	origExpired := cleanupExpired
	origDiagnostic := cleanupDiagnostic
	cleanupOneShot = false
	cleanupExpired = false
	cleanupDiagnostic = true
	defer func() {
		cleanupOneShot = origOneShot
		cleanupExpired = origExpired
		cleanupDiagnostic = origDiagnostic
	}()

	// Test with created-by=kubectl-plugin
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "petstore.example.com/v1alpha1",
			"kind":       "Pet",
			"metadata": map[string]interface{}{
				"name":      "plugin-pet",
				"namespace": "default",
				"annotations": map[string]interface{}{
					"petstore.example.com/created-by": "kubectl-plugin",
				},
			},
		},
	}

	target := checkResourceForCleanup(obj, "Pet")
	if target == nil {
		t.Fatal("checkResourceForCleanup() returned nil, expected cleanup target for created-by")
	}

	if target.Reason != "diagnostic" {
		t.Errorf("target.Reason = %q, want %q", target.Reason, "diagnostic")
	}
}

func TestCheckResourceForCleanup_NoAnnotations(t *testing.T) {
	origOneShot := cleanupOneShot
	origExpired := cleanupExpired
	origDiagnostic := cleanupDiagnostic
	cleanupOneShot = true
	cleanupExpired = true
	cleanupDiagnostic = true
	defer func() {
		cleanupOneShot = origOneShot
		cleanupExpired = origExpired
		cleanupDiagnostic = origDiagnostic
	}()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "petstore.example.com/v1alpha1",
			"kind":       "Pet",
			"metadata": map[string]interface{}{
				"name":      "regular-pet",
				"namespace": "default",
			},
		},
	}

	target := checkResourceForCleanup(obj, "Pet")
	if target != nil {
		t.Error("checkResourceForCleanup() should return nil for resource without annotations")
	}
}

func TestCheckResourceForCleanup_NoMatchingAnnotations(t *testing.T) {
	origOneShot := cleanupOneShot
	origExpired := cleanupExpired
	origDiagnostic := cleanupDiagnostic
	cleanupOneShot = true
	cleanupExpired = true
	cleanupDiagnostic = false
	defer func() {
		cleanupOneShot = origOneShot
		cleanupExpired = origExpired
		cleanupDiagnostic = origDiagnostic
	}()

	// Resource with unrelated annotations
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "petstore.example.com/v1alpha1",
			"kind":       "Pet",
			"metadata": map[string]interface{}{
				"name":      "regular-pet",
				"namespace": "default",
				"annotations": map[string]interface{}{
					"some-other/annotation": "value",
				},
			},
		},
	}

	target := checkResourceForCleanup(obj, "Pet")
	if target != nil {
		t.Error("checkResourceForCleanup() should return nil for resource without matching annotations")
	}
}

func TestCheckResourceForCleanup_OneShotDisabled(t *testing.T) {
	origOneShot := cleanupOneShot
	origExpired := cleanupExpired
	origDiagnostic := cleanupDiagnostic
	cleanupOneShot = false // One-shot cleanup disabled
	cleanupExpired = false
	cleanupDiagnostic = false
	defer func() {
		cleanupOneShot = origOneShot
		cleanupExpired = origExpired
		cleanupDiagnostic = origDiagnostic
	}()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "petstore.example.com/v1alpha1",
			"kind":       "PetFindbystatusQuery",
			"metadata": map[string]interface{}{
				"name":      "test-query",
				"namespace": "default",
				"annotations": map[string]interface{}{
					"petstore.example.com/one-shot": "true",
				},
			},
		},
	}

	target := checkResourceForCleanup(obj, "PetFindbystatusQuery")
	if target != nil {
		t.Error("checkResourceForCleanup() should return nil when one-shot cleanup is disabled")
	}
}

// =============================================================================
// resolveQueryPlural tests (cleanup helper)
// =============================================================================

func TestResolveQueryPlural(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"singular petfindbystatus", "petfindbystatusquery", "petfindbystatusqueries"},
		{"plural petfindbystatus", "petfindbystatusqueries", "petfindbystatusqueries"},
		{"singular petfindbytags", "petfindbytagsquery", "petfindbytagsqueries"},
		{"plural petfindbytags", "petfindbytagsqueries", "petfindbytagsqueries"},
		{"singular storeinventory", "storeinventoryquery", "storeinventoryqueries"},
		{"plural storeinventory", "storeinventoryqueries", "storeinventoryqueries"},
		{"singular userlogin", "userloginquery", "userloginqueries"},
		{"plural userlogin", "userloginqueries", "userloginqueries"},
		{"singular userlogout", "userlogoutquery", "userlogoutqueries"},
		{"plural userlogout", "userlogoutqueries", "userlogoutqueries"},
		{"mixed case", "PetFindbystatusQuery", "petfindbystatusqueries"},
		{"unknown query", "unknownquery", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveQueryPlural(tt.input)
			if got != tt.expected {
				t.Errorf("resolveQueryPlural(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// resolveKindPlural tests (shared across commands)
// =============================================================================

func TestResolveKindPlural(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Resource kinds
		{"pet singular", "pet", "pets"},
		{"pet plural", "pets", "pets"},
		{"order singular", "order", "orders"},
		{"order plural", "orders", "orders"},
		{"user singular", "user", "users"},
		{"user plural", "users", "users"},

		// Query kinds
		{"petfindbystatusquery", "petfindbystatusquery", "petfindbystatusqueries"},
		{"storeinventoryquery", "storeinventoryquery", "storeinventoryqueries"},

		// Action kinds
		{"petuploadimageaction", "petuploadimageaction", "petuploadimageactions"},
		{"usercreatewithlistaction", "usercreatewithlistaction", "usercreatewithlistactions"},

		// Unknown
		{"unknown", "widget", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveKindPlural(tt.input)
			if got != tt.expected {
				t.Errorf("resolveKindPlural(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
