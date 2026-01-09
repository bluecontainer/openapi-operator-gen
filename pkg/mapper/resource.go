package mapper

import (
	"sort"
	"strings"

	"github.com/bluecontainer/openapi-operator-gen/internal/config"
	"github.com/bluecontainer/openapi-operator-gen/pkg/parser"
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

	// Query endpoint fields
	IsQuery         bool               // True if this is a query/action CRD
	QueryPath       string             // Full query path (e.g., /pet/findByTags)
	QueryParams     []QueryParamField  // Query parameters for building URL
	ResponseType    string             // Go type for response (e.g., "[]Pet", "Pet")
	ResponseIsArray bool               // True if response is an array
	ResultItemType  string             // Item type if ResponseIsArray (e.g., "Pet")
	ResultFields    []*FieldDefinition // Fields for the result type (used to generate result struct)
	UsesSharedType  bool               // True if ResultItemType is a shared type from another CRD

	// Action endpoint fields
	IsAction       bool   // True if this is an action CRD (one-shot operation)
	ActionPath     string // Full action path (e.g., /pet/{petId}/uploadImage)
	ActionMethod   string // HTTP method (POST or PUT)
	ParentResource string // Parent resource kind (e.g., "Pet")
	ParentIDParam  string // Parent ID parameter name (e.g., "petId")
	ActionName     string // Action name (e.g., "uploadImage")
}

// QueryParamField represents a query parameter as a spec field
type QueryParamField struct {
	Name        string
	JSONName    string
	GoType      string
	Description string
	Required    bool
	IsArray     bool   // True if this is an array parameter (e.g., tags[])
	ItemType    string // Type of array items if IsArray is true
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
	MinItems  *int64
	MaxItems  *int64
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
	var crds []*CRDDefinition

	switch m.config.MappingMode {
	case config.SingleCRD:
		crds, _ = m.mapSingleCRD(spec)
	default:
		crds, _ = m.mapPerResource(spec)
	}

	// Collect known resource kinds for type reuse in query endpoints
	knownKinds := make(map[string]bool)
	for _, crd := range crds {
		knownKinds[crd.Kind] = true
	}

	// Also map query endpoints to CRDs
	queryCRDs := m.mapQueryEndpoints(spec.QueryEndpoints, knownKinds)
	crds = append(crds, queryCRDs...)

	// Also map action endpoints to CRDs
	actionCRDs := m.mapActionEndpoints(spec.ActionEndpoints, knownKinds)
	crds = append(crds, actionCRDs...)

	return crds, nil
}

// mapQueryEndpoints converts query endpoints to CRD definitions
func (m *Mapper) mapQueryEndpoints(queryEndpoints []*parser.QueryEndpoint, knownKinds map[string]bool) []*CRDDefinition {
	crds := make([]*CRDDefinition, 0, len(queryEndpoints))

	for _, qe := range queryEndpoints {
		crd := &CRDDefinition{
			APIGroup:    m.config.APIGroup,
			APIVersion:  m.config.APIVersion,
			Kind:        qe.Name,
			Plural:      strings.ToLower(qe.Name) + "s",
			ShortNames:  m.generateShortNames(qe.Name),
			Scope:       "Namespaced",
			Description: qe.Summary,
			BasePath:    qe.BasePath,
			IsQuery:     true,
			QueryPath:   qe.Path,
			QueryParams: m.mapQueryParams(qe.QueryParams),
		}

		// Generate spec fields from query parameters
		crd.Spec = m.createQuerySpec(qe)

		// Map response schema to typed result fields
		m.mapResponseSchema(crd, qe, knownKinds)

		// Generate status fields (includes results)
		crd.Status = m.createQueryStatusDefinition()

		// Add a single GET operation
		crd.Operations = []OperationMapping{
			{
				CRDAction:  "Query",
				HTTPMethod: "GET",
				Path:       qe.Path,
			},
		}

		crds = append(crds, crd)
	}

	return crds
}

