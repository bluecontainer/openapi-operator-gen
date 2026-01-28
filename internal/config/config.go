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
	// GeneratorVersion is the version of openapi-operator-gen used to generate the code.
	// This is embedded in the generated go.mod to ensure correct dependency versions.
	GeneratorVersion string
	// CommitHash is the git commit hash (short form, at least 12 chars) for pseudo-version generation.
	CommitHash string
	// CommitTimestamp is the commit timestamp in YYYYMMDDHHMMSS format for pseudo-version generation.
	CommitTimestamp string
	// GenerateAggregate controls whether to generate a Status Aggregator CRD.
	// When true, generates an aggregate CRD that observes and aggregates status from other CRDs.
	GenerateAggregate bool
	// GenerateBundle controls whether to generate an Inline Composition Bundle CRD (Option 2).
	// When true, generates a bundle CRD that creates and manages multiple child resources.
	GenerateBundle bool

	// GenerateKubectlPlugin controls whether to generate a kubectl plugin alongside the operator.
	// When true, generates a kubectl plugin for managing and diagnosing operator resources.
	GenerateKubectlPlugin bool

	// UpdateWithPost specifies which resources should use POST for updates when PUT is not available.
	// Can be:
	// - Empty: disabled (default)
	// - ["*"]: enabled for all resources without PUT
	// - ["/store/order", "/users"]: enabled only for specific paths (glob patterns supported)
	// This is useful for APIs that use POST for both creation and updates.
	UpdateWithPost []string

	// Resource Filtering Options
	// IncludePaths specifies paths to include (glob patterns supported).
	// If set, only paths matching these patterns will be processed.
	// Example: "/users,/pets,/orders/*"
	IncludePaths []string
	// ExcludePaths specifies paths to exclude (glob patterns supported).
	// Paths matching these patterns will be skipped even if they match IncludePaths.
	// Example: "/internal/*,/admin/*"
	ExcludePaths []string
	// IncludeTags specifies OpenAPI tags to include.
	// If set, only endpoints with at least one matching tag will be processed.
	// Example: "public,v2"
	IncludeTags []string
	// ExcludeTags specifies OpenAPI tags to exclude.
	// Endpoints with any matching tag will be skipped.
	// Example: "deprecated,internal"
	ExcludeTags []string
	// IncludeOperations specifies operationIds to include.
	// If set, only operations with matching operationIds will be processed.
	// Supports glob patterns: "get*", "create*", "Pet*"
	// Example: "getPetById,createPet,updatePet"
	IncludeOperations []string
	// ExcludeOperations specifies operationIds to exclude.
	// Operations with matching operationIds will be skipped.
	// Supports glob patterns: "*Deprecated", "internal*"
	// Example: "deletePet,deprecatedGetPets"
	ExcludeOperations []string

	// ID Field Merging Options
	// NoIDMerge disables automatic ID field merging.
	// When false (default), the generator automatically merges path parameters like {orderId}
	// with body fields named "id" when they represent the same value.
	NoIDMerge bool
	// IDFieldMap provides explicit mappings from path parameters to body fields.
	// Format: "pathParam=bodyField" (e.g., "orderId=id", "petId=id")
	// This overrides auto-detection for specific parameters.
	IDFieldMap map[string]string
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

// ShouldUpdateWithPost checks if a given path should use POST for updates.
// Returns true if:
// - UpdateWithPost contains "*" (all resources)
// - UpdateWithPost contains a pattern that matches the path
func (c *Config) ShouldUpdateWithPost(resourcePath string) bool {
	if len(c.UpdateWithPost) == 0 {
		return false
	}
	for _, pattern := range c.UpdateWithPost {
		if pattern == "*" {
			return true
		}
		if matchPath(pattern, resourcePath) {
			return true
		}
	}
	return false
}

// GetIDFieldMapping returns the body field name that a path parameter should be merged with.
// It checks in order:
// 1. Explicit IDFieldMap configuration
// 2. OpenAPI x-k8s-id-field extension (passed as extensionMapping)
// 3. Auto-detection based on naming conventions (if NoIDMerge is false)
//
// Returns the body field name to merge with, or empty string if no merge should happen.
func (c *Config) GetIDFieldMapping(pathParam string, kindName string, extensionMapping string) string {
	// 1. Check explicit IDFieldMap configuration (highest priority)
	if c.IDFieldMap != nil {
		if bodyField, ok := c.IDFieldMap[pathParam]; ok {
			return bodyField
		}
	}

	// 2. Check OpenAPI x-k8s-id-field extension
	if extensionMapping != "" {
		return extensionMapping
	}

	// 3. Auto-detection (if enabled)
	if !c.NoIDMerge {
		return c.autoDetectIDFieldMapping(pathParam, kindName)
	}

	return ""
}

