package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
spec: ./api/openapi.yaml
output: ./my-output
group: test.example.com
version: v1beta1
module: github.com/test/operator
mapping: single-crd
aggregate: true
bundle: true
updateWithPost:
  - /store/order
  - /users/*
filters:
  includePaths:
    - /users
    - /pets/*
  excludeTags:
    - deprecated
idMerge:
  disabled: true
  fieldMap:
    orderId: id
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFile failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}

	// Verify values
	if cfg.Spec != "./api/openapi.yaml" {
		t.Errorf("expected spec './api/openapi.yaml', got %q", cfg.Spec)
	}
	if cfg.Output != "./my-output" {
		t.Errorf("expected output './my-output', got %q", cfg.Output)
	}
	if cfg.Group != "test.example.com" {
		t.Errorf("expected group 'test.example.com', got %q", cfg.Group)
	}
	if cfg.Version != "v1beta1" {
		t.Errorf("expected version 'v1beta1', got %q", cfg.Version)
	}
	if cfg.Module != "github.com/test/operator" {
		t.Errorf("expected module 'github.com/test/operator', got %q", cfg.Module)
	}
	if cfg.Mapping != "single-crd" {
		t.Errorf("expected mapping 'single-crd', got %q", cfg.Mapping)
	}
	if cfg.Aggregate == nil || !*cfg.Aggregate {
		t.Error("expected aggregate to be true")
	}
	if cfg.Bundle == nil || !*cfg.Bundle {
		t.Error("expected bundle to be true")
	}
	if len(cfg.UpdateWithPost) != 2 {
		t.Errorf("expected 2 updateWithPost entries, got %d", len(cfg.UpdateWithPost))
	}
	if cfg.Filters == nil {
		t.Fatal("expected filters, got nil")
	}
	if len(cfg.Filters.IncludePaths) != 2 {
		t.Errorf("expected 2 includePaths, got %d", len(cfg.Filters.IncludePaths))
	}
	if len(cfg.Filters.ExcludeTags) != 1 {
		t.Errorf("expected 1 excludeTag, got %d", len(cfg.Filters.ExcludeTags))
	}
	if cfg.IDMerge == nil {
		t.Fatal("expected idMerge, got nil")
	}
	if !cfg.IDMerge.Disabled {
		t.Error("expected idMerge.disabled to be true")
	}
	if cfg.IDMerge.FieldMap["orderId"] != "id" {
		t.Errorf("expected fieldMap['orderId']='id', got %q", cfg.IDMerge.FieldMap["orderId"])
	}
}

func TestLoadConfigFile_NotFound(t *testing.T) {
	cfg, err := LoadConfigFile("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config for missing file")
	}
}

func TestMergeConfigFile(t *testing.T) {
	// Start with default CLI config
	cfg := &Config{
		OutputDir:   "./generated",
		APIVersion:  "v1alpha1",
		MappingMode: PerResource,
		ModuleName:  "github.com/bluecontainer/generated-operator",
	}

	// Config file with some values
	aggregate := true
	fileCfg := &ConfigFile{
		Spec:      "./api/openapi.yaml",
		Group:     "test.example.com",
		Output:    "./custom-output",
		Aggregate: &aggregate,
		Filters: &FilterConfig{
			IncludePaths: []string{"/users", "/pets"},
		},
	}

	MergeConfigFile(cfg, fileCfg)

	// Check merged values
	if cfg.SpecPath != "./api/openapi.yaml" {
		t.Errorf("expected spec './api/openapi.yaml', got %q", cfg.SpecPath)
	}
	if cfg.APIGroup != "test.example.com" {
		t.Errorf("expected group 'test.example.com', got %q", cfg.APIGroup)
	}
	if cfg.OutputDir != "./custom-output" {
		t.Errorf("expected output './custom-output', got %q", cfg.OutputDir)
	}
	if !cfg.GenerateAggregate {
		t.Error("expected aggregate to be true")
	}
	if len(cfg.IncludePaths) != 2 {
		t.Errorf("expected 2 includePaths, got %d", len(cfg.IncludePaths))
	}

	// Check defaults preserved
	if cfg.APIVersion != "v1alpha1" {
		t.Errorf("expected version 'v1alpha1' preserved, got %q", cfg.APIVersion)
	}
}

func TestMergeConfigFile_CLIOverrides(t *testing.T) {
	// CLI config with explicit values (should not be overridden)
	cfg := &Config{
		SpecPath:    "./cli-spec.yaml",
		APIGroup:    "cli.example.com",
		OutputDir:   "./cli-output",
		APIVersion:  "v1alpha1",
		MappingMode: PerResource,
		ModuleName:  "github.com/bluecontainer/generated-operator",
	}

	// Config file with different values
	fileCfg := &ConfigFile{
		Spec:   "./file-spec.yaml",
		Group:  "file.example.com",
		Output: "./file-output",
	}

	MergeConfigFile(cfg, fileCfg)

	// CLI values should be preserved
	if cfg.SpecPath != "./cli-spec.yaml" {
		t.Errorf("expected CLI spec preserved, got %q", cfg.SpecPath)
	}
	if cfg.APIGroup != "cli.example.com" {
		t.Errorf("expected CLI group preserved, got %q", cfg.APIGroup)
	}
	// OutputDir is a special case - ./generated is the default, but ./cli-output is not
	// So it should be preserved since it was explicitly set
	if cfg.OutputDir != "./cli-output" {
		t.Errorf("expected CLI output preserved, got %q", cfg.OutputDir)
	}
}

func TestFindConfigFile(t *testing.T) {
	// Create a temp directory and change to it
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(tmpDir)

	// No config file should be found initially
	if found := FindConfigFile(); found != "" {
		t.Errorf("expected no config file, found %q", found)
	}

	// Create a config file
	configPath := filepath.Join(tmpDir, ".openapi-operator-gen.yaml")
	if err := os.WriteFile(configPath, []byte("spec: test.yaml"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Should find it now
	if found := FindConfigFile(); found != ".openapi-operator-gen.yaml" {
		t.Errorf("expected '.openapi-operator-gen.yaml', found %q", found)
	}
}

func TestWriteExampleConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	if err := WriteExampleConfig(configPath); err != nil {
		t.Fatalf("WriteExampleConfig failed: %v", err)
	}

	// Check file exists
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// Try to write again - should fail
	if err := WriteExampleConfig(configPath); err == nil {
		t.Error("expected error when file already exists")
	}

	// Read and verify content has key fields
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	contentStr := string(content)
	expectedFields := []string{"spec:", "output:", "group:", "version:", "module:", "aggregate:", "bundle:", "filters:", "idMerge:"}
	for _, field := range expectedFields {
		if !contains(contentStr, field) {
			t.Errorf("expected config to contain %q", field)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