// mapActionEndpoints converts action endpoints to CRD definitions
func (m *Mapper) mapActionEndpoints(actionEndpoints []*parser.ActionEndpoint, knownKinds map[string]bool) []*CRDDefinition {
	crds := make([]*CRDDefinition, 0, len(actionEndpoints))

	for _, ae := range actionEndpoints {
		crd := &CRDDefinition{
			APIGroup:       m.config.APIGroup,
			APIVersion:     m.config.APIVersion,
			Kind:           ae.Name,
			Plural:         strings.ToLower(ae.Name) + "s",
			ShortNames:     m.generateShortNames(ae.Name),
			Scope:          "Namespaced",
			Description:    ae.Summary,
			IsAction:       true,
			ActionPath:     ae.Path,
			ActionMethod:   ae.HTTPMethod,
			ParentResource: ae.ParentResource,
			ParentIDParam:  ae.ParentIDParam,
			ActionName:     ae.ActionName,
		}

		// Generate spec fields from request schema and path params
		crd.Spec = m.createActionSpec(ae)

		// Map response schema
		m.mapActionResponseSchema(crd, ae, knownKinds)

		// Generate status fields
		crd.Status = m.createActionStatusDefinition()

		// Add single operation for the action
		crd.Operations = []OperationMapping{
			{
				CRDAction:  "Execute",
				HTTPMethod: ae.HTTPMethod,
				Path:       ae.Path,
			},
		}

		crds = append(crds, crd)
	}

	return crds
}

// createActionSpec creates the spec definition for an action CRD
func (m *Mapper) createActionSpec(ae *parser.ActionEndpoint) *FieldDefinition {
	spec := &FieldDefinition{
		Name:     "Spec",
		JSONName: "spec",
		GoType:   "struct",
		Fields:   make([]*FieldDefinition, 0),
	}

	// Add parent resource ID field (required) - only if the action has a parent ID
	if ae.ParentIDParam != "" {
		parentIDField := &FieldDefinition{
			Name:        strcase.ToCamel(ae.ParentIDParam),
			JSONName:    strcase.ToLowerCamel(ae.ParentIDParam),
			GoType:      "string",
			Description: "ID of the parent " + ae.ParentResource + " resource",
			Required:    true,
		}
		spec.Fields = append(spec.Fields, parentIDField)
	}

	// Add re-execution interval field (optional)
	reExecuteIntervalField := &FieldDefinition{
		Name:        "ReExecuteInterval",
		JSONName:    "reExecuteInterval",
		GoType:      "*metav1.Duration",
		Description: "Interval at which to re-execute the action (e.g., 30s, 5m, 1h). If not set, action is one-shot.",
		Required:    false,
	}
	spec.Fields = append(spec.Fields, reExecuteIntervalField)

	// Add path parameters (excluding parent ID which is already added)
	for _, param := range ae.PathParams {
		if param.Name == ae.ParentIDParam {
			continue
		}
		field := &FieldDefinition{
			Name:        strcase.ToCamel(param.Name),
			JSONName:    strcase.ToLowerCamel(param.Name),
			GoType:      m.mapParamType(param.Type),
			Description: param.Description,
			Required:    param.Required,
		}
		spec.Fields = append(spec.Fields, field)
	}

	// Add query parameters
	for _, param := range ae.QueryParams {
		goType := m.mapParamType(param.Type)
		isArray := false

		if strings.HasPrefix(param.Type, "array:") {
			isArray = true
			itemType := strings.TrimPrefix(param.Type, "array:")
			goType = "[]" + m.mapParamType(itemType)
		}

		field := &FieldDefinition{
			Name:        strcase.ToCamel(param.Name),
			JSONName:    strcase.ToLowerCamel(param.Name),
			GoType:      goType,
			Description: param.Description,
			Required:    param.Required,
		}

		if isArray {
			field.ItemType = &FieldDefinition{
				GoType: m.mapParamType(strings.TrimPrefix(param.Type, "array:")),
			}
		}

		spec.Fields = append(spec.Fields, field)
	}

	// Add request body fields if present
	if ae.RequestSchema != nil && len(ae.RequestSchema.Properties) > 0 {
		// Sort property names for deterministic output
		propNames := make([]string, 0, len(ae.RequestSchema.Properties))
		for propName := range ae.RequestSchema.Properties {
			propNames = append(propNames, propName)
		}
		sort.Strings(propNames)

		for _, propName := range propNames {
			propSchema := ae.RequestSchema.Properties[propName]
			propField := m.schemaToFieldDefinition(propName, propSchema, false)
			// Check if property is required
			for _, req := range ae.RequestSchema.Required {
				if req == propName {
					propField.Required = true
					break
				}
			}
			spec.Fields = append(spec.Fields, propField)
		}
	}

	return spec
}

