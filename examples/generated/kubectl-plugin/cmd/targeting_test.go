package cmd

import (
	"reflect"
	"testing"
)

// =============================================================================
// isTargetingFlag tests
// =============================================================================

func TestIsTargetingFlag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Targeting flags
		{"target-pod-ordinal", "target-pod-ordinal", true},
		{"target-base-url", "target-base-url", true},
		{"target-base-urls", "target-base-urls", true},
		{"target-statefulset", "target-statefulset", true},
		{"target-deployment", "target-deployment", true},
		{"target-pod", "target-pod", true},
		{"target-helm-release", "target-helm-release", true},
		{"target-namespace", "target-namespace", true},

		// Non-targeting flags (old names without target- prefix should not match)
		{"pod", "pod", false},
		{"base-url", "base-url", false},
		{"statefulset", "statefulset", false},
		{"deployment", "deployment", false},
		{"helm-release", "helm-release", false},
		{"name", "name", false},
		{"namespace", "namespace", false},
		{"output", "output", false},
		{"dry-run", "dry-run", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTargetingFlag(tt.input)
			if got != tt.expected {
				t.Errorf("isTargetingFlag(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// validateTargetingFlags tests
// =============================================================================

func TestValidateTargetingFlags(t *testing.T) {
	tests := []struct {
		name      string
		setup     func()
		wantError bool
	}{
		{
			"no flags set",
			func() { resetTargetingFlags() },
			false,
		},
		{
			"base-url only",
			func() {
				resetTargetingFlags()
				targetBaseURL = "http://localhost:8080"
			},
			false,
		},
		{
			"base-urls only",
			func() {
				resetTargetingFlags()
				targetBaseURLs = "http://host1:8080,http://host2:8080"
			},
			false,
		},
		{
			"base-url and base-urls mutually exclusive",
			func() {
				resetTargetingFlags()
				targetBaseURL = "http://localhost:8080"
				targetBaseURLs = "http://host1:8080,http://host2:8080"
			},
			true,
		},
		{
			"pod with statefulset is valid",
			func() {
				resetTargetingFlags()
				targetPodOrdinal = 0
				targetStatefulSet = "my-sts"
			},
			false,
		},
		{
			"pod with helm-release is valid",
			func() {
				resetTargetingFlags()
				targetPodOrdinal = 0
				targetHelmRelease = "my-release"
			},
			false,
		},
		{
			"pod without statefulset or helm-release is invalid",
			func() {
				resetTargetingFlags()
				targetPodOrdinal = 0
			},
			true,
		},
		{
			"pod with deployment only is invalid",
			func() {
				resetTargetingFlags()
				targetPodOrdinal = 1
				targetDeployment = "my-deploy"
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer resetTargetingFlags()

			err := validateTargetingFlags()
			if (err != nil) != tt.wantError {
				t.Errorf("validateTargetingFlags() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// =============================================================================
// buildTargetSpec tests
// =============================================================================

func TestBuildTargetSpec_NoFlags(t *testing.T) {
	resetTargetingFlags()
	defer resetTargetingFlags()

	target := buildTargetSpec()
	if target != nil {
		t.Errorf("buildTargetSpec() = %v, want nil when no flags set", target)
	}
}

func TestBuildTargetSpec_PodOrdinal(t *testing.T) {
	resetTargetingFlags()
	targetPodOrdinal = 3
	defer resetTargetingFlags()

	target := buildTargetSpec()
	if target == nil {
		t.Fatal("buildTargetSpec() = nil, want map with podOrdinal")
	}
	if target["podOrdinal"] != 3 {
		t.Errorf("target[podOrdinal] = %v, want 3", target["podOrdinal"])
	}
}

func TestBuildTargetSpec_BaseURL(t *testing.T) {
	resetTargetingFlags()
	targetBaseURL = "http://localhost:8080"
	defer resetTargetingFlags()

	target := buildTargetSpec()
	if target == nil {
		t.Fatal("buildTargetSpec() = nil, want map with baseURL")
	}
	if target["baseURL"] != "http://localhost:8080" {
		t.Errorf("target[baseURL] = %v, want %q", target["baseURL"], "http://localhost:8080")
	}
}

func TestBuildTargetSpec_BaseURLs(t *testing.T) {
	resetTargetingFlags()
	targetBaseURLs = "http://host1:8080, http://host2:8080"
	defer resetTargetingFlags()

	target := buildTargetSpec()
	if target == nil {
		t.Fatal("buildTargetSpec() = nil, want map with baseURLs")
	}
	urls, ok := target["baseURLs"].([]interface{})
	if !ok {
		t.Fatalf("target[baseURLs] type = %T, want []interface{}", target["baseURLs"])
	}
	expected := []interface{}{"http://host1:8080", "http://host2:8080"}
	if !reflect.DeepEqual(urls, expected) {
		t.Errorf("target[baseURLs] = %v, want %v", urls, expected)
	}
}

func TestBuildTargetSpec_StatefulSet(t *testing.T) {
	resetTargetingFlags()
	targetStatefulSet = "my-statefulset"
	defer resetTargetingFlags()

	target := buildTargetSpec()
	if target == nil {
		t.Fatal("buildTargetSpec() = nil, want map with statefulSet")
	}
	if target["statefulSet"] != "my-statefulset" {
		t.Errorf("target[statefulSet] = %v, want %q", target["statefulSet"], "my-statefulset")
	}
}

func TestBuildTargetSpec_Deployment(t *testing.T) {
	resetTargetingFlags()
	targetDeployment = "my-deploy"
	defer resetTargetingFlags()

	target := buildTargetSpec()
	if target == nil {
		t.Fatal("buildTargetSpec() = nil, want map with deployment")
	}
	if target["deployment"] != "my-deploy" {
		t.Errorf("target[deployment] = %v, want %q", target["deployment"], "my-deploy")
	}
}

func TestBuildTargetSpec_TargetPod(t *testing.T) {
	resetTargetingFlags()
	targetPod = "my-pod-0"
	defer resetTargetingFlags()

	target := buildTargetSpec()
	if target == nil {
		t.Fatal("buildTargetSpec() = nil, want map with pod")
	}
	if target["pod"] != "my-pod-0" {
		t.Errorf("target[pod] = %v, want %q", target["pod"], "my-pod-0")
	}
}

func TestBuildTargetSpec_HelmRelease(t *testing.T) {
	resetTargetingFlags()
	targetHelmRelease = "my-release"
	defer resetTargetingFlags()

	target := buildTargetSpec()
	if target == nil {
		t.Fatal("buildTargetSpec() = nil, want map with helmRelease")
	}
	if target["helmRelease"] != "my-release" {
		t.Errorf("target[helmRelease] = %v, want %q", target["helmRelease"], "my-release")
	}
}

func TestBuildTargetSpec_TargetNamespace(t *testing.T) {
	resetTargetingFlags()
	targetNamespace = "other-ns"
	defer resetTargetingFlags()

	target := buildTargetSpec()
	if target == nil {
		t.Fatal("buildTargetSpec() = nil, want map with namespace")
	}
	if target["namespace"] != "other-ns" {
		t.Errorf("target[namespace] = %v, want %q", target["namespace"], "other-ns")
	}
}

func TestBuildTargetSpec_MultipleFlags(t *testing.T) {
	resetTargetingFlags()
	targetPodOrdinal = 0
	targetStatefulSet = "my-sts"
	targetNamespace = "prod"
	defer resetTargetingFlags()

	target := buildTargetSpec()
	if target == nil {
		t.Fatal("buildTargetSpec() = nil, want map with multiple fields")
	}
	if target["podOrdinal"] != 0 {
		t.Errorf("target[podOrdinal] = %v, want 0", target["podOrdinal"])
	}
	if target["statefulSet"] != "my-sts" {
		t.Errorf("target[statefulSet] = %v, want %q", target["statefulSet"], "my-sts")
	}
	if target["namespace"] != "prod" {
		t.Errorf("target[namespace] = %v, want %q", target["namespace"], "prod")
	}
	if len(target) != 3 {
		t.Errorf("target has %d fields, want 3", len(target))
	}
}

// =============================================================================
// resetTargetingFlags tests
// =============================================================================

func TestResetTargetingFlags(t *testing.T) {
	// Set all flags to non-default values
	targetPodOrdinal = 5
	targetBaseURL = "http://example.com"
	targetBaseURLs = "http://a.com,http://b.com"
	targetStatefulSet = "sts"
	targetDeployment = "deploy"
	targetPod = "pod-0"
	targetHelmRelease = "release"
	targetNamespace = "ns"

	resetTargetingFlags()

	if targetPodOrdinal != -1 {
		t.Errorf("targetPodOrdinal = %d, want -1", targetPodOrdinal)
	}
	if targetBaseURL != "" {
		t.Errorf("targetBaseURL = %q, want empty", targetBaseURL)
	}
	if targetBaseURLs != "" {
		t.Errorf("targetBaseURLs = %q, want empty", targetBaseURLs)
	}
	if targetStatefulSet != "" {
		t.Errorf("targetStatefulSet = %q, want empty", targetStatefulSet)
	}
	if targetDeployment != "" {
		t.Errorf("targetDeployment = %q, want empty", targetDeployment)
	}
	if targetPod != "" {
		t.Errorf("targetPod = %q, want empty", targetPod)
	}
	if targetHelmRelease != "" {
		t.Errorf("targetHelmRelease = %q, want empty", targetHelmRelease)
	}
	if targetNamespace != "" {
		t.Errorf("targetNamespace = %q, want empty", targetNamespace)
	}
}
