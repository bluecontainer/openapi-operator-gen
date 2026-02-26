package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

// isURL checks if the given path is a URL
func isURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// readSpec reads the spec content from a file or URL
func readSpec(specPath string) ([]byte, error) {
	if isURL(specPath) {
		resp, err := http.Get(specPath)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch spec from URL: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch spec: HTTP %d", resp.StatusCode)
		}

		return io.ReadAll(resp.Body)
	}
	return os.ReadFile(specPath)
}

// detectSpecVersion detects whether the spec is Swagger 2.0 or OpenAPI 3.x
// Returns "2.0" for Swagger 2.0, "3.x" for OpenAPI 3.0/3.1
func detectSpecVersion(data []byte) string {
	// Check for swagger key (Swagger 2.0)
	if bytes.Contains(data, []byte(`"swagger"`)) || bytes.Contains(data, []byte(`swagger:`)) {
		return "2.0"
	}
	// Default to OpenAPI 3.x
	return "3.x"
}

// parseSwagger2 parses a Swagger 2.0 spec and converts it to OpenAPI 3.0
func parseSwagger2(data []byte) (*openapi3.T, error) {
	var swagger openapi2.T

	// Try JSON first
	if err := json.Unmarshal(data, &swagger); err != nil {
		// If JSON fails, convert YAML to JSON first then parse
		// This avoids YAML unmarshalling issues with kin-openapi types
		var yamlData interface{}
		if err := yaml.Unmarshal(data, &yamlData); err != nil {
			return nil, fmt.Errorf("failed to parse Swagger 2.0 spec as YAML: %w", err)
		}

		// Convert map[interface{}]interface{} to map[string]interface{} for JSON marshalling
		converted := convertYAMLMapKeys(yamlData)

		// Normalize "type" fields from string to array format for openapi3.Types compatibility
		// Swagger 2.0 uses "type": "string" but openapi3.Types expects array format
		normalizeTypeFields(converted)

		// Convert to JSON
		jsonData, err := json.Marshal(converted)
		if err != nil {
			return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
		}

		// Parse as JSON
		if err := json.Unmarshal(jsonData, &swagger); err != nil {
			return nil, fmt.Errorf("failed to parse Swagger 2.0 spec: %w", err)
		}
	}

	// Convert to OpenAPI 3.0
	doc, err := openapi2conv.ToV3(&swagger)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Swagger 2.0 to OpenAPI 3.0: %w", err)
	}

	return doc, nil
}

// normalizeTypeFields recursively finds "type" fields and converts string values to arrays
// This is needed because openapi3.Types expects array format but Swagger 2.0 uses string format
func normalizeTypeFields(v interface{}) {
	switch x := v.(type) {
	case map[string]interface{}:
		for k, val := range x {
			if k == "type" {
				// Convert string type to array format
				if s, ok := val.(string); ok {
					x[k] = []interface{}{s}
				}
			} else {
				normalizeTypeFields(val)
			}
		}
	case []interface{}:
		for _, item := range x {
			normalizeTypeFields(item)
		}
	}
}

// convertYAMLMapKeys recursively converts map[interface{}]interface{} to map[string]interface{}
// This is needed because YAML unmarshalling creates interface{} keys which JSON can't handle
func convertYAMLMapKeys(v interface{}) interface{} {
	switch x := v.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for k, val := range x {
			m[fmt.Sprintf("%v", k)] = convertYAMLMapKeys(val)
		}
		return m
	case map[string]interface{}:
		m := make(map[string]interface{})
		for k, val := range x {
			m[k] = convertYAMLMapKeys(val)
		}
		return m
	case []interface{}:
		for i, val := range x {
			x[i] = convertYAMLMapKeys(val)
		}
		return x
	default:
		return v
	}
}

// Resource represents a REST API resource extracted from OpenAPI spec
type Resource struct {
	Name        string
	PluralName  string
	Path        string
	Operations  []Operation
	Schema      *Schema
	Description string
}

// Operation represents an HTTP operation on a resource
type Operation struct {
	Method       string
	Path         string
	OperationID  string
	Summary      string
	RequestBody  *Schema
	ResponseBody *Schema
	PathParams   []Parameter
	QueryParams  []Parameter
}

// Parameter represents an API parameter
type Parameter struct {
	Name        string
	In          string // path, query, header
	Required    bool
	Type        string
	Description string
	// IDFieldRef is the value of x-k8s-id-field extension, indicating which body field
	// this path parameter should be merged with (e.g., "id" for orderId -> id mapping)
	IDFieldRef string
}

// Schema represents a data schema
type Schema struct {
	Name        string
	Type        string
	Format      string
	Description string
	Required    []string
	Properties  map[string]*Schema
	Items       *Schema // for arrays
	Ref         string
	Enum        []interface{}
	Default     interface{}
	Nullable    bool
	MinLength   *int64
	MaxLength   *int64
	Minimum     *float64
	Maximum     *float64
	Pattern     string
	MinItems    *int64
	MaxItems    *int64
}

// QueryEndpoint represents a query/search endpoint (GET-only with query params)
type QueryEndpoint struct {
	Name              string // e.g., "PetFindByTags"
	OperationID       string // e.g., "findPetsByTags"
	Path              string // e.g., "/pet/findByTags"
	BasePath          string // e.g., "/pet"
	Operation         string // e.g., "findByTags"
	Summary           string
	Description       string
	PathParams        []Parameter // Path parameters become spec fields
	QueryParams       []Parameter // Query parameters become spec fields
	ResponseSchema    *Schema     // Response schema for status
	ResponseSchemaRef string      // Reference name if response uses $ref (e.g., "Pet")
	ResponseIsArray   bool        // True if response is an array
}

// ActionEndpoint represents an action endpoint (POST/PUT on /{resource}/{id}/{action})
type ActionEndpoint struct {
	Name           string // e.g., "PetUploadImage"
	OperationID    string // e.g., "uploadFile"
	Path           string // e.g., "/pet/{petId}/uploadImage"
	ParentResource string // e.g., "Pet"
	ParentIDParam  string // e.g., "petId"
	ParentIDType   string // e.g., "integer" - OpenAPI type of parent ID param
	ActionName     string // e.g., "uploadImage"
	HTTPMethod     string // POST or PUT
	Summary        string
	Description    string
	PathParams     []Parameter // Path parameters (excluding parent ID)
	QueryParams    []Parameter // Query parameters
	RequestSchema  *Schema     // Request body schema
	ResponseSchema *Schema     // Response schema
	// Binary upload fields
	HasBinaryBody     bool   // True if request body is binary (application/octet-stream or multipart/form-data with binary)
	BinaryContentType string // Content type for binary data (e.g., "application/octet-stream", "multipart/form-data")
}

