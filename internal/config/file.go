package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigFile represents the YAML configuration file structure.
// All fields are optional - CLI flags take precedence over config file values.
type ConfigFile struct {
	// Spec is the path or URL to the OpenAPI specification file
	Spec string `yaml:"spec,omitempty"`

	// Output is the directory where generated code will be written
	Output string `yaml:"output,omitempty"`

	// Group is the Kubernetes API group (e.g., "myapp.example.com")
	Group string `yaml:"group,omitempty"`

	// Version is the Kubernetes API version (e.g., "v1alpha1")
	Version string `yaml:"version,omitempty"`

	// Module is the Go module name for generated code
	Module string `yaml:"module,omitempty"`

	// Mapping determines how REST resources map to CRDs: "per-resource" or "single-crd"
	Mapping string `yaml:"mapping,omitempty"`

	// RootKind is the Kind name to use for the root "/" endpoint
	RootKind string `yaml:"rootKind,omitempty"`

	// GenerateCRDs controls whether to generate CRD YAML manifests directly
	GenerateCRDs *bool `yaml:"generateCRDs,omitempty"`

	// Aggregate controls whether to generate a Status Aggregator CRD
	Aggregate *bool `yaml:"aggregate,omitempty"`

	// Bundle controls whether to generate an Inline Composition Bundle CRD
	Bundle *bool `yaml:"bundle,omitempty"`

	// Filters contains path, tag, and operation filtering options
	Filters *FilterConfig `yaml:"filters,omitempty"`

	// IDMerge contains ID field merging options
	IDMerge *IDMergeConfig `yaml:"idMerge,omitempty"`

	// UpdateWithPost specifies which resources should use POST for updates when PUT is not available
	// Can be: ["*"] for all, or specific paths like ["/store/order", "/users/*"]
	UpdateWithPost []string `yaml:"updateWithPost,omitempty"`
}

// FilterConfig contains filtering options for paths, tags, and operations
type FilterConfig struct {
	// IncludePaths specifies paths to include (glob patterns supported)
	// Example: ["/users", "/pets", "/orders/*"]
	IncludePaths []string `yaml:"includePaths,omitempty"`

	// ExcludePaths specifies paths to exclude (glob patterns supported)
	// Example: ["/internal/*", "/admin/*"]
	ExcludePaths []string `yaml:"excludePaths,omitempty"`

	// IncludeTags specifies OpenAPI tags to include
	// Example: ["public", "v2"]
	IncludeTags []string `yaml:"includeTags,omitempty"`

	// ExcludeTags specifies OpenAPI tags to exclude
	// Example: ["deprecated", "internal"]
	ExcludeTags []string `yaml:"excludeTags,omitempty"`

	// IncludeOperations specifies operationIds to include (glob patterns supported)
	// Example: ["getPet*", "createPet"]
	IncludeOperations []string `yaml:"includeOperations,omitempty"`

	// ExcludeOperations specifies operationIds to exclude (glob patterns supported)
	// Example: ["*Deprecated", "deletePet"]
	ExcludeOperations []string `yaml:"excludeOperations,omitempty"`
}

// IDMergeConfig contains ID field merging options
type IDMergeConfig struct {
	// Disabled disables automatic ID field merging
	Disabled bool `yaml:"disabled,omitempty"`

	// FieldMap provides explicit mappings from path parameters to body fields
	// Example: {"orderId": "id", "petId": "id"}
	FieldMap map[string]string `yaml:"fieldMap,omitempty"`
}

