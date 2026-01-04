package config

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
	return nil
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