// ParsedSpec contains the parsed OpenAPI specification
type ParsedSpec struct {
	Title           string
	Version         string
	Description     string
	BaseURL         string
	Resources       []*Resource
	QueryEndpoints  []*QueryEndpoint
	ActionEndpoints []*ActionEndpoint
	Schemas         map[string]*Schema
}

// PathFilter interface for filtering paths, tags, and operationIds
type PathFilter interface {
	// ShouldIncludePath returns true if the path should be included based on path patterns
	ShouldIncludePath(path string) bool
	// ShouldIncludeTags returns true if the tags pass the tag filter
	ShouldIncludeTags(tags []string) bool
	// ShouldIncludeOperation returns true if the operationId passes the operation filter
	ShouldIncludeOperation(operationID string) bool
	// ShouldInclude returns true if both path and tags pass all filters (no operationId check)
	ShouldInclude(path string, tags []string) bool
	// ShouldIncludeWithOperations returns true if path, tags, and operationIds pass all filters
	ShouldIncludeWithOperations(path string, tags []string, operationIDs []string) bool
	// HasFilters returns true if any filters are configured
	HasFilters() bool
	// HasOperationFilters returns true if operationId filters are configured
	HasOperationFilters() bool
}

// Parser parses OpenAPI specifications
type Parser struct {
	// RootKind is the Kind name to use for the root "/" endpoint
	RootKind string
	// Filter is an optional filter for paths and tags
	Filter PathFilter
}

// NewParser creates a new OpenAPI parser
func NewParser() *Parser {
	return &Parser{}
}

// NewParserWithRootKind creates a new OpenAPI parser with a specified root kind
func NewParserWithRootKind(rootKind string) *Parser {
	return &Parser{RootKind: rootKind}
}

// NewParserWithFilter creates a new OpenAPI parser with a filter
func NewParserWithFilter(rootKind string, filter PathFilter) *Parser {
	return &Parser{RootKind: rootKind, Filter: filter}
}

// Parse parses an OpenAPI specification file or URL
// Supports both Swagger 2.0 and OpenAPI 3.0/3.1 specifications
func (p *Parser) Parse(specPath string) (*ParsedSpec, error) {
	// Read the raw spec first to detect version
	data, err := readSpec(specPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec: %w", err)
	}

	version := detectSpecVersion(data)

	var doc *openapi3.T
	isSwagger2 := false

	if version == "2.0" {
		// Parse as Swagger 2.0 and convert to OpenAPI 3.0
		fmt.Println("Detected Swagger 2.0 specification, converting to OpenAPI 3.0...")
		doc, err = parseSwagger2(data)
		if err != nil {
			return nil, err
		}
		isSwagger2 = true
	} else {
		// Parse as OpenAPI 3.x
		loader := openapi3.NewLoader()
		loader.IsExternalRefsAllowed = true

		if isURL(specPath) {
			// Load from URL
			specURL, parseErr := url.Parse(specPath)
			if parseErr != nil {
				return nil, fmt.Errorf("failed to parse spec URL: %w", parseErr)
			}
			doc, err = loader.LoadFromURI(specURL)
			if err != nil {
				return nil, fmt.Errorf("failed to load OpenAPI spec from URL: %w", err)
			}
		} else {
			// Load from file
			doc, err = loader.LoadFromFile(specPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load OpenAPI spec from file: %w", err)
			}
		}
	}

	// Validate the spec - use lenient validation for converted Swagger 2.0 specs
	// since they may have incomplete response definitions
	if isSwagger2 {
		// Skip strict validation for Swagger 2.0 specs - just do minimal checks
		if doc.Paths == nil {
			return nil, fmt.Errorf("invalid OpenAPI spec: no paths defined")
		}
	} else {
		// Disable example validation - example values don't affect code generation
		// and many real-world specs have type mismatches in examples (e.g. numeric zip codes
		// declared as string type)
		ctx := openapi3.WithValidationOptions(context.Background(),
			openapi3.DisableExamplesValidation(),
		)
		if err := doc.Validate(ctx); err != nil {
			return nil, fmt.Errorf("invalid OpenAPI spec: %w", err)
		}
	}

	spec := &ParsedSpec{
		Title:           doc.Info.Title,
		Version:         doc.Info.Version,
		Schemas:         make(map[string]*Schema),
		Resources:       make([]*Resource, 0),
		QueryEndpoints:  make([]*QueryEndpoint, 0),
		ActionEndpoints: make([]*ActionEndpoint, 0),
	}

	if doc.Info.Description != "" {
		spec.Description = doc.Info.Description
	}

	// Extract base URL from servers
	if len(doc.Servers) > 0 {
		spec.BaseURL = doc.Servers[0].URL
	}

	// Parse component schemas
	if doc.Components != nil && doc.Components.Schemas != nil {
		for name, schemaRef := range doc.Components.Schemas {
			spec.Schemas[name] = p.convertSchema(name, schemaRef.Value)
		}
	}

	// Parse paths and extract resources, query endpoints, and action endpoints
	resources, queryEndpoints, actionEndpoints := p.extractResourcesQueriesAndActions(doc)
	spec.Resources = resources
	spec.QueryEndpoints = queryEndpoints
	spec.ActionEndpoints = actionEndpoints

	return spec, nil
}