// mapActionResponseSchema maps the response schema for action CRDs
func (m *Mapper) mapActionResponseSchema(crd *CRDDefinition, ae *parser.ActionEndpoint, knownKinds map[string]bool) {
	if ae.ResponseSchema == nil {
		crd.ResponseType = "*runtime.RawExtension"
		return
	}

	schema := ae.ResponseSchema

	// Check if response is an array
	if schema.Type == "array" && schema.Items != nil {
		itemSchema := schema.Items
		crd.ResponseIsArray = true
		crd.ResultItemType = crd.Kind + "Result"

		if itemSchema.Type == "object" && len(itemSchema.Properties) > 0 {
			crd.ResultFields = m.schemaToResultFields(itemSchema)
			crd.ResponseType = "[]" + crd.ResultItemType
		} else {
			crd.ResponseType = "[]" + m.mapType(itemSchema)
			crd.ResultFields = nil
		}
	} else if schema.Type == "object" && len(schema.Properties) > 0 {
		crd.ResultItemType = crd.Kind + "Result"
		crd.ResultFields = m.schemaToResultFields(schema)
		crd.ResponseType = "*" + crd.ResultItemType
	} else {
		crd.ResponseType = "*runtime.RawExtension"
	}
}

// createActionStatusDefinition creates status fields for action CRDs
func (m *Mapper) createActionStatusDefinition() *FieldDefinition {
	return &FieldDefinition{
		Name:     "Status",
		JSONName: "status",
		GoType:   "struct",
		Fields: []*FieldDefinition{
			{
				Name:        "State",
				JSONName:    "state",
				GoType:      "string",
				Description: "Current state of the action (Pending, Executing, Completed, Failed)",
			},
			{
				Name:        "ExecutedAt",
				JSONName:    "executedAt",
				GoType:      "metav1.Time",
				Description: "Time when the action was first executed",
			},
			{
				Name:        "CompletedAt",
				JSONName:    "completedAt",
				GoType:      "metav1.Time",
				Description: "Time when the action completed",
			},
			{
				Name:        "LastExecutionTime",
				JSONName:    "lastExecutionTime",
				GoType:      "*metav1.Time",
				Description: "Time of the last execution (used to calculate next re-execution time)",
			},
			{
				Name:        "NextExecutionTime",
				JSONName:    "nextExecutionTime",
				GoType:      "*metav1.Time",
				Description: "Calculated time when the next re-execution will occur (if reExecuteInterval is set)",
			},
			{
				Name:        "ExecutionCount",
				JSONName:    "executionCount",
				GoType:      "int",
				Description: "Number of times the action has been executed",
			},
			{
				Name:        "Message",
				JSONName:    "message",
				GoType:      "string",
				Description: "Human-readable message about the current state",
			},
			{
				Name:        "HTTPStatusCode",
				JSONName:    "httpStatusCode",
				GoType:      "int",
				Description: "HTTP status code from the action response",
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
				Description: "Response from the action execution",
			},
		},
	}
}

// mapResponseSchema maps the response schema to typed result fields
func (m *Mapper) mapResponseSchema(crd *CRDDefinition, qe *parser.QueryEndpoint, knownKinds map[string]bool) {
	if qe.ResponseSchema == nil {
		// No response schema - use raw extension fallback
		crd.ResponseType = "*runtime.RawExtension"
		return
	}

	schema := qe.ResponseSchema
	crd.ResponseIsArray = qe.ResponseIsArray

	// Check if response references a known resource type (e.g., Pet)
	if qe.ResponseSchemaRef != "" {
		// Convert schema ref to PascalCase Kind (e.g., "Pet" -> "Pet")
		refKind := m.singularize(m.toPascalCase(qe.ResponseSchemaRef))

		if knownKinds[refKind] {
			// Use the existing resource type
			crd.UsesSharedType = true
			crd.ResultItemType = refKind
			crd.ResultFields = nil // Don't generate fields, use shared type

			if qe.ResponseIsArray {
				crd.ResponseType = "[]" + refKind
			} else {
				crd.ResponseType = "*" + refKind
			}
			return
		}
	}

	// Check if response is an array
	if schema.Type == "array" && schema.Items != nil {
		itemSchema := schema.Items

		// Generate the item type name based on the query name
		// e.g., PetFindByTags -> PetFindByTagsResult
		crd.ResultItemType = crd.Kind + "Result"

		// If item is an object with properties, extract fields
		if itemSchema.Type == "object" && len(itemSchema.Properties) > 0 {
			crd.ResultFields = m.schemaToResultFields(itemSchema)
			crd.ResponseType = "[]" + crd.ResultItemType
		} else {
			// Simple array type (e.g., []string)
			crd.ResponseType = "[]" + m.mapType(itemSchema)
			crd.ResultFields = nil
		}
	} else if schema.Type == "object" && len(schema.Properties) > 0 {
		// Single object response
		crd.ResultItemType = crd.Kind + "Result"
		crd.ResultFields = m.schemaToResultFields(schema)
		crd.ResponseType = "*" + crd.ResultItemType
	} else {
		// Fallback to raw extension for unknown types
		crd.ResponseType = "*runtime.RawExtension"
	}
}