// LoadConfigFile loads a configuration file from the specified path.
// Supports YAML format. Returns nil config if file doesn't exist.
func LoadConfigFile(path string) (*ConfigFile, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg ConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// FindConfigFile searches for a config file in standard locations.
// Returns the path to the first found config file, or empty string if none found.
// Search order:
//  1. .openapi-operator-gen.yaml in current directory
//  2. .openapi-operator-gen.yml in current directory
//  3. openapi-operator-gen.yaml in current directory
//  4. openapi-operator-gen.yml in current directory
func FindConfigFile() string {
	names := []string{
		".openapi-operator-gen.yaml",
		".openapi-operator-gen.yml",
		"openapi-operator-gen.yaml",
		"openapi-operator-gen.yml",
	}

	for _, name := range names {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}

	return ""
}

// MergeConfigFile merges config file values into the Config struct.
// CLI flags (non-zero values in cfg) take precedence over config file values.
func MergeConfigFile(cfg *Config, file *ConfigFile) {
	if file == nil {
		return
	}

	// Merge simple string fields (only if CLI didn't set them)
	if cfg.SpecPath == "" && file.Spec != "" {
		cfg.SpecPath = file.Spec
	}
	if cfg.OutputDir == "./generated" && file.Output != "" {
		// ./generated is the default, so override if config file specifies something
		cfg.OutputDir = file.Output
	}
	if cfg.APIGroup == "" && file.Group != "" {
		cfg.APIGroup = file.Group
	}
	if cfg.APIVersion == "v1alpha1" && file.Version != "" {
		// v1alpha1 is the default, so override if config file specifies something
		cfg.APIVersion = file.Version
	}
	if cfg.ModuleName == "github.com/bluecontainer/generated-operator" && file.Module != "" {
		// default module name, so override if config file specifies something
		cfg.ModuleName = file.Module
	}
	if cfg.MappingMode == PerResource && file.Mapping != "" {
		// per-resource is the default
		cfg.MappingMode = MappingMode(file.Mapping)
	}
	if cfg.RootKind == "" && file.RootKind != "" {
		cfg.RootKind = file.RootKind
	}

	// Merge boolean fields (only if config file explicitly sets them)
	if file.GenerateCRDs != nil && !cfg.GenerateCRDs {
		cfg.GenerateCRDs = *file.GenerateCRDs
	}
	if file.Aggregate != nil && !cfg.GenerateAggregate {
		cfg.GenerateAggregate = *file.Aggregate
	}
	if file.Bundle != nil && !cfg.GenerateBundle {
		cfg.GenerateBundle = *file.Bundle
	}

	// Merge UpdateWithPost (only if CLI didn't set it)
	if len(cfg.UpdateWithPost) == 0 && len(file.UpdateWithPost) > 0 {
		cfg.UpdateWithPost = file.UpdateWithPost
	}

	// Merge filter options
	if file.Filters != nil {
		if len(cfg.IncludePaths) == 0 && len(file.Filters.IncludePaths) > 0 {
			cfg.IncludePaths = file.Filters.IncludePaths
		}
		if len(cfg.ExcludePaths) == 0 && len(file.Filters.ExcludePaths) > 0 {
			cfg.ExcludePaths = file.Filters.ExcludePaths
		}
		if len(cfg.IncludeTags) == 0 && len(file.Filters.IncludeTags) > 0 {
			cfg.IncludeTags = file.Filters.IncludeTags
		}
		if len(cfg.ExcludeTags) == 0 && len(file.Filters.ExcludeTags) > 0 {
			cfg.ExcludeTags = file.Filters.ExcludeTags
		}
		if len(cfg.IncludeOperations) == 0 && len(file.Filters.IncludeOperations) > 0 {
			cfg.IncludeOperations = file.Filters.IncludeOperations
		}
		if len(cfg.ExcludeOperations) == 0 && len(file.Filters.ExcludeOperations) > 0 {
			cfg.ExcludeOperations = file.Filters.ExcludeOperations
		}
	}

	// Merge ID merge options
	if file.IDMerge != nil {
		if !cfg.NoIDMerge && file.IDMerge.Disabled {
			cfg.NoIDMerge = file.IDMerge.Disabled
		}
		if cfg.IDFieldMap == nil && len(file.IDMerge.FieldMap) > 0 {
			cfg.IDFieldMap = file.IDMerge.FieldMap
		}
	}
}

// GenerateExampleConfig generates an example configuration file content
func GenerateExampleConfig() string {
	return `# openapi-operator-gen configuration file
# All options can be overridden by CLI flags

# OpenAPI specification path or URL (required)
spec: ./api/openapi.yaml

# Output directory for generated code
output: ./generated

# Kubernetes API group (required)
group: myapp.example.com

# Kubernetes API version
version: v1alpha1

# Go module name for generated code
module: github.com/myorg/myapp-operator

# Resource mapping mode: per-resource or single-crd
mapping: per-resource

# Kind name for root "/" endpoint (derived from spec filename if not set)
# rootKind: MyApp

# Generate CRD YAML manifests directly (default: use controller-gen)
generateCRDs: false

# Generate a Status Aggregator CRD for observing multiple resources
aggregate: true

# Generate an Inline Composition Bundle CRD for creating multiple resources
bundle: true

# Use POST for updates when PUT is not available
# Can be ["*"] for all, or specific paths
updateWithPost:
  # - "*"
  # - /store/order
  # - /users/*

# Path, tag, and operation filtering
filters:
  # Only include paths matching these patterns (glob supported)
  includePaths:
    # - /users
    # - /pets/*
    # - /orders

  # Exclude paths matching these patterns
  excludePaths:
    # - /internal/*
    # - /admin/*

  # Only include endpoints with these OpenAPI tags
  includeTags:
    # - public
    # - v2

  # Exclude endpoints with these OpenAPI tags
  excludeTags:
    # - deprecated
    # - internal

  # Only include operations with these operationIds (glob supported)
  includeOperations:
    # - getPet*
    # - createPet

  # Exclude operations with these operationIds
  excludeOperations:
    # - *Deprecated
    # - deletePet

# ID field merging options
idMerge:
  # Disable automatic merging of path ID parameters with body 'id' fields
  disabled: false

  # Explicit path param to body field mappings
  fieldMap:
    # orderId: id
    # petId: id
`
}

// WriteExampleConfig writes an example config file to the specified path
func WriteExampleConfig(path string) error {
	// Check if file already exists
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists: %s", path)
	}

	// Create parent directories if needed
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	content := GenerateExampleConfig()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