// getPathTagsAndOperationIDs extracts all unique tags and operationIds from operations on a path
func (p *Parser) getPathTagsAndOperationIDs(pathItem *openapi3.PathItem) ([]string, []string) {
	tagSet := make(map[string]bool)
	operationIDs := make([]string, 0)

	ops := []*openapi3.Operation{
		pathItem.Get,
		pathItem.Post,
		pathItem.Put,
		pathItem.Delete,
		pathItem.Patch,
		pathItem.Head,
		pathItem.Options,
	}

	for _, op := range ops {
		if op != nil {
			for _, tag := range op.Tags {
				tagSet[tag] = true
			}
			if op.OperationID != "" {
				operationIDs = append(operationIDs, op.OperationID)
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags, operationIDs
}

// getPathTags extracts all unique tags from operations on a path (for backwards compatibility)
func (p *Parser) getPathTags(pathItem *openapi3.PathItem) []string {
	tags, _ := p.getPathTagsAndOperationIDs(pathItem)
	return tags
}

// MethodFilterResult represents the filtering status of methods on a path
type MethodFilterResult struct {
	PassedMethods   []string // Methods that passed all filters
	FilteredMethods []string // Methods that were filtered out (with operationId)
	AllFiltered     bool     // True if the entire path is filtered (path/tag filter)
	PathFiltered    bool     // True if filtered by path pattern
	TagFiltered     bool     // True if filtered by tag
}

// getMethodFilterResults returns detailed filtering info for each method on a path
func (p *Parser) getMethodFilterResults(path string, pathItem *openapi3.PathItem) MethodFilterResult {
	result := MethodFilterResult{
		PassedMethods:   make([]string, 0),
		FilteredMethods: make([]string, 0),
	}

	if p.Filter == nil {
		// No filter, all methods pass
		result.PassedMethods = append(result.PassedMethods, p.getMethodsForPathAsList(pathItem)...)
		return result
	}

	// First check path filter
	if !p.Filter.ShouldIncludePath(path) {
		result.AllFiltered = true
		result.PathFiltered = true
		return result
	}

	// Then check tag filter
	tags := p.getPathTags(pathItem)
	if !p.Filter.ShouldIncludeTags(tags) {
		result.AllFiltered = true
		result.TagFiltered = true
		return result
	}

	// If no operation filters, all methods pass
	if !p.Filter.HasOperationFilters() {
		result.PassedMethods = append(result.PassedMethods, p.getMethodsForPathAsList(pathItem)...)
		return result
	}

	// Check each method individually
	methodOps := []struct {
		method string
		op     *openapi3.Operation
	}{
		{"GET", pathItem.Get},
		{"POST", pathItem.Post},
		{"PUT", pathItem.Put},
		{"DELETE", pathItem.Delete},
		{"PATCH", pathItem.Patch},
	}

	for _, mo := range methodOps {
		if mo.op == nil {
			continue
		}
		opID := mo.op.OperationID
		if p.Filter.ShouldIncludeOperation(opID) {
			result.PassedMethods = append(result.PassedMethods, mo.method)
		} else {
			// Include operationId in the filtered method display
			if opID != "" {
				result.FilteredMethods = append(result.FilteredMethods, fmt.Sprintf("%s(%s)", mo.method, opID))
			} else {
				result.FilteredMethods = append(result.FilteredMethods, mo.method)
			}
		}
	}

	// If all methods were filtered by operation filter, mark as all filtered
	result.AllFiltered = len(result.PassedMethods) == 0

	return result
}

// getMethodsForPathAsList returns methods as a string slice
func (p *Parser) getMethodsForPathAsList(pathItem *openapi3.PathItem) []string {
	methods := make([]string, 0)
	if pathItem.Get != nil {
		methods = append(methods, "GET")
	}
	if pathItem.Post != nil {
		methods = append(methods, "POST")
	}
	if pathItem.Put != nil {
		methods = append(methods, "PUT")
	}
	if pathItem.Delete != nil {
		methods = append(methods, "DELETE")
	}
	if pathItem.Patch != nil {
		methods = append(methods, "PATCH")
	}
	return methods
}

// shouldIncludePath checks if a path should be included based on the parser's filter
func (p *Parser) shouldIncludePath(path string, pathItem *openapi3.PathItem) bool {
	if p.Filter == nil {
		return true
	}
	tags, operationIDs := p.getPathTagsAndOperationIDs(pathItem)
	return p.Filter.ShouldIncludeWithOperations(path, tags, operationIDs)
}

func (p *Parser) extractResourcesQueriesAndActions(doc *openapi3.T) ([]*Resource, []*QueryEndpoint, []*ActionEndpoint) {
	resourceMap := make(map[string]*Resource)
	queryEndpoints := make([]*QueryEndpoint, 0)
	actionEndpoints := make([]*ActionEndpoint, 0)

	// Build map of base paths to their corresponding resource ID paths
	// e.g., /pet -> /pet/{petId}
	resourceIDPaths := p.buildResourceIDPaths(doc)

	// Track which paths are part of a combined resource (base path for POST)
	combinedBasePaths := make(map[string]bool)

	// Get sorted paths for deterministic output
	paths := make([]string, 0, len(doc.Paths.Map()))
	for path := range doc.Paths.Map() {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	// Count filtered paths if filter is active
	filteredCount := 0
	if p.Filter != nil && p.Filter.HasFilters() {
		for _, path := range paths {
			pathItem := doc.Paths.Map()[path]
			if !p.shouldIncludePath(path, pathItem) {
				filteredCount++
			}
		}
		if filteredCount > 0 {
			fmt.Printf("Filtering: %d of %d paths excluded by filter\n", filteredCount, len(paths))
		}
	}

	// Log endpoint classification header
	fmt.Println("\n┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│                                        Endpoint Classification                                                    │")
	fmt.Println("├────────────────────────────────────┬──────────────┬────────────────────┬─────────────────────┬─────────────────────┤")
	fmt.Println("│ Endpoint                           │ Method       │ Classification     │ Kind                │ Parent ID Param     │")
	fmt.Println("├────────────────────────────────────┼──────────────┼────────────────────┼─────────────────────┼─────────────────────┤")

	for _, path := range paths {
		pathItem := doc.Paths.Map()[path]
		methods := p.getMethodsForPath(pathItem)

		// Get detailed filter results for this path
		filterResult := p.getMethodFilterResults(path, pathItem)

		// Apply path/tag filter
		if !p.shouldIncludePath(path, pathItem) {
			// Show detailed filtering info
			classification := "Filtered"
			if filterResult.PathFiltered {
				classification = "Filtered (path)"
			} else if filterResult.TagFiltered {
				classification = "Filtered (tag)"
			}

			// If some methods were filtered by operationId, show which ones
			methodDisplay := methods
			if len(filterResult.FilteredMethods) > 0 && !filterResult.PathFiltered && !filterResult.TagFiltered {
				// Show passed methods normally, filtered methods with strikethrough indication
				passed := strings.Join(filterResult.PassedMethods, ",")
				filtered := strings.Join(filterResult.FilteredMethods, ",")
				if passed != "" && filtered != "" {
					methodDisplay = fmt.Sprintf("%s ~%s~", passed, filtered)
				} else if filtered != "" {
					methodDisplay = fmt.Sprintf("~%s~", filtered)
				}
				classification = "Filtered (op)"
			}

			printWrappedTableRow(path, methodDisplay, classification, "-", "-")
			continue
		}

		// Check if this path is a base path with POST that has a corresponding resource ID path
		// e.g., /pet with POST + /pet/{petId} with GET/PUT/DELETE = combined resource
		if p.hasCorrespondingResourceIDPath(path, doc, resourceIDPaths) && pathItem.Post != nil {
			// This is a base path that should be combined with its ID path
			// Mark it as combined and process as a resource
			combinedBasePaths[path] = true
			// Continue processing as resource below (don't skip)
		} else {
			// Check if this is an action endpoint FIRST (before query endpoints)
			// This ensures /user/login (GET with strong action keyword) is treated as action, not query
			if actionEndpoint := p.extractActionEndpoint(path, pathItem, doc); actionEndpoint != nil {
				actionEndpoints = append(actionEndpoints, actionEndpoint)
				parentIDDisplay := actionEndpoint.ParentIDParam
				if parentIDDisplay == "" {
					parentIDDisplay = "-"
				}
				printWrappedTableRow(path, actionEndpoint.HTTPMethod, "ActionEndpoint", actionEndpoint.Name, parentIDDisplay)
				continue
			}

			// Check if this is a query endpoint
			if queryEndpoint := p.extractQueryEndpoint(path, pathItem, doc); queryEndpoint != nil {
				queryEndpoints = append(queryEndpoints, queryEndpoint)
				printWrappedTableRow(path, "GET", "QueryEndpoint", queryEndpoint.Name, "-")
				continue
			}
		}

		resourceName := p.extractResourceName(path)
		if resourceName == "" {
			printWrappedTableRow(path, methods, "Skipped", "-", "-")
			continue
		}

		resource, exists := resourceMap[resourceName]
		if !exists {
			resource = &Resource{
				Name:       resourceName,
				PluralName: p.pluralize(resourceName),
				Path:       p.getBasePath(path),
				Operations: make([]Operation, 0),
			}
			resourceMap[resourceName] = resource
		}

		// Check if this is a combined resource (base path that was combined with ID path)
		classification := "Resource"
		if combinedBasePaths[path] {
			classification = "Resource (POST)"
		} else if p.isResourceIDPath(path) {
			// Check if this ID path has a corresponding base path with POST
			basePath := p.getBasePathForIDPath(path)
			if combinedBasePaths[basePath] {
				classification = "Resource (ID)"
			}
		}

		// Show method display with filtered methods indicated
		methodDisplay := methods
		if len(filterResult.FilteredMethods) > 0 {
			// Some methods were filtered by operationId - show which
			passed := strings.Join(filterResult.PassedMethods, ",")
			filtered := strings.Join(filterResult.FilteredMethods, ",")
			methodDisplay = fmt.Sprintf("%s ~%s~", passed, filtered)
		}

		printWrappedTableRow(path, methodDisplay, classification, resourceName, "-")

		// Extract operations
		ops := p.extractOperations(path, pathItem)
		resource.Operations = append(resource.Operations, ops...)

		// Try to extract schema from POST/PUT request body
		if resource.Schema == nil {
			resource.Schema = p.extractResourceSchema(pathItem, doc)
		}
	}

	fmt.Println("└────────────────────────────────────┴──────────────┴────────────────────┴─────────────────────┴─────────────────────┘")
	fmt.Println()

	// Convert map to slice
	resources := make([]*Resource, 0, len(resourceMap))
	for _, r := range resourceMap {
		resources = append(resources, r)
	}

	// Sort for deterministic output
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Name < resources[j].Name
	})

	return resources, queryEndpoints, actionEndpoints
}

// extractResourcesAndQueries is kept for backwards compatibility
func (p *Parser) extractResourcesAndQueries(doc *openapi3.T) ([]*Resource, []*QueryEndpoint) {
	resources, queryEndpoints, _ := p.extractResourcesQueriesAndActions(doc)
	return resources, queryEndpoints
}

// isActionEndpoint checks if a path is an action endpoint
// Action endpoints are POST or PUT only (no GET) with patterns:
//   - /{action} (e.g., /login, /store)
//   - /{resource}/{action} (e.g., /api/echo, /user/register)
//   - /{resource}/{resourceId}/{action} (e.g., /pet/{petId}/uploadImage)
func (p *Parser) isActionEndpoint(path string, pathItem *openapi3.PathItem) bool {
	hasPost := pathItem.Post != nil
	hasPut := pathItem.Put != nil
	hasGet := pathItem.Get != nil
	hasDelete := pathItem.Delete != nil
	hasPatch := pathItem.Patch != nil

	// Action endpoints must be POST/PUT only - no GET, DELETE, or PATCH
	if !hasPost && !hasPut {
		return false
	}
	if hasGet || hasDelete || hasPatch {
		return false
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 1 {
		return false
	}

	// Last segment must not be a parameter
	lastPart := parts[len(parts)-1]
	if strings.HasPrefix(lastPart, "{") {
		return false
	}

	// Pattern 1: /{resource}/{resourceId}/{action} (3+ segments with middle parameter)
	if len(parts) >= 3 {
		hasParamInMiddle := false
		for i := 1; i < len(parts)-1; i++ {
			if strings.HasPrefix(parts[i], "{") && strings.HasSuffix(parts[i], "}") {
				hasParamInMiddle = true
				break
			}
		}
		if hasParamInMiddle {
			return p.isActionSegment(lastPart)
		}
	}

	// Pattern 2: /{resource}/{action} (2 segments) or /{action} (1 segment)
	// POST/PUT only = always an action (e.g., /api/echo, /store, /login)
	if len(parts) >= 1 {
		return true
	}

	return false
}

// isStrongActionKeyword checks if a segment is a strong action keyword
// These keywords clearly indicate an action even for GET-only endpoints
func (p *Parser) isStrongActionKeyword(segment string) bool {
	strongActions := []string{
		"login", "logout", "signin", "signout", "register",
		"verify", "activate", "deactivate", "reset", "refresh",
		"revoke", "authorize", "authenticate", "token",
	}
	lower := strings.ToLower(segment)
	for _, action := range strongActions {
		if strings.Contains(lower, action) {
			return true
		}
	}
	return false
}

// extractActionEndpoint extracts an action endpoint definition
func (p *Parser) extractActionEndpoint(path string, pathItem *openapi3.PathItem, doc *openapi3.T) *ActionEndpoint {
	if !p.isActionEndpoint(path, pathItem) {
		return nil
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Find the parent resource and its ID parameter
	var parentResource string
	var parentIDParam string
	var actionName string
	var name string

	// Pattern 1: Find structure /{resource}/{resourceId}/{action} (3+ segments with ID param)
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			// This is a parameter - the previous segment is the resource
			if i > 0 {
				parentResource = p.singularize(p.toPascalCase(parts[i-1]))
				parentIDParam = part[1 : len(part)-1] // Remove { and }
			}
		}
	}

	actionName = parts[len(parts)-1]

	// Pattern 2: /{resource}/{action} (2 segments, no ID param)
	if parentResource == "" && len(parts) == 2 {
		parentResource = p.singularize(p.toPascalCase(parts[0]))
		// parentIDParam stays empty - this is an action without parent ID
	}

	// Pattern 3: /{action} (1 segment, no parent resource)
	if parentResource == "" && len(parts) == 1 {
		// For single-segment actions, there's no parent resource
		// The action name becomes the Kind (e.g., /store -> Store)
		parentResource = ""
		// parentIDParam stays empty
	}

	// Pattern 4: / (root endpoint)
	// Use RootKind if configured, otherwise skip this endpoint
	if actionName == "" {
		if p.RootKind != "" {
			name = p.RootKind + "Action"
			actionName = strings.ToLower(p.RootKind)
		} else {
			// No RootKind configured, skip root endpoint
			return nil
		}
	} else {
		// Build the action endpoint name
		if parentResource != "" {
			name = parentResource + p.toPascalCase(actionName) + "Action"
		} else {
			// Single-segment action: use action name as the Kind
			name = p.toPascalCase(actionName) + "Action"
		}
	}

	// Get the operation (prefer POST over PUT, then GET for strong action keywords)
	var op *openapi3.Operation
	var httpMethod string
	if pathItem.Post != nil {
		op = pathItem.Post
		httpMethod = "POST"
	} else if pathItem.Put != nil {
		op = pathItem.Put
		httpMethod = "PUT"
	} else if pathItem.Get != nil && p.isStrongActionKeyword(actionName) {
		op = pathItem.Get
		httpMethod = "GET"
	}

	if op == nil {
		return nil
	}

	actionEndpoint := &ActionEndpoint{
		Name:           name,
		OperationID:    op.OperationID,
		Path:           path,
		ParentResource: parentResource,
		ParentIDParam:  parentIDParam,
		ActionName:     actionName,
		HTTPMethod:     httpMethod,
		Summary:        op.Summary,
		Description:    op.Description,
		PathParams:     make([]Parameter, 0),
		QueryParams:    make([]Parameter, 0),
	}

	// Extract parameters
	for _, paramRef := range op.Parameters {
		if paramRef.Value == nil {
			continue
		}
		param := Parameter{
			Name:        paramRef.Value.Name,
			In:          paramRef.Value.In,
			Required:    paramRef.Value.Required,
			Description: paramRef.Value.Description,
		}
		if paramRef.Value.Schema != nil && paramRef.Value.Schema.Value != nil {
			if len(paramRef.Value.Schema.Value.Type.Slice()) > 0 {
				param.Type = paramRef.Value.Schema.Value.Type.Slice()[0]
			}
		}

		// Extract x-k8s-id-field extension if present
		if paramRef.Value.Extensions != nil {
			if idFieldRef, ok := paramRef.Value.Extensions["x-k8s-id-field"]; ok {
				if strVal, ok := idFieldRef.(string); ok {
					param.IDFieldRef = strVal
				}
			}
		}

		// Capture the parent ID param's type, then skip it (handled separately)
		if param.Name == parentIDParam {
			actionEndpoint.ParentIDType = param.Type
			continue
		}

		switch param.In {
		case "path":
			actionEndpoint.PathParams = append(actionEndpoint.PathParams, param)
		case "query":
			actionEndpoint.QueryParams = append(actionEndpoint.QueryParams, param)
		}
	}

	// Extract request body schema
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		if content, ok := op.RequestBody.Value.Content["application/json"]; ok {
			if content.Schema != nil && content.Schema.Value != nil {
				actionEndpoint.RequestSchema = p.convertSchema("RequestBody", content.Schema.Value)
			}
		}
		// Check for multipart/form-data (common for file uploads)
		if content, ok := op.RequestBody.Value.Content["multipart/form-data"]; ok {
			if content.Schema != nil && content.Schema.Value != nil {
				schema := content.Schema.Value
				actionEndpoint.RequestSchema = p.convertSchema("RequestBody", schema)
				// Check if any property has binary format
				if schema.Properties != nil {
					for _, prop := range schema.Properties {
						if prop.Value != nil && prop.Value.Format == "binary" {
							actionEndpoint.HasBinaryBody = true
							actionEndpoint.BinaryContentType = "multipart/form-data"
							break
						}
					}
				}
			}
		}
		// Check for application/octet-stream (raw binary)
		if content, ok := op.RequestBody.Value.Content["application/octet-stream"]; ok {
			if content.Schema != nil && content.Schema.Value != nil {
				schema := content.Schema.Value
				actionEndpoint.RequestSchema = p.convertSchema("RequestBody", schema)
				// application/octet-stream with binary format is a binary upload
				schemaType := ""
				if len(schema.Type.Slice()) > 0 {
					schemaType = schema.Type.Slice()[0]
				}
				if schema.Format == "binary" || schemaType == "string" {
					actionEndpoint.HasBinaryBody = true
					actionEndpoint.BinaryContentType = "application/octet-stream"
				}
			}
		}
	}

	// Extract response schema
	for _, code := range []string{"200", "201"} {
		if resp := op.Responses.Status(p.parseStatusCode(code)); resp != nil && resp.Value != nil {
			if content, ok := resp.Value.Content["application/json"]; ok {
				if content.Schema != nil && content.Schema.Value != nil {
					actionEndpoint.ResponseSchema = p.convertSchema("Response", content.Schema.Value)
					break
				}
			}
		}
	}

	return actionEndpoint
}

// isQueryEndpoint checks if a path is a query/search endpoint
// Query endpoints are any endpoint with GET method only (no POST, PUT, PATCH, DELETE)
func (p *Parser) isQueryEndpoint(path string, pathItem *openapi3.PathItem) bool {
	// Must have GET operation and no other methods
	return pathItem.Get != nil &&
		pathItem.Post == nil &&
		pathItem.Put == nil &&
		pathItem.Patch == nil &&
		pathItem.Delete == nil
}

// extractQueryEndpoint extracts a query endpoint definition
func (p *Parser) extractQueryEndpoint(path string, pathItem *openapi3.PathItem, doc *openapi3.T) *QueryEndpoint {
	if !p.isQueryEndpoint(path, pathItem) {
		return nil
	}

	op := pathItem.Get
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Handle root "/" endpoint
	if path == "/" || (len(parts) == 1 && parts[0] == "") {
		if p.RootKind == "" {
			// No RootKind configured, skip root endpoint
			return nil
		}
		parts = []string{strings.ToLower(p.RootKind)}
	}

	// Build the query endpoint name
	// /pet/findByTags -> PetFindByTagsQuery
	// /user/login -> UserLoginQuery
	nameParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if !strings.HasPrefix(part, "{") {
			nameParts = append(nameParts, p.toPascalCase(part))
		}
	}
	name := strings.Join(nameParts, "") + "Query"

	// Extract base resource and operation
	basePath := "/" + parts[0]
	operation := parts[len(parts)-1]

	queryEndpoint := &QueryEndpoint{
		Name:        name,
		OperationID: op.OperationID,
		Path:        path,
		BasePath:    basePath,
		Operation:   operation,
		Summary:     op.Summary,
		Description: op.Description,
		PathParams:  make([]Parameter, 0),
		QueryParams: make([]Parameter, 0),
	}

	// Extract path and query parameters
	for _, paramRef := range op.Parameters {
		if paramRef.Value == nil {
			continue
		}
		if paramRef.Value.In == "path" {
			param := Parameter{
				Name:        paramRef.Value.Name,
				In:          paramRef.Value.In,
				Required:    paramRef.Value.Required,
				Description: paramRef.Value.Description,
			}
			if paramRef.Value.Schema != nil && paramRef.Value.Schema.Value != nil {
				if len(paramRef.Value.Schema.Value.Type.Slice()) > 0 {
					param.Type = paramRef.Value.Schema.Value.Type.Slice()[0]
				}
			}
			// Extract x-k8s-id-field extension if present
			if paramRef.Value.Extensions != nil {
				if idFieldRef, ok := paramRef.Value.Extensions["x-k8s-id-field"]; ok {
					if strVal, ok := idFieldRef.(string); ok {
						param.IDFieldRef = strVal
					}
				}
			}
			queryEndpoint.PathParams = append(queryEndpoint.PathParams, param)
		} else if paramRef.Value.In == "query" {
			param := Parameter{
				Name:        paramRef.Value.Name,
				In:          paramRef.Value.In,
				Required:    paramRef.Value.Required,
				Description: paramRef.Value.Description,
			}
			if paramRef.Value.Schema != nil && paramRef.Value.Schema.Value != nil {
				schemaVal := paramRef.Value.Schema.Value
				if len(schemaVal.Type.Slice()) > 0 {
					param.Type = schemaVal.Type.Slice()[0]
				}
				// Handle array query params (e.g., tags[])
				if param.Type == "array" && schemaVal.Items != nil && schemaVal.Items.Value != nil {
					if len(schemaVal.Items.Value.Type.Slice()) > 0 {
						param.Type = "array:" + schemaVal.Items.Value.Type.Slice()[0]
					}
				}
			}
			queryEndpoint.QueryParams = append(queryEndpoint.QueryParams, param)
		}
	}

	// Extract response schema and capture reference name
	for _, code := range []string{"200", "201"} {
		if resp := op.Responses.Status(p.parseStatusCode(code)); resp != nil && resp.Value != nil {
			if content, ok := resp.Value.Content["application/json"]; ok {
				if content.Schema != nil {
					schemaRef := content.Schema

					// Check if it's an array with items referencing a schema
					if schemaRef.Value != nil && schemaRef.Value.Type.Slice() != nil {
						for _, t := range schemaRef.Value.Type.Slice() {
							if t == "array" {
								queryEndpoint.ResponseIsArray = true
								// Check if array items have a $ref
								if schemaRef.Value.Items != nil && schemaRef.Value.Items.Ref != "" {
									queryEndpoint.ResponseSchemaRef = p.extractRefName(schemaRef.Value.Items.Ref)
								}
								break
							}
						}
					}

					// Check for direct $ref (non-array response)
					if !queryEndpoint.ResponseIsArray && schemaRef.Ref != "" {
						queryEndpoint.ResponseSchemaRef = p.extractRefName(schemaRef.Ref)
					}

					if schemaRef.Value != nil {
						queryEndpoint.ResponseSchema = p.convertSchema("Response", schemaRef.Value)
					}
					break
				}
			}
		}
	}

	return queryEndpoint
}

