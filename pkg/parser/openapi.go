package parser

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

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
	ActionName     string // e.g., "uploadImage"
	HTTPMethod     string // POST or PUT
	Summary        string
	Description    string
	PathParams     []Parameter // Path parameters (excluding parent ID)
	QueryParams    []Parameter // Query parameters
	RequestSchema  *Schema     // Request body schema
	ResponseSchema *Schema     // Response schema
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

// Parser parses OpenAPI specifications
type Parser struct{}

// NewParser creates a new OpenAPI parser
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses an OpenAPI specification file
func (p *Parser) Parse(specPath string) (*ParsedSpec, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	if err := doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI spec: %w", err)
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

func (p *Parser) extractResourcesQueriesAndActions(doc *openapi3.T) ([]*Resource, []*QueryEndpoint, []*ActionEndpoint) {
	resourceMap := make(map[string]*Resource)
	queryEndpoints := make([]*QueryEndpoint, 0)
	actionEndpoints := make([]*ActionEndpoint, 0)

	// Get sorted paths for deterministic output
	paths := make([]string, 0, len(doc.Paths.Map()))
	for path := range doc.Paths.Map() {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		pathItem := doc.Paths.Map()[path]

		// Check if this is a query endpoint
		if queryEndpoint := p.extractQueryEndpoint(path, pathItem, doc); queryEndpoint != nil {
			queryEndpoints = append(queryEndpoints, queryEndpoint)
			continue
		}

		// Check if this is an action endpoint
		if actionEndpoint := p.extractActionEndpoint(path, pathItem, doc); actionEndpoint != nil {
			actionEndpoints = append(actionEndpoints, actionEndpoint)
			continue
		}

		resourceName := p.extractResourceName(path)
		if resourceName == "" {
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

		// Extract operations
		ops := p.extractOperations(path, pathItem)
		resource.Operations = append(resource.Operations, ops...)

		// Try to extract schema from POST/PUT request body
		if resource.Schema == nil {
			resource.Schema = p.extractResourceSchema(pathItem, doc)
		}
	}

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
// Action endpoints have the pattern /{resource}/{resourceId}/{action} with POST or PUT only
func (p *Parser) isActionEndpoint(path string, pathItem *openapi3.PathItem) bool {
	// Must have POST or PUT (but not both regular CRUD operations)
	hasPost := pathItem.Post != nil
	hasPut := pathItem.Put != nil
	hasGet := pathItem.Get != nil
	hasDelete := pathItem.Delete != nil
	hasPatch := pathItem.Patch != nil

	// Action endpoints typically only have POST (or PUT for some actions)
	// and don't have GET/DELETE which would indicate a resource endpoint
	if !hasPost && !hasPut {
		return false
	}

	// If it has GET or DELETE, it's likely a resource endpoint, not an action
	if hasGet || hasDelete || hasPatch {
		return false
	}

	// Check path structure: must have /{resource}/{param}/{action}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 {
		return false
	}

	// Must have a parameter in the middle (like {petId})
	hasParamInMiddle := false
	for i := 1; i < len(parts)-1; i++ {
		if strings.HasPrefix(parts[i], "{") && strings.HasSuffix(parts[i], "}") {
			hasParamInMiddle = true
			break
		}
	}
	if !hasParamInMiddle {
		return false
	}

	// Last segment must be an action (not a parameter)
	lastPart := parts[len(parts)-1]
	if strings.HasPrefix(lastPart, "{") {
		return false
	}

	// Check if the last segment looks like an action
	return p.isActionSegment(lastPart)
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

	// Find the structure: {resource}/{resourceId}/{action}
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

	if parentResource == "" || parentIDParam == "" {
		return nil
	}

	// Build the action endpoint name
	name := parentResource + p.toPascalCase(actionName)

	// Get the operation (prefer POST over PUT)
	var op *openapi3.Operation
	var httpMethod string
	if pathItem.Post != nil {
		op = pathItem.Post
		httpMethod = "POST"
	} else if pathItem.Put != nil {
		op = pathItem.Put
		httpMethod = "PUT"
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

		// Skip the parent ID param as it's handled separately
		if param.Name == parentIDParam {
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
		// Also check for multipart/form-data (common for file uploads)
		if content, ok := op.RequestBody.Value.Content["multipart/form-data"]; ok {
			if content.Schema != nil && content.Schema.Value != nil {
				actionEndpoint.RequestSchema = p.convertSchema("RequestBody", content.Schema.Value)
			}
		}
		// Check for application/octet-stream
		if content, ok := op.RequestBody.Value.Content["application/octet-stream"]; ok {
			if content.Schema != nil && content.Schema.Value != nil {
				actionEndpoint.RequestSchema = p.convertSchema("RequestBody", content.Schema.Value)
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
// Query endpoints are GET-only paths without {id} parameter that have sub-paths like /findByTags
func (p *Parser) isQueryEndpoint(path string, pathItem *openapi3.PathItem) bool {
	// Must have GET operation
	if pathItem.Get == nil {
		return false
	}

	// Must NOT have POST, PUT, PATCH, DELETE (read-only)
	if pathItem.Post != nil || pathItem.Put != nil || pathItem.Patch != nil || pathItem.Delete != nil {
		return false
	}

	// Check path structure - query endpoints typically have action names in the path
	// Examples: /pet/findByStatus, /pet/findByTags, /user/login
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		return false
	}

	// Last segment should be an action name (not a parameter)
	lastPart := parts[len(parts)-1]
	if strings.HasPrefix(lastPart, "{") {
		return false
	}

	// Should contain action verbs or be a sub-resource query
	actionKeywords := []string{"find", "search", "query", "list", "get", "login", "logout", "check", "validate", "lookup"}
	lowerLastPart := strings.ToLower(lastPart)
	for _, keyword := range actionKeywords {
		if strings.Contains(lowerLastPart, keyword) {
			return true
		}
	}

	// Also consider it a query if it has query parameters and is GET-only
	if pathItem.Get != nil && len(pathItem.Get.Parameters) > 0 {
		for _, param := range pathItem.Get.Parameters {
			if param.Value != nil && param.Value.In == "query" {
				return true
			}
		}
	}

	return false
}

// extractQueryEndpoint extracts a query endpoint definition
func (p *Parser) extractQueryEndpoint(path string, pathItem *openapi3.PathItem, doc *openapi3.T) *QueryEndpoint {
	if !p.isQueryEndpoint(path, pathItem) {
		return nil
	}

	op := pathItem.Get
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Build the query endpoint name
	// /pet/findByTags -> PetFindByTags
	// /user/login -> UserLogin
	nameParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if !strings.HasPrefix(part, "{") {
			nameParts = append(nameParts, p.toPascalCase(part))
		}
	}
	name := strings.Join(nameParts, "")

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
		QueryParams: make([]Parameter, 0),
	}

	// Extract query parameters
	for _, paramRef := range op.Parameters {
		if paramRef.Value == nil {
			continue
		}
		if paramRef.Value.In == "query" {
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
		// e.g., "order" followed by "{orderId}"
		if i < len(parts)-1 {
			nextPart := parts[i+1]
			if strings.HasPrefix(nextPart, "{") && strings.HasSuffix(nextPart, "}") {
				paramName := strings.ToLower(nextPart[1 : len(nextPart)-1])
				segmentName := strings.ToLower(part)
				// Check if param name contains the segment name (e.g., "orderid" contains "order")
				// or is a generic "id" parameter
				if strings.Contains(paramName, segmentName) || paramName == "id" {
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
	if strings.HasSuffix(s, "es") {
		return s[:len(s)-2]
	}
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