// autoDetectIDFieldMapping detects if a path parameter should be merged with a body "id" field.
// It uses naming convention heuristics:
// - {kindName}Id -> id (e.g., orderId -> id for Order kind)
// - {kindName}ID -> id (e.g., orderID -> id for Order kind)
func (c *Config) autoDetectIDFieldMapping(pathParam string, kindName string) string {
	// Normalize for comparison
	paramLower := strings.ToLower(pathParam)
	kindLower := strings.ToLower(kindName)

	// Check if path param follows the pattern {kind}Id or {kind}ID
	// e.g., orderId, petId, userId -> maps to "id"
	if paramLower == kindLower+"id" {
		return "id"
	}

	return ""
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

// PathFilter provides path, tag, and operationId filtering functionality
type PathFilter struct {
	includePaths      []string
	excludePaths      []string
	includeTags       []string
	excludeTags       []string
	includeOperations []string
	excludeOperations []string
}

// NewPathFilter creates a new PathFilter from config
func NewPathFilter(cfg *Config) *PathFilter {
	return &PathFilter{
		includePaths:      cfg.IncludePaths,
		excludePaths:      cfg.ExcludePaths,
		includeTags:       cfg.IncludeTags,
		excludeTags:       cfg.ExcludeTags,
		includeOperations: cfg.IncludeOperations,
		excludeOperations: cfg.ExcludeOperations,
	}
}

// HasFilters returns true if any filters are configured
func (f *PathFilter) HasFilters() bool {
	return len(f.includePaths) > 0 || len(f.excludePaths) > 0 ||
		len(f.includeTags) > 0 || len(f.excludeTags) > 0 ||
		len(f.includeOperations) > 0 || len(f.excludeOperations) > 0
}

// HasOperationFilters returns true if operationId filters are configured
func (f *PathFilter) HasOperationFilters() bool {
	return len(f.includeOperations) > 0 || len(f.excludeOperations) > 0
}

// ShouldIncludePath returns true if the path should be included based on path filters
// Note: This only checks path patterns, not tags. Use ShouldInclude for full filtering.
func (f *PathFilter) ShouldIncludePath(path string) bool {
	// If no path filters configured, include all
	if len(f.includePaths) == 0 && len(f.excludePaths) == 0 {
		return true
	}

	// Check exclude patterns first (exclude takes precedence)
	for _, pattern := range f.excludePaths {
		if matchPath(pattern, path) {
			return false
		}
	}

	// If include patterns are specified, path must match at least one
	if len(f.includePaths) > 0 {
		for _, pattern := range f.includePaths {
			if matchPath(pattern, path) {
				return true
			}
		}
		return false // No include pattern matched
	}

	return true
}

// ShouldIncludeTags returns true if the tags pass the tag filter
// tags parameter is the list of OpenAPI tags on the operation
func (f *PathFilter) ShouldIncludeTags(tags []string) bool {
	// If no tag filters configured, include all
	if len(f.includeTags) == 0 && len(f.excludeTags) == 0 {
		return true
	}

	// Check exclude tags first (exclude takes precedence)
	for _, excludeTag := range f.excludeTags {
		for _, tag := range tags {
			if strings.EqualFold(tag, excludeTag) {
				return false
			}
		}
	}

	// If include tags are specified, operation must have at least one matching tag
	if len(f.includeTags) > 0 {
		for _, includeTag := range f.includeTags {
			for _, tag := range tags {
				if strings.EqualFold(tag, includeTag) {
					return true
				}
			}
		}
		return false // No include tag matched
	}

	return true
}

// ShouldIncludeOperation returns true if the operationId passes the operation filter
func (f *PathFilter) ShouldIncludeOperation(operationID string) bool {
	// If no operation filters configured, include all
	if len(f.includeOperations) == 0 && len(f.excludeOperations) == 0 {
		return true
	}

	// Check exclude patterns first (exclude takes precedence)
	for _, pattern := range f.excludeOperations {
		if matchOperationID(pattern, operationID) {
			return false
		}
	}

	// If include patterns are specified, operationId must match at least one
	if len(f.includeOperations) > 0 {
		for _, pattern := range f.includeOperations {
			if matchOperationID(pattern, operationID) {
				return true
			}
		}
		return false // No include pattern matched
	}

	return true
}

// ShouldInclude returns true if the path and tags pass all filters
// Note: This does not check operationId - use ShouldIncludeWithOperations for full filtering
func (f *PathFilter) ShouldInclude(path string, tags []string) bool {
	return f.ShouldIncludePath(path) && f.ShouldIncludeTags(tags)
}

// ShouldIncludeWithOperations returns true if path, tags, and all operationIds pass filters
// operationIDs is a list of all operationIds for the path - if any passes, the path is included
func (f *PathFilter) ShouldIncludeWithOperations(path string, tags []string, operationIDs []string) bool {
	// First check path and tags
	if !f.ShouldIncludePath(path) || !f.ShouldIncludeTags(tags) {
		return false
	}

	// If no operation filters, we're done
	if !f.HasOperationFilters() {
		return true
	}

	// Check if at least one operationId passes the filter
	for _, opID := range operationIDs {
		if opID != "" && f.ShouldIncludeOperation(opID) {
			return true
		}
	}

	// If there are operation filters but no operationIds provided, check if we should include
	// paths with no operationIds (include them if there's no include filter)
	if len(operationIDs) == 0 || allEmpty(operationIDs) {
		return len(f.includeOperations) == 0
	}

	return false
}

// allEmpty returns true if all strings in the slice are empty
func allEmpty(s []string) bool {
	for _, v := range s {
		if v != "" {
			return false
		}
	}
	return true
}

// matchOperationID matches an operationId against a pattern supporting glob wildcards
// Patterns:
//   - Exact match: "getPetById" matches "getPetById"
//   - Prefix match: "get*" matches "getPetById", "getUser"
//   - Suffix match: "*Pet" matches "getPet", "createPet"
//   - Contains match: "*Pet*" matches "getPetById", "updatePetStatus"
func matchOperationID(pattern, operationID string) bool {
	// Empty pattern or operationId
	if pattern == "" || operationID == "" {
		return pattern == operationID
	}

	// Exact match (case-sensitive for operationIds)
	if pattern == operationID {
		return true
	}

	// Prefix match: pattern*
	if strings.HasSuffix(pattern, "*") && !strings.HasPrefix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(operationID, prefix)
	}

	// Suffix match: *pattern
	if strings.HasPrefix(pattern, "*") && !strings.HasSuffix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(operationID, suffix)
	}

	// Contains match: *pattern*
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		middle := strings.TrimPrefix(strings.TrimSuffix(pattern, "*"), "*")
		return strings.Contains(operationID, middle)
	}

	// Complex glob pattern (e.g., "get*ById")
	if strings.Contains(pattern, "*") {
		return matchGlobString(pattern, operationID)
	}

	return false
}