// extractResources is kept for backwards compatibility
func (p *Parser) extractResources(doc *openapi3.T) []*Resource {
	resources, _ := p.extractResourcesAndQueries(doc)
	return resources
}

func (p *Parser) extractResourceName(path string) string {
	// Remove leading slash and split by /
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}

	// Handle root "/" endpoint
	if path == "/" || (len(parts) == 1 && parts[0] == "") {
		if p.RootKind != "" {
			return p.RootKind
		}
		return ""
	}

	// Strategy: Find the resource name based on path patterns.
	//
	// Pattern 1: Resource with ID parameter
	//   /pet/{petId} -> "Pet" (segment before matching {petId})
	//   /store/order/{orderId} -> "Order" (segment before matching {orderId})
	//
	// Pattern 2: Nested sub-resource (has parent ID before child resource)
	//   /users/{userId}/posts -> "User" (posts is a sub-resource of users)
	//   The {userId} parameter before "posts" indicates posts belong to users
	//
	// Pattern 3: Namespaced resource (no parameters between segments)
	//   /store/order -> "Order" (order is the resource under store namespace)
	//   /api/v1/users -> "User" (users is the resource under api/v1)
	//
	// Pattern 4: Action endpoints
	//   /pet/{petId}/uploadImage -> "Pet" (uploadImage is an action, not resource)

	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]

		// Skip parameters
		if strings.HasPrefix(part, "{") {
			continue
		}

		// Check if this segment is followed by a matching ID parameter
		// e.g., "order" followed by "{orderId}" or "variables" followed by "{variableName}"
		if i < len(parts)-1 {
			nextPart := parts[i+1]
			if strings.HasPrefix(nextPart, "{") && strings.HasSuffix(nextPart, "}") {
				paramName := strings.ToLower(nextPart[1 : len(nextPart)-1])
				segmentName := strings.ToLower(part)
				singularSegment := strings.ToLower(p.singularize(part))
				// Check if param name contains the segment name or its singular form
				// (e.g., "orderid" contains "order", "variablename" contains "variable")
				// or is a generic "id" parameter
				if strings.Contains(paramName, segmentName) || strings.Contains(paramName, singularSegment) || paramName == "id" {
					return p.singularize(p.toPascalCase(part))
				}
			}
		}

		// If this is the last segment (not a parameter)
		if i == len(parts)-1 {
			// Skip if it looks like an action (e.g., uploadImage)
			if p.isActionSegment(part) {
				continue
			}

			// Check if there's a parameter BEFORE this segment
			// e.g., /users/{userId}/posts - {userId} is before "posts"
			// This indicates a nested sub-resource pattern; fall through to find parent
			if i > 0 && strings.HasPrefix(parts[i-1], "{") {
				continue
			}

			return p.singularize(p.toPascalCase(part))
		}
	}

	// Fallback to first non-parameter segment
	for _, part := range parts {
		if !strings.HasPrefix(part, "{") {
			return p.singularize(p.toPascalCase(part))
		}
	}

	return ""
}