// toPascalCase converts a string to PascalCase
func (m *Mapper) toPascalCase(s string) string {
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

// singularize converts a plural word to singular
func (m *Mapper) singularize(s string) string {
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

// schemaToResultFields converts a schema to field definitions for the result type
func (m *Mapper) schemaToResultFields(schema *parser.Schema) []*FieldDefinition {
	if schema == nil || len(schema.Properties) == 0 {
		return nil
	}

	fields := make([]*FieldDefinition, 0, len(schema.Properties))

	// Sort property names for deterministic output
	propNames := make([]string, 0, len(schema.Properties))
	for propName := range schema.Properties {
		propNames = append(propNames, propName)
	}
	sort.Strings(propNames)

	for _, propName := range propNames {
		propSchema := schema.Properties[propName]
		field := m.schemaToFieldDefinition(propName, propSchema, false)

		// Check if property is required
		for _, req := range schema.Required {
			if req == propName {
				field.Required = true
				break
			}
		}

		fields = append(fields, field)
	}

	return fields
}

// mapQueryParams converts parser query params to QueryParamField
func (m *Mapper) mapQueryParams(params []parser.Parameter) []QueryParamField {
	fields := make([]QueryParamField, 0, len(params))

	for _, p := range params {
		field := QueryParamField{
			Name:        strcase.ToCamel(p.Name),
			JSONName:    strcase.ToLowerCamel(p.Name),
			Description: p.Description,
			Required:    p.Required,
		}

		// Handle array types (e.g., "array:string")
		if strings.HasPrefix(p.Type, "array:") {
			field.IsArray = true
			field.ItemType = strings.TrimPrefix(p.Type, "array:")
			field.GoType = "[]" + m.mapParamType(field.ItemType)
		} else {
			field.GoType = m.mapParamType(p.Type)
		}

		fields = append(fields, field)
	}

	return fields
}

// mapParamType maps OpenAPI parameter types to Go types
func (m *Mapper) mapParamType(t string) string {
	switch t {
	case "string":
		return "string"
	case "integer":
		return "int64"
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	default:
		return "string"
	}
}

// createQuerySpec creates the spec definition for a query CRD
func (m *Mapper) createQuerySpec(qe *parser.QueryEndpoint) *FieldDefinition {
	spec := &FieldDefinition{
		Name:     "Spec",
		JSONName: "spec",
		GoType:   "struct",
		Fields:   make([]*FieldDefinition, 0),
	}

	// Add path parameters as spec fields
	for _, param := range qe.PathParams {
		field := &FieldDefinition{
			Name:        strcase.ToCamel(param.Name),
			JSONName:    strcase.ToLowerCamel(param.Name),
			GoType:      m.mapParamType(param.Type),
			Description: param.Description,
			Required:    param.Required,
		}
		spec.Fields = append(spec.Fields, field)
	}

	// Add query parameters as spec fields
	for _, param := range qe.QueryParams {
		goType := m.mapParamType(param.Type)
		isArray := false

		// Handle array types
		if strings.HasPrefix(param.Type, "array:") {
			isArray = true
			itemType := strings.TrimPrefix(param.Type, "array:")
			goType = "[]" + m.mapParamType(itemType)
		}

		field := &FieldDefinition{
			Name:        strcase.ToCamel(param.Name),
			JSONName:    strcase.ToLowerCamel(param.Name),
			GoType:      goType,
			Description: param.Description,
			Required:    param.Required,
		}

		// Add item type info for arrays
		if isArray {
			field.ItemType = &FieldDefinition{
				GoType: m.mapParamType(strings.TrimPrefix(param.Type, "array:")),
			}
		}

		spec.Fields = append(spec.Fields, field)
	}

	return spec
}

// createQueryStatusDefinition creates status fields for query CRDs
func (m *Mapper) createQueryStatusDefinition() *FieldDefinition {
	return &FieldDefinition{
		Name:     "Status",
		JSONName: "status",
		GoType:   "struct",
		Fields: []*FieldDefinition{
			{
				Name:        "State",
				JSONName:    "state",
				GoType:      "string",
				Description: "Current state of the query (Pending, Queried, Failed)",
			},
			{
				Name:        "LastQueryTime",
				JSONName:    "lastQueryTime",
				GoType:      "metav1.Time",
				Description: "Last time the query was executed",
			},
			{
				Name:        "ResultCount",
				JSONName:    "resultCount",
				GoType:      "int",
				Description: "Number of results returned by the query",
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
				Name:        "Results",
				JSONName:    "results",
				GoType:      "runtime.RawExtension",
				Description: "Query results from the REST API",
			},
		},
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

		// Add path and query parameters from operations to spec fields
		m.addOperationParamsToSpec(crd.Spec, resource.Operations)

		// Generate status fields
		crd.Status = m.createStatusDefinition()

		crds = append(crds, crd)
	}

	return crds, nil
}

// addOperationParamsToSpec adds path and query parameters from operations to the spec
func (m *Mapper) addOperationParamsToSpec(spec *FieldDefinition, operations []parser.Operation) {
	if spec == nil {
		return
	}

	// Track existing field names to avoid duplicates
	existingFields := make(map[string]bool)
	for _, field := range spec.Fields {
		existingFields[strings.ToLower(field.JSONName)] = true
	}

	// Collect unique path params from all operations
	pathParamsSeen := make(map[string]bool)
	for _, op := range operations {
		for _, param := range op.PathParams {
			paramKey := strings.ToLower(param.Name)
			if pathParamsSeen[paramKey] || existingFields[paramKey] {
				continue
			}
			pathParamsSeen[paramKey] = true

			field := &FieldDefinition{
				Name:        strcase.ToCamel(param.Name),
				JSONName:    strcase.ToLowerCamel(param.Name),
				GoType:      m.mapParamType(param.Type),
				Description: param.Description,
				Required:    param.Required,
			}
			spec.Fields = append(spec.Fields, field)
			existingFields[paramKey] = true
		}
	}

	// Collect unique query params from all operations
	queryParamsSeen := make(map[string]bool)
	for _, op := range operations {
		for _, param := range op.QueryParams {
			paramKey := strings.ToLower(param.Name)
			if queryParamsSeen[paramKey] || existingFields[paramKey] {
				continue
			}
			queryParamsSeen[paramKey] = true

			goType := m.mapParamType(param.Type)
			isArray := false

			if strings.HasPrefix(param.Type, "array:") {
				isArray = true
				itemType := strings.TrimPrefix(param.Type, "array:")
				goType = "[]" + m.mapParamType(itemType)
			}

			field := &FieldDefinition{
				Name:        strcase.ToCamel(param.Name),
				JSONName:    strcase.ToLowerCamel(param.Name),
				GoType:      goType,
				Description: param.Description,
				Required:    param.Required,
			}

			if isArray {
				field.ItemType = &FieldDefinition{
					GoType: m.mapParamType(strings.TrimPrefix(param.Type, "array:")),
				}
			}

			spec.Fields = append(spec.Fields, field)
			existingFields[paramKey] = true
		}
	}
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
		schema.Maximum != nil || schema.Pattern != "" || len(schema.Enum) > 0 ||
		schema.MinItems != nil || schema.MaxItems != nil {
		field.Validation = &ValidationRules{
			MinLength: schema.MinLength,
			MaxLength: schema.MaxLength,
			Minimum:   schema.Minimum,
			Maximum:   schema.Maximum,
			Pattern:   schema.Pattern,
			MinItems:  schema.MinItems,
			MaxItems:  schema.MaxItems,
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
