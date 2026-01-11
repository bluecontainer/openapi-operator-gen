package config

import (
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// MappingMode defines how REST resources map to CRDs
type MappingMode string

const (
	// PerResource creates one CRD per REST resource
	PerResource MappingMode = "per-resource"
	// SingleCRD creates one CRD for the entire API
	SingleCRD MappingMode = "single-crd"
)

// Config holds the generator configuration
type Config struct {
	// SpecPath is the path to the OpenAPI specification file
	SpecPath string
	// OutputDir is the directory where generated code will be written
	OutputDir string
	// APIGroup is the Kubernetes API group (e.g., "myapp.example.com")
	APIGroup string
	// APIVersion is the Kubernetes API version (e.g., "v1alpha1")
	APIVersion string
	// MappingMode determines how REST resources map to CRDs
	MappingMode MappingMode
	// ModuleName is the Go module name for generated code
	ModuleName string
	// GenerateCRDs controls whether to generate CRD YAML manifests directly.
	// When false (default), CRDs should be generated using controller-gen.
	GenerateCRDs bool
	// RootKind is the Kind name to use for the root "/" endpoint.
	// If not specified, it's derived from the OpenAPI spec file name.
	RootKind string
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.SpecPath == "" {
		return &ValidationError{Field: "SpecPath", Message: "OpenAPI spec path is required"}
	}
	if c.OutputDir == "" {
		return &ValidationError{Field: "OutputDir", Message: "output directory is required"}
	}
	if c.APIGroup == "" {
		return &ValidationError{Field: "APIGroup", Message: "API group is required"}
	}
	if c.APIVersion == "" {
		c.APIVersion = "v1alpha1"
	}
	if c.MappingMode == "" {
		c.MappingMode = PerResource
	}
	if c.ModuleName == "" {
		c.ModuleName = "github.com/bluecontainer/generated-operator"
	}
	// Derive RootKind from spec file name if not provided
	if c.RootKind == "" {
		c.RootKind = c.deriveRootKindFromSpecPath()
	}
	return nil
}

// deriveRootKindFromSpecPath extracts a Kind name from the spec file name or URL
// e.g., "petstore.yaml" -> "Petstore", "my-api.json" -> "MyApi"
// e.g., "https://example.com/api/petstore.yaml" -> "Petstore"
func (c *Config) deriveRootKindFromSpecPath() string {
	var base string

	// Check if it's a URL
	if strings.HasPrefix(c.SpecPath, "http://") || strings.HasPrefix(c.SpecPath, "https://") {
		parsedURL, err := url.Parse(c.SpecPath)
		if err != nil {
			// Fall back to using the whole string
			base = c.SpecPath
		} else {
			// Get the filename from the URL path
			urlPath := parsedURL.Path
			if urlPath == "" || urlPath == "/" {
				return ""
			}
			base = path.Base(urlPath)
		}
	} else {
		// Get base name without directory for file paths
		base = filepath.Base(c.SpecPath)
	}

	// Remove extension
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// Remove version suffixes like ".1.0.27" or "-v1"
	// Handle patterns like "petstore.1.0.27" or "api-v2"
	for {
		newExt := filepath.Ext(name)
		if newExt == "" {
			break
		}
		// Check if extension looks like a version number
		trimmed := strings.TrimPrefix(newExt, ".")
		if isVersionLike(trimmed) {
			name = strings.TrimSuffix(name, newExt)
		} else {
			break
		}
	}

	// Convert to PascalCase
	return toPascalCase(name)
}

// isVersionLike checks if a string looks like a version number
func isVersionLike(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Check if it starts with a digit or is like "v1", "v2", etc.
	if s[0] >= '0' && s[0] <= '9' {
		return true
	}
	if len(s) >= 2 && (s[0] == 'v' || s[0] == 'V') && s[1] >= '0' && s[1] <= '9' {
		return true
	}
	return false
}

// toPascalCase converts a string to PascalCase
func toPascalCase(s string) string {
	// Split by common separators
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, ".", " ")
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, "")
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