// isActionSegment checks if a path segment looks like an action rather than a resource
func (p *Parser) isActionSegment(s string) bool {
	lower := strings.ToLower(s)
	actionKeywords := []string{
		"upload", "download", "find", "search", "get", "create", "delete",
		"update", "list", "login", "logout", "check", "validate", "verify",
		"send", "receive", "export", "import", "sync", "refresh", "reset",
	}
	for _, kw := range actionKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func (p *Parser) getBasePath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return "/"
	}

	// Build path up to and including the resource segment.
	// The resource is determined by extractResourceName logic:
	// - For /store/order/{orderId}, resource is "order", base path is /store/order
	// - For /users/{userId}/posts, resource is "user", base path is /users
	// - For /api/v1/users, resource is "user", base path is /api/v1/users
	//
	// Strategy: Include all segments until we hit a parameter, then stop.
	// But if the last included segment is followed by a matching ID param, that's the resource.

	var baseParts []string
	for i, part := range parts {
		// Stop at parameters
		if strings.HasPrefix(part, "{") {
			break
		}
		// Stop at action segments
		if p.isActionSegment(part) {
			break
		}
		baseParts = append(baseParts, part)

		// Check if this segment is followed by a matching ID parameter
		// If so, this is the resource and we should stop here
		if i < len(parts)-1 {
			nextPart := parts[i+1]
			if strings.HasPrefix(nextPart, "{") && strings.HasSuffix(nextPart, "}") {
				paramName := strings.ToLower(nextPart[1 : len(nextPart)-1])
				segmentName := strings.ToLower(part)
				if strings.Contains(paramName, segmentName) || paramName == "id" {
					break
				}
			}
		}
	}

	if len(baseParts) == 0 {
		return "/"
	}

	return "/" + strings.Join(baseParts, "/")
}

