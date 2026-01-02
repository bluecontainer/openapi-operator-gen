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
	Method        string
	Path          string
	OperationID   string
	Summary       string
	RequestBody   *Schema
	ResponseBody  *Schema
	PathParams    []Parameter
	QueryParams   []Parameter
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
}

// ParsedSpec contains the parsed OpenAPI specification
type ParsedSpec struct {
	Title       string
	Version     string
	Description string
	BaseURL     string
	Resources   []*Resource
	Schemas     map[string]*Schema
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
		Title:     doc.Info.Title,
		Version:   doc.Info.Version,
		Schemas:   make(map[string]*Schema),
		Resources: make([]*Resource, 0),
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

	// Parse paths and extract resources
	resources := p.extractResources(doc)
	spec.Resources = resources

	return spec, nil
}

func (p *Parser) extractResources(doc *openapi3.T) []*Resource {
	resourceMap := make(map[string]*Resource)

	// Get sorted paths for deterministic output
	paths := make([]string, 0, len(doc.Paths.Map()))
	for path := range doc.Paths.Map() {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		pathItem := doc.Paths.Map()[path]
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

	return resources
}

func (p *Parser) extractResourceName(path string) string {
	// Remove leading slash and split by /
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}

	// Find the first non-parameter path segment
	for _, part := range parts {
		if !strings.HasPrefix(part, "{") {
			// Convert to singular PascalCase
			return p.singularize(p.toPascalCase(part))
		}
	}
	return ""
}

func (p *Parser) getBasePath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return "/"
	}
	// Return the first segment
	return "/" + parts[0]
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
