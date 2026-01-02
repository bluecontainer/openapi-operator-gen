package mapper

import (
	"sort"
	"strings"

	"github.com/example/openapi-operator-gen/internal/config"
	"github.com/example/openapi-operator-gen/pkg/parser"
	"github.com/iancoleman/strcase"
)

// CRDDefinition represents a Kubernetes CRD to be generated
type CRDDefinition struct {
	APIGroup    string
	APIVersion  string
	Kind        string
	Plural      string
	ShortNames  []string
	Scope       string // Namespaced or Cluster
	Description string
	Spec        *FieldDefinition
	Status      *FieldDefinition
	Operations  []OperationMapping
	BasePath    string
}

// OperationMapping maps a CRD operation to a REST API call
type OperationMapping struct {
	CRDAction   string // Create, Update, Delete, Get
	HTTPMethod  string
	Path        string
	PathParams  []string
	QueryParams []string
}

// FieldDefinition represents a field in the CRD spec or status
type FieldDefinition struct {
	Name        string
	JSONName    string
	GoType      string
	Description string
	Required    bool
	Validation  *ValidationRules
	Fields      []*FieldDefinition // nested fields for structs
	ItemType    *FieldDefinition   // for arrays/slices
	Enum        []string
}

// ValidationRules contains kubebuilder validation markers
type ValidationRules struct {
	MinLength *int64
	MaxLength *int64
	Minimum   *float64
	Maximum   *float64
	Pattern   string
	Enum      []string
}

// Mapper maps REST resources to Kubernetes CRD definitions
type Mapper struct {
	config *config.Config
}

// NewMapper creates a new resource mapper
func NewMapper(cfg *config.Config) *Mapper {
	return &Mapper{config: cfg}
}

// MapResources converts parsed OpenAPI resources to CRD definitions
func (m *Mapper) MapResources(spec *parser.ParsedSpec) ([]*CRDDefinition, error) {
	switch m.config.MappingMode {
	case config.SingleCRD:
		return m.mapSingleCRD(spec)
	default:
		return m.mapPerResource(spec)
	}
}

func (m *Mapper) mapPerResource(spec *parser.ParsedSpec) ([]*CRDDefinition, error) {
	crds := make([]*CRDDefinition, 0, len(spec.Resources))

	for _, resource := range spec.Resources {
		crd := &CRDDefinition{
			APIGroup:    m.config.APIGroup,
			APIVersion:  m.config.APIVersion,
			Kind:        resource.Name,
			Plural:      strings.ToLower(resource.PluralName),
			ShortNames:  m.generateShortNames(resource.Name),
			Scope:       "Namespaced",
			Description: resource.Description,
			BasePath:    resource.Path,
			Operations:  m.mapOperations(resource.Operations),
		}

		// Generate spec fields from resource schema
		if resource.Schema != nil {
			crd.Spec = m.schemaToFieldDefinition("Spec", resource.Schema, true)
		} else {
			// Create a generic spec if no schema found
			crd.Spec = m.createGenericSpec()
		}

		// Generate status fields
		crd.Status = m.createStatusDefinition()

		crds = append(crds, crd)
	}

	return crds, nil
}

func (m *Mapper) mapSingleCRD(spec *parser.ParsedSpec) ([]*CRDDefinition, error) {
	// Create a single CRD that handles all resources
	crd := &CRDDefinition{
		APIGroup:    m.config.APIGroup,
		APIVersion:  m.config.APIVersion,
		Kind:        "APIResource",
		Plural:      "apiresources",
		ShortNames:  []string{"apir"},
		Scope:       "Namespaced",
		Description: spec.Description,
		Operations:  make([]OperationMapping, 0),
	}

	// Collect all operations from all resources
	for _, resource := range spec.Resources {
		ops := m.mapOperations(resource.Operations)
		crd.Operations = append(crd.Operations, ops...)
	}

	// Create spec with resourceType field
	resourceTypes := make([]string, 0, len(spec.Resources))
	for _, r := range spec.Resources {
		resourceTypes = append(resourceTypes, r.Name)
	}
	sort.Strings(resourceTypes)

	crd.Spec = &FieldDefinition{
		Name:     "Spec",
		JSONName: "spec",
		GoType:   "struct",
		Fields: []*FieldDefinition{
			{
				Name:        "ResourceType",
				JSONName:    "resourceType",
				GoType:      "string",
				Description: "Type of REST resource to manage",
				Required:    true,
				Enum:        resourceTypes,
			},
			{
				Name:        "ResourcePath",
				JSONName:    "resourcePath",
				GoType:      "string",
				Description: "REST API path for the resource",
				Required:    true,
			},
			{
				Name:        "Method",
				JSONName:    "method",
				GoType:      "string",
				Description: "HTTP method for the operation",
				Required:    false,
				Enum:        []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
			},
			{
				Name:        "Data",
				JSONName:    "data",
				GoType:      "runtime.RawExtension",
				Description: "Request body data as JSON",
				Required:    false,
			},
		},
	}

	crd.Status = m.createStatusDefinition()

	return []*CRDDefinition{crd}, nil
}

func (m *Mapper) mapOperations(ops []parser.Operation) []OperationMapping {
	mappings := make([]OperationMapping, 0, len(ops))

	for _, op := range ops {
		mapping := OperationMapping{
			HTTPMethod:  op.Method,
			Path:        op.Path,
			PathParams:  make([]string, 0),
			QueryParams: make([]string, 0),
		}

		// Map HTTP method to CRD action
		switch op.Method {
		case "POST":
			mapping.CRDAction = "Create"
		case "PUT", "PATCH":
			mapping.CRDAction = "Update"
		case "DELETE":
			mapping.CRDAction = "Delete"
		case "GET":
			mapping.CRDAction = "Get"
		}

		for _, p := range op.PathParams {
			mapping.PathParams = append(mapping.PathParams, p.Name)
		}
		for _, p := range op.QueryParams {
			mapping.QueryParams = append(mapping.QueryParams, p.Name)
		}

		mappings = append(mappings, mapping)
	}

	return mappings
}