func (p *Parser) extractOperations(path string, pathItem *openapi3.PathItem) []Operation {
	ops := make([]Operation, 0)

	methods := map[string]*openapi3.Operation{
		"GET":    pathItem.Get,
		"POST":   pathItem.Post,
		"PUT":    pathItem.Put,
		"PATCH":  pathItem.Patch,
		"DELETE": pathItem.Delete,
	}

	for method, op := range methods {
		if op == nil {
			continue
		}

		operation := Operation{
			Method:      method,
			Path:        path,
			OperationID: op.OperationID,
			Summary:     op.Summary,
			PathParams:  make([]Parameter, 0),
			QueryParams: make([]Parameter, 0),
		}

		// Extract parameters
		for _, paramRef := range op.Parameters {
			if paramRef.Value == nil {
				continue
			}
			param := Parameter{
				Name:        paramRef.Value.Name,
				In:          paramRef.Value.In,
				Required:    paramRef.Value.Required,
				Description: paramRef.Value.Description,
			}
			if paramRef.Value.Schema != nil && paramRef.Value.Schema.Value != nil {
				param.Type = paramRef.Value.Schema.Value.Type.Slice()[0]
			}

			// Extract x-k8s-id-field extension if present
			if paramRef.Value.Extensions != nil {
				if idFieldRef, ok := paramRef.Value.Extensions["x-k8s-id-field"]; ok {
					if strVal, ok := idFieldRef.(string); ok {
						param.IDFieldRef = strVal
					}
				}
			}

			switch param.In {
			case "path":
				operation.PathParams = append(operation.PathParams, param)
			case "query":
				operation.QueryParams = append(operation.QueryParams, param)
			}
		}

		// Extract request body schema
		if op.RequestBody != nil && op.RequestBody.Value != nil {
			if content, ok := op.RequestBody.Value.Content["application/json"]; ok {
				if content.Schema != nil && content.Schema.Value != nil {
					operation.RequestBody = p.convertSchema("RequestBody", content.Schema.Value)
				}
			}
		}

		// Extract response body schema (from 200 or 201 response)
		for _, code := range []string{"200", "201"} {
			if resp := op.Responses.Status(p.parseStatusCode(code)); resp != nil && resp.Value != nil {
				if content, ok := resp.Value.Content["application/json"]; ok {
					if content.Schema != nil && content.Schema.Value != nil {
						operation.ResponseBody = p.convertSchema("ResponseBody", content.Schema.Value)
						break
					}
				}
			}
		}

		ops = append(ops, operation)
	}

	return ops
}