// matchGlobString performs glob-style matching on strings
func matchGlobString(pattern, s string) bool {
	// Split pattern by * and match segments
	parts := strings.Split(pattern, "*")
	if len(parts) == 0 {
		return true
	}

	// First part must be a prefix
	if parts[0] != "" && !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = strings.TrimPrefix(s, parts[0])

	// Middle parts must be found in order
	for i := 1; i < len(parts)-1; i++ {
		if parts[i] == "" {
			continue
		}
		idx := strings.Index(s, parts[i])
		if idx == -1 {
			return false
		}
		s = s[idx+len(parts[i]):]
	}

	// Last part must be a suffix
	if len(parts) > 1 && parts[len(parts)-1] != "" {
		return strings.HasSuffix(s, parts[len(parts)-1])
	}

	return true
}

// matchPath matches a path against a pattern supporting glob wildcards
// Patterns:
//   - Exact match: "/users" matches "/users"
//   - Prefix match: "/users/*" matches "/users/123", "/users/123/profile"
//   - Single segment: "/users/?" matches "/users/123" but not "/users/123/profile"
//   - Path parameter agnostic: Pattern and path both normalized (path params treated as wildcards)
func matchPath(pattern, path string) bool {
	// Normalize: remove trailing slashes for comparison
	pattern = strings.TrimSuffix(pattern, "/")
	path = strings.TrimSuffix(path, "/")

	// Handle exact match
	if pattern == path {
		return true
	}

	// Handle wildcard patterns
	if strings.HasSuffix(pattern, "/*") {
		// Prefix match: /users/* matches /users/anything
		prefix := strings.TrimSuffix(pattern, "/*")
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}

	if strings.HasSuffix(pattern, "/?") {
		// Single segment match: /users/? matches /users/123 but not /users/123/profile
		prefix := strings.TrimSuffix(pattern, "/?")
		if !strings.HasPrefix(path, prefix+"/") {
			return false
		}
		remainder := strings.TrimPrefix(path, prefix+"/")
		return !strings.Contains(remainder, "/")
	}

	// Handle glob patterns with * in the middle (e.g., /api/*/users)
	if strings.Contains(pattern, "*") {
		return matchGlob(pattern, path)
	}

	return false
}

// matchGlob performs glob-style matching
func matchGlob(pattern, path string) bool {
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	return matchGlobParts(patternParts, pathParts)
}

// matchGlobParts recursively matches pattern parts against path parts
// For glob matching within paths, * matches exactly one segment (not zero)
func matchGlobParts(pattern, path []string) bool {
	for len(pattern) > 0 && len(path) > 0 {
		p := pattern[0]

		if p == "*" {
			// In path context, * matches exactly one segment
			pattern = pattern[1:]
			path = path[1:]
			continue
		}

		if p == "?" || p == path[0] || (strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}")) {
			// Single segment match, exact match, or path parameter placeholder
			pattern = pattern[1:]
			path = path[1:]
			continue
		}

		return false
	}

	// Both exhausted = match
	// Either one remaining = no match (for strict segment matching)
	return len(pattern) == 0 && len(path) == 0
}