func (m *Mapper) schemaToFieldDefinition(name string, schema *parser.Schema, isRoot bool) *FieldDefinition {
	if schema == nil {
		return nil
	}

	field := &FieldDefinition{
		Name:        strcase.ToCamel(name),
		JSONName:    strcase.ToLowerCamel(name),
		Description: schema.Description,
	}

	// Set required if in parent's required list
	for _, req := range schema.Required {
		if req == name {
			field.Required = true
			break
		}
	}

	// Map OpenAPI type to Go type
	field.GoType = m.mapType(schema)

	// Handle validation
	if schema.MinLength != nil || schema.MaxLength != nil || schema.Minimum != nil ||
		schema.Maximum != nil || schema.Pattern != "" || len(schema.Enum) > 0 {
		field.Validation = &ValidationRules{
			MinLength: schema.MinLength,
			MaxLength: schema.MaxLength,
			Minimum:   schema.Minimum,
			Maximum:   schema.Maximum,
			Pattern:   schema.Pattern,
		}
		for _, e := range schema.Enum {
			if s, ok := e.(string); ok {
				field.Validation.Enum = append(field.Validation.Enum, s)
			}
		}
	}

	// Handle nested properties
	if schema.Type == "object" && len(schema.Properties) > 0 {
		field.Fields = make([]*FieldDefinition, 0, len(schema.Properties))

		// Sort property names for deterministic output
		propNames := make([]string, 0, len(schema.Properties))
		for propName := range schema.Properties {
			propNames = append(propNames, propName)
		}
		sort.Strings(propNames)

		for _, propName := range propNames {
			propSchema := schema.Properties[propName]
			propField := m.schemaToFieldDefinition(propName, propSchema, false)
			// Check if property is required
			for _, req := range schema.Required {
				if req == propName {
					propField.Required = true
					break
				}
			}
			field.Fields = append(field.Fields, propField)
		}
	}

	// Handle arrays
	if schema.Type == "array" && schema.Items != nil {
		field.ItemType = m.schemaToFieldDefinition("Item", schema.Items, false)
	}

	// Handle enums
	if len(schema.Enum) > 0 {
		field.Enum = make([]string, 0, len(schema.Enum))
		for _, e := range schema.Enum {
			if s, ok := e.(string); ok {
				field.Enum = append(field.Enum, s)
			}
		}
	}

	return field
}

func (m *Mapper) mapType(schema *parser.Schema) string {
	switch schema.Type {
	case "string":
		switch schema.Format {
		case "date-time":
			return "metav1.Time"
		case "byte":
			return "[]byte"
		default:
			return "string"
		}
	case "integer":
		switch schema.Format {
		case "int64":
			return "int64"
		case "int32":
			return "int32"
		default:
			return "int"
		}
	case "number":
		switch schema.Format {
		case "float":
			return "float32"
		case "double":
			return "float64"
		default:
			return "float64"
		}
	case "boolean":
		return "bool"
	case "array":
		if schema.Items != nil {
			itemType := m.mapType(schema.Items)
			return "[]" + itemType
		}
		return "[]interface{}"
	case "object":
		if len(schema.Properties) == 0 {
			return "map[string]interface{}"
		}
		return "struct"
	default:
		return "interface{}"
	}
}

func (m *Mapper) createGenericSpec() *FieldDefinition {
	return &FieldDefinition{
		Name:     "Spec",
		JSONName: "spec",
		GoType:   "struct",
		Fields: []*FieldDefinition{
			{
				Name:        "Data",
				JSONName:    "data",
				GoType:      "runtime.RawExtension",
				Description: "Resource data as JSON",
				Required:    false,
			},
		},
	}
}

func (m *Mapper) createStatusDefinition() *FieldDefinition {
	return &FieldDefinition{
		Name:     "Status",
		JSONName: "status",
		GoType:   "struct",
		Fields: []*FieldDefinition{
			{
				Name:        "State",
				JSONName:    "state",
				GoType:      "string",
				Description: "Current state of the resource (Pending, Synced, Failed)",
			},
			{
				Name:        "LastSyncTime",
				JSONName:    "lastSyncTime",
				GoType:      "metav1.Time",
				Description: "Last time the resource was synced with the REST API",
			},
			{
				Name:        "ExternalID",
				JSONName:    "externalID",
				GoType:      "string",
				Description: "ID of the resource in the external REST API",
			},
			{
				Name:        "Message",
				JSONName:    "message",
				GoType:      "string",
				Description: "Human-readable message about the current state",
			},
			{
				Name:        "Conditions",
				JSONName:    "conditions",
				GoType:      "[]metav1.Condition",
				Description: "Conditions representing the current state",
			},
			{
				Name:        "ObservedGeneration",
				JSONName:    "observedGeneration",
				GoType:      "int64",
				Description: "Last observed generation of the resource",
			},
			{
				Name:        "Response",
				JSONName:    "response",
				GoType:      "runtime.RawExtension",
				Description: "Last response from the REST API",
			},
		},
	}
}

func (m *Mapper) generateShortNames(kind string) []string {
	// Generate short names based on kind
	lower := strings.ToLower(kind)
	shortNames := []string{}

	// Use first 2-3 characters
	if len(lower) >= 2 {
		shortNames = append(shortNames, lower[:2])
	}
	if len(lower) >= 3 {
		shortNames = append(shortNames, lower[:3])
	}

	return shortNames
}