func (p *Parser) parseStatusCode(code string) int {
	switch code {
	case "200":
		return 200
	case "201":
		return 201
	default:
		return 200
	}
}

func (p *Parser) extractResourceSchema(pathItem *openapi3.PathItem, doc *openapi3.T) *Schema {
	// Try POST first, then PUT
	for _, op := range []*openapi3.Operation{pathItem.Post, pathItem.Put} {
		if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
			continue
		}
		if content, ok := op.RequestBody.Value.Content["application/json"]; ok {
			if content.Schema != nil {
				if content.Schema.Ref != "" {
					// Resolve reference
					refName := p.extractRefName(content.Schema.Ref)
					if schema, ok := doc.Components.Schemas[refName]; ok {
						return p.convertSchema(refName, schema.Value)
					}
				}
				if content.Schema.Value != nil {
					return p.convertSchema("Resource", content.Schema.Value)
				}
			}
		}
	}
	return nil
}

func (p *Parser) extractRefName(ref string) string {
	// Extract name from "#/components/schemas/Name"
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	parts := strings.Split(u.Fragment, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func (p *Parser) convertSchema(name string, schema *openapi3.Schema) *Schema {
	if schema == nil {
		return nil
	}

	s := &Schema{
		Name:        name,
		Description: schema.Description,
		Required:    schema.Required,
		Properties:  make(map[string]*Schema),
		Nullable:    schema.Nullable,
		Pattern:     schema.Pattern,
	}

	// Handle type - it can be a slice in OpenAPI 3.1
	if len(schema.Type.Slice()) > 0 {
		s.Type = schema.Type.Slice()[0]
	}
	s.Format = schema.Format

	// Infer type from structure if not explicitly set
	if s.Type == "" {
		if len(schema.Properties) > 0 {
			s.Type = "object"
		} else if schema.Items != nil {
			s.Type = "array"
		}
	}

	// Handle validation
	if schema.MinLength != 0 {
		v := int64(schema.MinLength)
		s.MinLength = &v
	}
	if schema.MaxLength != nil {
		v := int64(*schema.MaxLength)
		s.MaxLength = &v
	}
	if schema.Min != nil {
		s.Minimum = schema.Min
	}
	if schema.Max != nil {
		s.Maximum = schema.Max
	}
	if schema.MinItems != 0 {
		v := int64(schema.MinItems)
		s.MinItems = &v
	}
	if schema.MaxItems != nil {
		v := int64(*schema.MaxItems)
		s.MaxItems = &v
	}

	// Handle enum
	s.Enum = schema.Enum
	s.Default = schema.Default

	// Handle properties for objects
	if schema.Properties != nil {
		for propName, propRef := range schema.Properties {
			if propRef.Value != nil {
				s.Properties[propName] = p.convertSchema(propName, propRef.Value)
			}
		}
	}

	// Handle array items
	if schema.Items != nil && schema.Items.Value != nil {
		s.Items = p.convertSchema("Items", schema.Items.Value)
	}

	return s
}

func (p *Parser) toPascalCase(s string) string {
	// Simple conversion - split by common separators and capitalize
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, "")
}

func (p *Parser) singularize(s string) string {
	// Simple singularization
	if strings.HasSuffix(s, "ies") {
		return s[:len(s)-3] + "y"
	}
	// Remove "es" for words ending in sibilants (s, x, z, ch, sh) + es
	// e.g., "classes" -> "class", "buses" -> "bus", "boxes" -> "box"
	// Note: "ses" is included for words like "buses" but may incorrectly
	// singularize rare words like "cases" -> "cas" (should be "case")
	if strings.HasSuffix(s, "sses") || strings.HasSuffix(s, "ses") ||
		strings.HasSuffix(s, "xes") || strings.HasSuffix(s, "zes") ||
		strings.HasSuffix(s, "ches") || strings.HasSuffix(s, "shes") {
		return s[:len(s)-2]
	}
	// For other words ending in "s" (like "profiles", "variables"), just remove "s"
	if strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss") {
		return s[:len(s)-1]
	}
	return s
}

func (p *Parser) pluralize(s string) string {
	// Simple pluralization
	if strings.HasSuffix(s, "y") {
		return s[:len(s)-1] + "ies"
	}
	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") || strings.HasSuffix(s, "ch") {
		return s + "es"
	}
	return s + "s"
}

// getMethodsForPath returns a comma-separated string of HTTP methods for a path
func (p *Parser) getMethodsForPath(pathItem *openapi3.PathItem) string {
	methods := make([]string, 0)
	if pathItem.Get != nil {
		methods = append(methods, "GET")
	}
	if pathItem.Post != nil {
		methods = append(methods, "POST")
	}
	if pathItem.Put != nil {
		methods = append(methods, "PUT")
	}
	if pathItem.Patch != nil {
		methods = append(methods, "PATCH")
	}
	if pathItem.Delete != nil {
		methods = append(methods, "DELETE")
	}
	return strings.Join(methods, ",")
}

// wrapText wraps text into lines of at most maxLen characters
// It tries to break at natural break points (/, -, _, ,) when possible
func wrapText(s string, maxLen int) []string {
	if len(s) <= maxLen {
		return []string{s}
	}

	var lines []string
	remaining := s

	for len(remaining) > maxLen {
		// Find a good break point within the maxLen limit
		breakPoint := maxLen

		// Look for natural break characters (/, -, _, ,) working backwards from maxLen
		for i := maxLen - 1; i > maxLen/2; i-- {
			if remaining[i] == '/' || remaining[i] == '-' || remaining[i] == '_' || remaining[i] == ',' {
				breakPoint = i + 1 // Include the break character in the first part
				break
			}
		}

		lines = append(lines, remaining[:breakPoint])
		remaining = remaining[breakPoint:]
	}

	if len(remaining) > 0 {
		lines = append(lines, remaining)
	}

	return lines
}

// printWrappedTableRow prints a table row with text wrapping for cells that exceed column width
// Column widths: Endpoint=34, Method=12, Classification=18, Kind=19, ParentID=19
func printWrappedTableRow(endpoint, method, classification, kind, parentID string) {
	// Wrap each cell
	endpointLines := wrapText(endpoint, 34)
	methodLines := wrapText(method, 12)
	classificationLines := wrapText(classification, 18)
	kindLines := wrapText(kind, 19)
	parentIDLines := wrapText(parentID, 19)

	// Find the maximum number of lines needed
	maxLines := len(endpointLines)
	if len(methodLines) > maxLines {
		maxLines = len(methodLines)
	}
	if len(classificationLines) > maxLines {
		maxLines = len(classificationLines)
	}
	if len(kindLines) > maxLines {
		maxLines = len(kindLines)
	}
	if len(parentIDLines) > maxLines {
		maxLines = len(parentIDLines)
	}

	// Print each line
	for i := 0; i < maxLines; i++ {
		e := ""
		if i < len(endpointLines) {
			e = endpointLines[i]
		}
		m := ""
		if i < len(methodLines) {
			m = methodLines[i]
		}
		c := ""
		if i < len(classificationLines) {
			c = classificationLines[i]
		}
		k := ""
		if i < len(kindLines) {
			k = kindLines[i]
		}
		p := ""
		if i < len(parentIDLines) {
			p = parentIDLines[i]
		}

		fmt.Printf("│ %-34s │ %-12s │ %-18s │ %-19s │ %-19s │\n", e, m, c, k, p)
	}
}

// isResourceIDPath checks if a path is a resource with an ID parameter
// e.g., /pet/{petId}, /store/order/{orderId}, /users/{userId}
// Returns true if the path ends with a resource segment followed by an ID parameter
func (p *Parser) isResourceIDPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		return false
	}

	// Last part must be a parameter
	lastPart := parts[len(parts)-1]
	if !strings.HasPrefix(lastPart, "{") || !strings.HasSuffix(lastPart, "}") {
		return false
	}

	// Second to last must be a non-parameter (the resource segment)
	secondLastPart := parts[len(parts)-2]
	if strings.HasPrefix(secondLastPart, "{") {
		return false
	}

	// Check if the param name relates to the resource name (e.g., petId for pet)
	paramName := strings.ToLower(lastPart[1 : len(lastPart)-1])
	segmentName := strings.ToLower(secondLastPart)
	singularSegment := strings.ToLower(p.singularize(secondLastPart))

	// Common patterns: {petId}, {id}, {pet_id}, {userId} for /users
	return strings.Contains(paramName, segmentName) ||
		strings.Contains(paramName, singularSegment) ||
		paramName == "id"
}

// getBasePathForIDPath returns the base path for a resource ID path
// e.g., /pet/{petId} -> /pet, /store/order/{orderId} -> /store/order
func (p *Parser) getBasePathForIDPath(path string) string {
	if !p.isResourceIDPath(path) {
		return ""
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		return ""
	}

	// Remove the last part (ID param) to get the base path
	baseParts := parts[:len(parts)-1]
	return "/" + strings.Join(baseParts, "/")
}

// buildResourceIDPaths builds a map of base paths to their corresponding ID paths
// This identifies all resource paths like /pet -> /pet/{petId}
func (p *Parser) buildResourceIDPaths(doc *openapi3.T) map[string]string {
	result := make(map[string]string)

	for path := range doc.Paths.Map() {
		if p.isResourceIDPath(path) {
			basePath := p.getBasePathForIDPath(path)
			if basePath != "" {
				result[basePath] = path
			}
		}
	}

	return result
}

// hasCorrespondingResourceIDPath checks if a path (like /pet with POST) has a
// corresponding resource ID path (like /pet/{petId} with GET/PUT/DELETE)
func (p *Parser) hasCorrespondingResourceIDPath(path string, doc *openapi3.T, resourceIDPaths map[string]string) bool {
	idPath, exists := resourceIDPaths[path]
	if !exists {
		return false
	}

	// Check if the ID path has typical resource operations (GET, PUT, DELETE)
	idPathItem := doc.Paths.Map()[idPath]
	if idPathItem == nil {
		return false
	}

	// Resource ID paths typically have at least GET
	return idPathItem.Get != nil || idPathItem.Put != nil || idPathItem.Delete != nil
}
