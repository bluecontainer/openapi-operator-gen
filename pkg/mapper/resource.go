package mapper

import (
	"sort"
	"strings"

	"github.com/bluecontainer/openapi-operator-gen/internal/config"
	"github.com/bluecontainer/openapi-operator-gen/pkg/parser"
	"github.com/iancoleman/strcase"
)

// pluralize converts a Kind name to its plural form for Kubernetes resource names.
// Example: "Order" -> "orders", "Pet" -> "pets", "StoreInventoryQuery" -> "storeinventoryqueries"
func pluralize(kind string) string {
	lower := strings.ToLower(kind)
	// Handle common irregular pluralizations
	if strings.HasSuffix(lower, "query") {
		return lower[:len(lower)-1] + "ies" // query -> queries
	}
	if strings.HasSuffix(lower, "y") && len(lower) > 1 {
		// Check if preceded by a consonant (not a vowel)
		prev := lower[len(lower)-2]
		if prev != 'a' && prev != 'e' && prev != 'i' && prev != 'o' && prev != 'u' {
			return lower[:len(lower)-1] + "ies" // e.g., policy -> policies
		}
	}
	if strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "x") ||
		strings.HasSuffix(lower, "ch") || strings.HasSuffix(lower, "sh") {
		return lower + "es" // e.g., class -> classes, box -> boxes
	}
	return lower + "s"
}

// CRDDefinition represents a Kubernetes CRD to be generated
type CRDDefinition struct {
	APIGroup     string
	APIVersion   string
	Kind         string
	Plural       string
	ShortNames   []string
	Scope        string // Namespaced or Cluster
	Description  string
	Spec         *FieldDefinition
	Status       *FieldDefinition
	Operations   []OperationMapping
	BasePath     string
	ResourcePath string // Full path template with placeholders (e.g., /classes/{className}/variables/{variableName})

	// Per-method paths (when different methods use different paths)
	GetPath    string // Path for GET operations (e.g., /pet/{petId})
	PutPath    string // Path for PUT operations (e.g., /pet - when ID is in body)
	DeletePath string // Path for DELETE operations (e.g., /pet/{petId})

	// HTTP method availability (for conditional controller logic)
	HasDelete bool // True if DELETE method is available for this resource
	HasPost   bool // True if POST method is available for this resource
	HasPut    bool // True if PUT method is available for this resource
	HasPatch  bool // True if PATCH method is available for this resource

	// UpdateWithPost enables using POST for updates when PUT is not available.
	// This is set when --update-with-post flag is used AND HasPut is false AND HasPost is true.
	UpdateWithPost bool

	// ExternalIDRef handling
	NeedsExternalIDRef bool // True if externalIDRef field is needed (no path params to identify resource)

	// Query endpoint fields
	IsQuery            bool               // True if this is a query/action CRD
	QueryPath          string             // Full query path (e.g., /pet/findByTags)
	QueryPathParams    []QueryParamField  // Path parameters for query endpoints (e.g., serviceName, parameterName)
	QueryParams        []QueryParamField  // Query parameters for building URL
	ResponseType       string             // Go type for response (e.g., "[]Pet", "Pet")
	ResponseIsArray    bool               // True if response is an array
	ResultItemType     string             // Item type if ResponseIsArray (e.g., "Pet")
	ResultFields       []*FieldDefinition // Fields for the result type (used to generate result struct)
	UsesSharedType     bool               // True if ResultItemType is a shared type from another CRD
	IsPrimitiveArray   bool               // True if response is a simple array of primitives ([]string, []int, etc.)
	PrimitiveArrayType string             // The Go type for primitive arrays (e.g., "string", "int64")

	// Action endpoint fields
	IsAction       bool   // True if this is an action CRD (one-shot operation)
	ActionPath     string // Full action path (e.g., /pet/{petId}/uploadImage)
	ActionMethod   string // HTTP method (POST or PUT)
	ParentResource string // Parent resource kind (e.g., "Pet")
	ParentIDParam  string // Parent ID parameter name (e.g., "petId")
	ParentIDType   string // Parent ID OpenAPI type (e.g., "integer")
	ParentIDGoType string // Parent ID Go type (e.g., "int64", "string")
	ActionName     string // Action name (e.g., "uploadImage")

	// Binary upload fields
	HasBinaryBody     bool   // True if the action accepts binary data uploads
	BinaryContentType string // Content type for binary data (e.g., "application/octet-stream")

	// IDFieldMappings stores mappings from path parameters to body fields.
	// This is used when a path param like {orderId} maps to the body's "id" field.
	// The controller uses this to:
	// 1. Build URLs using the merged field's value
	// 2. Optionally rename the field in the JSON body when sending to the API
	IDFieldMappings []IDFieldMapping

	// CELValidationRules contains CEL validation rules for conditional field requirements.
	// These rules make OpenAPI-required fields optional when referencing existing resources
	// via path parameters or externalIDRef.
	CELValidationRules []CELValidationRule
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
	IsPointer   bool   // True if this is a pointer type (for optional numeric types)
	BaseType    string // Base type without pointer (e.g., "int64" for "*int64")
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
	// PathParamName is set when this field is merged with a path parameter.
	// The controller uses this to substitute the field value into the URL path.
	// e.g., for orderId -> id merge, the "id" field will have PathParamName = "orderId"
	PathParamName string
	// OpenAPIRequired indicates if this field is required in the OpenAPI spec for resource creation.
	// This is used to generate CEL validation rules that make the field conditionally required
	// (required when creating a new resource, optional when referencing an existing one).
	OpenAPIRequired bool
}

// IDFieldMapping represents a mapping from a path parameter to a body field.
// This is used to handle cases where {orderId} in the URL maps to "id" in the request body.
type IDFieldMapping struct {
	PathParam string // The path parameter name (e.g., "orderId")
	BodyField string // The body field name (e.g., "id")
}

// CELValidationRule represents a CEL validation rule for conditional field requirements.
// This is used to make OpenAPI-required fields optional when referencing existing resources.
type CELValidationRule struct {
	Rule    string // The CEL expression (e.g., "has(self.petId) || has(self.name)")
	Message string // The validation error message
}

// generateCELValidationRules creates CEL validation rules for conditional field requirements.
// For each OpenAPI-required field, it generates a rule that makes the field optional when:
// - A path parameter field is set (e.g., petId for /pet/{petId})
// - OR externalIDRef is set (for resources without path params)
// This allows users to reference existing resources without providing all creation fields.
func generateCELValidationRules(crd *CRDDefinition) {
	if crd.Spec == nil || len(crd.Spec.Fields) == 0 {
		return
	}

	// Skip Query and Action CRDs - they don't need conditional validation
	if crd.IsQuery || crd.IsAction {
		return
	}

	// Find path parameter fields (fields that can identify an existing resource)
	var pathParamFields []string
	for _, field := range crd.Spec.Fields {
		if field.PathParamName != "" {
			// This field is merged with a path param, so it can identify existing resources
			pathParamFields = append(pathParamFields, field.JSONName)
		}
	}

	// Check if we have any path params or need externalIDRef
	hasPathParams := len(pathParamFields) > 0
	needsExternalIDRef := crd.NeedsExternalIDRef

	// If there's no way to reference existing resources, no CEL rules needed
	if !hasPathParams && !needsExternalIDRef {
		return
	}

	// Build the condition prefix for referencing existing resources
	// e.g., "has(self.petId)" or "has(self.externalIDRef)"
	var conditions []string
	for _, pathParam := range pathParamFields {
		conditions = append(conditions, "has(self."+pathParam+")")
	}
	if needsExternalIDRef && crd.HasPost {
		// Only add externalIDRef condition if POST is available (optional externalIDRef case)
		conditions = append(conditions, "has(self.externalIDRef)")
	}

	// If no conditions to check, no rules needed
	if len(conditions) == 0 {
		return
	}

	conditionPrefix := strings.Join(conditions, " || ")

	// Generate CEL rules for each OpenAPI-required field
	for _, field := range crd.Spec.Fields {
		if !field.OpenAPIRequired {
			continue
		}

		// Skip the path param fields themselves - they're the condition, not the target
		isPathParam := false
		for _, pp := range pathParamFields {
			if field.JSONName == pp {
				isPathParam = true
				break
			}
		}
		if isPathParam {
			continue
		}

		// Generate the rule: "condition || has(self.fieldName)"
		rule := conditionPrefix + " || has(self." + field.JSONName + ")"
		message := field.JSONName + " is required when creating a new resource"

		crd.CELValidationRules = append(crd.CELValidationRules, CELValidationRule{
			Rule:    rule,
			Message: message,
		})

		// Mark the field as no longer strictly required since CEL handles it
		field.Required = false
	}
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

	// Generate CEL validation rules for conditional field requirements
	for _, crd := range crds {
		generateCELValidationRules(crd)
	}

	return crds, nil
}

// mapQueryEndpoints converts query endpoints to CRD definitions
func (m *Mapper) mapQueryEndpoints(queryEndpoints []*parser.QueryEndpoint, knownKinds map[string]bool) []*CRDDefinition {
	crds := make([]*CRDDefinition, 0, len(queryEndpoints))
	seenKinds := make(map[string]bool)

	for _, qe := range queryEndpoints {
		// Skip duplicate Kind names to avoid redeclaration errors
		if seenKinds[qe.Name] {
			continue
		}
		seenKinds[qe.Name] = true

		crd := &CRDDefinition{
			APIGroup:        m.config.APIGroup,
			APIVersion:      m.config.APIVersion,
			Kind:            qe.Name,
			Plural:          pluralize(qe.Name),
			ShortNames:      []string{}, // Query CRDs don't get short names to avoid conflicts
			Scope:           "Namespaced",
			Description:     qe.Summary,
			BasePath:        qe.BasePath,
			IsQuery:         true,
			QueryPath:       qe.Path,
			QueryPathParams: m.mapQueryPathParams(qe.PathParams),
			QueryParams:     m.mapQueryParams(qe.QueryParams),
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
	seenKinds := make(map[string]bool)

	for _, ae := range actionEndpoints {
		// Skip duplicate Kind names to avoid redeclaration errors
		if seenKinds[ae.Name] {
			continue
		}
		seenKinds[ae.Name] = true

		crd := &CRDDefinition{
			APIGroup:          m.config.APIGroup,
			APIVersion:        m.config.APIVersion,
			Kind:              ae.Name,
			Plural:            pluralize(ae.Name),
			ShortNames:        []string{}, // Action CRDs don't get short names to avoid conflicts
			Scope:             "Namespaced",
			Description:       ae.Summary,
			IsAction:          true,
			ActionPath:        ae.Path,
			ActionMethod:      ae.HTTPMethod,
			ParentResource:    ae.ParentResource,
			ParentIDParam:     ae.ParentIDParam,
			ParentIDType:      ae.ParentIDType,
			ParentIDGoType:    m.mapParamType(ae.ParentIDType),
			ActionName:        ae.ActionName,
			HasBinaryBody:     ae.HasBinaryBody,
			BinaryContentType: ae.BinaryContentType,
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
		// Use the actual type from the OpenAPI spec for the parent ID field
		parentIDGoType := m.mapParamType(ae.ParentIDType)
		if parentIDGoType == "" {
			parentIDGoType = "string" // fallback for unspecified types
		}
		parentIDField := &FieldDefinition{
			Name:        strcase.ToCamel(ae.ParentIDParam),
			JSONName:    strcase.ToLowerCamel(ae.ParentIDParam),
			GoType:      parentIDGoType,
			Description: "ID of the parent " + ae.ParentResource + " resource",
			Required:    true,
		}
		spec.Fields = append(spec.Fields, parentIDField)
	}

	// Add execution interval field (optional) - normalized across Query/Action/Resource CRDs
	executionIntervalField := &FieldDefinition{
		Name:        "ExecutionInterval",
		JSONName:    "executionInterval",
		GoType:      "*metav1.Duration",
		Description: "Interval at which to re-execute (e.g., 30s, 5m, 1h). If not set, uses controller default. Set to 0 to disable periodic re-execution.",
		Required:    false,
	}
	spec.Fields = append(spec.Fields, executionIntervalField)

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
			// Check if property is required in OpenAPI spec
			for _, req := range ae.RequestSchema.Required {
				if req == propName {
					propField.Required = true
					propField.OpenAPIRequired = true // Mark as OpenAPI-required for CEL validation
					break
				}
			}
			spec.Fields = append(spec.Fields, propField)
		}
	}

	// Add binary upload fields if the action has binary body
	if ae.HasBinaryBody {
		spec.Fields = append(spec.Fields, m.createBinaryUploadFields()...)
	}

	return spec
}

// createBinaryUploadFields creates the fields for binary upload support
// Supports multiple data sources: inline base64, ConfigMap/Secret reference, URL, and PVC
func (m *Mapper) createBinaryUploadFields() []*FieldDefinition {
	fields := make([]*FieldDefinition, 0)

	// Option 1: Inline base64-encoded data
	fields = append(fields, &FieldDefinition{
		Name:        "Data",
		JSONName:    "data",
		GoType:      "string",
		Description: "Base64-encoded binary data to upload. Mutually exclusive with dataFrom, dataURL, and dataFromFile.",
		Required:    false,
	})

	// Option 2: Reference to ConfigMap or Secret
	fields = append(fields, &FieldDefinition{
		Name:        "DataFrom",
		JSONName:    "dataFrom",
		GoType:      "*BinaryDataSource",
		Description: "Reference to a ConfigMap or Secret containing the binary data. Mutually exclusive with data, dataURL, and dataFromFile.",
		Required:    false,
	})

	// Option 3: URL to fetch data from
	fields = append(fields, &FieldDefinition{
		Name:        "DataURL",
		JSONName:    "dataURL",
		GoType:      "string",
		Description: "URL to fetch binary data from. Mutually exclusive with data, dataFrom, and dataFromFile.",
		Required:    false,
	})

	// Option 4: File path reference
	fields = append(fields, &FieldDefinition{
		Name:        "DataFromFile",
		JSONName:    "dataFromFile",
		GoType:      "*FileDataSource",
		Description: "Path to a file containing the binary data (requires operator filesystem access). Mutually exclusive with data, dataFrom, and dataURL.",
		Required:    false,
	})

	// Content type override (optional)
	fields = append(fields, &FieldDefinition{
		Name:        "ContentType",
		JSONName:    "contentType",
		GoType:      "string",
		Description: "Content-Type header to use for the upload. Defaults to application/octet-stream.",
		Required:    false,
	})

	return fields
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

	// NOTE: We intentionally do NOT reuse existing resource types (like PetSpec) for query results.
	// The Spec types include controller-specific fields (TargetPodOrdinal, TargetHelmRelease,
	// MergeOnUpdate, OnDelete, etc.) that don't exist in the API response. If we reused PetSpec,
	// the CRD schema would expect these fields but the API response wouldn't have them, causing
	// validation errors like "unknown field" and "expected map, got string".
	//
	// Instead, we always generate a dedicated result type from the response schema, which only
	// contains the actual API response fields.

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
			// Simple array type (e.g., []string, []int64)
			itemType := m.mapType(itemSchema)
			crd.ResponseType = "[]" + itemType
			crd.ResultFields = nil
			// Mark as primitive array so the template can generate proper typed field
			if m.isPrimitiveType(itemType) {
				crd.IsPrimitiveArray = true
				crd.PrimitiveArrayType = itemType
			}
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

		// Check if property is required in OpenAPI spec
		for _, req := range schema.Required {
			if req == propName {
				field.Required = true
				field.OpenAPIRequired = true // Mark as OpenAPI-required for CEL validation
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
			field.BaseType = field.GoType
		} else {
			baseType := m.mapParamType(p.Type)
			field.BaseType = baseType
			// Add pointer for optional numeric types (matches resolveGoType in types.go)
			if !p.Required && m.isNumericType(baseType) {
				field.GoType = "*" + baseType
				field.IsPointer = true
			} else {
				field.GoType = baseType
			}
		}

		fields = append(fields, field)
	}

	return fields
}

// mapQueryPathParams converts parser path params to QueryParamField for query endpoints
func (m *Mapper) mapQueryPathParams(params []parser.Parameter) []QueryParamField {
	fields := make([]QueryParamField, 0, len(params))

	for _, p := range params {
		baseType := m.mapParamType(p.Type)
		field := QueryParamField{
			Name:        strcase.ToCamel(p.Name),
			JSONName:    strcase.ToLowerCamel(p.Name),
			Description: p.Description,
			Required:    p.Required,
			BaseType:    baseType,
		}
		// Add pointer for optional numeric types (matches resolveGoType in types.go)
		if !p.Required && m.isNumericType(baseType) {
			field.GoType = "*" + baseType
			field.IsPointer = true
		} else {
			field.GoType = baseType
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

// isNumericType returns true if the Go type is a numeric type that should be a pointer when optional
func (m *Mapper) isNumericType(goType string) bool {
	switch goType {
	case "int", "int32", "int64", "float32", "float64":
		return true
	default:
		return false
	}
}

// isPrimitiveType returns true if the Go type is a primitive type (not struct/object)
func (m *Mapper) isPrimitiveType(goType string) bool {
	switch goType {
	case "string", "bool", "int", "int32", "int64", "float32", "float64":
		return true
	default:
		return false
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

	// Add execution interval field (optional) - normalized across Query/Action/Resource CRDs
	executionIntervalField := &FieldDefinition{
		Name:        "ExecutionInterval",
		JSONName:    "executionInterval",
		GoType:      "*metav1.Duration",
		Description: "Interval at which to re-execute (e.g., 30s, 5m, 1h). If not set, uses controller default. Set to 0 to disable periodic re-execution.",
		Required:    false,
	}
	spec.Fields = append(spec.Fields, executionIntervalField)

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

		// Check method availability and collect per-method paths
		for _, op := range resource.Operations {
			switch op.Method {
			case "DELETE":
				crd.HasDelete = true
				if crd.DeletePath == "" {
					crd.DeletePath = op.Path
				}
			case "POST":
				crd.HasPost = true
			case "PUT":
				crd.HasPut = true
				if crd.PutPath == "" {
					crd.PutPath = op.Path
				}
			case "PATCH":
				crd.HasPatch = true
			case "GET":
				if crd.GetPath == "" {
					crd.GetPath = op.Path
				}
			}
		}

		// Set UpdateWithPost if configured for this path and neither PUT nor PATCH is available but POST is
		if m.config.ShouldUpdateWithPost(resource.Path) && !crd.HasPut && !crd.HasPatch && crd.HasPost {
			crd.UpdateWithPost = true
		}

		// Find the full resource path with parameters (prefer GET path with params)
		// This is kept for backward compatibility with ResourcePath
		for _, op := range resource.Operations {
			if op.Method == "GET" && strings.Contains(op.Path, "{") {
				crd.ResourcePath = op.Path
				break
			}
		}
		if crd.ResourcePath == "" {
			for _, op := range resource.Operations {
				if op.Method == "PUT" && strings.Contains(op.Path, "{") {
					crd.ResourcePath = op.Path
					break
				}
			}
		}
		// Fallback to BasePath if no parameterized path found
		if crd.ResourcePath == "" {
			crd.ResourcePath = resource.Path
		}

		// ExternalIDRef is only needed when there are no path parameters to identify the resource
		// If path params exist (e.g., /pet/{petId}), those fields serve as the identifier
		crd.NeedsExternalIDRef = !strings.Contains(crd.ResourcePath, "{")

		// Generate spec fields from resource schema
		if resource.Schema != nil {
			crd.Spec = m.schemaToFieldDefinition("Spec", resource.Schema, true)
		} else {
			// Create a generic spec if no schema found
			crd.Spec = m.createGenericSpec()
		}

		// Add path and query parameters from operations to spec fields
		// Use the merge-enabled version to handle ID field merging
		m.addOperationParamsToSpecWithMerge(crd.Spec, resource.Operations, crd, crd.Kind)

		// Generate status fields
		crd.Status = m.createStatusDefinition()

		crds = append(crds, crd)
	}

	return crds, nil
}

// addOperationParamsToSpec adds path and query parameters from operations to the spec.
// It also handles ID field merging: when a path parameter like {orderId} maps to a body field "id",
// the path param is not added as a separate field; instead, the body field is annotated with PathParamName.
func (m *Mapper) addOperationParamsToSpec(spec *FieldDefinition, operations []parser.Operation) {
	m.addOperationParamsToSpecWithMerge(spec, operations, nil, "")
}

// addOperationParamsToSpecWithMerge adds path and query parameters with ID field merging support.
// crd is used to store ID field mappings, and kindName is used for auto-detection heuristics.
func (m *Mapper) addOperationParamsToSpecWithMerge(spec *FieldDefinition, operations []parser.Operation, crd *CRDDefinition, kindName string) {
	if spec == nil {
		return
	}

	// Track existing field names to avoid duplicates
	// Also build a map for quick lookup of field definitions by JSON name
	existingFields := make(map[string]bool)
	fieldByJSONName := make(map[string]*FieldDefinition)
	for _, field := range spec.Fields {
		key := strings.ToLower(field.JSONName)
		existingFields[key] = true
		fieldByJSONName[key] = field
	}

	// Collect unique path params from all operations
	// Sort operations by method to ensure deterministic output (GET first for better descriptions)
	sortedOps := make([]parser.Operation, len(operations))
	copy(sortedOps, operations)
	sort.Slice(sortedOps, func(i, j int) bool {
		methodOrder := map[string]int{"GET": 0, "POST": 1, "PUT": 2, "PATCH": 3, "DELETE": 4}
		return methodOrder[sortedOps[i].Method] < methodOrder[sortedOps[j].Method]
	})

	pathParamsSeen := make(map[string]bool)
	for _, op := range sortedOps {
		for _, param := range op.PathParams {
			paramKey := strings.ToLower(param.Name)
			if pathParamsSeen[paramKey] {
				continue
			}
			pathParamsSeen[paramKey] = true

			// Check if this path param should be merged with an existing body field
			// Priority: 1) x-k8s-id-field extension, 2) --id-field-map flag, 3) auto-detection
			bodyFieldName := m.config.GetIDFieldMapping(param.Name, kindName, param.IDFieldRef)

			if bodyFieldName != "" {
				// Check if the body field exists in the spec
				bodyFieldKey := strings.ToLower(bodyFieldName)
				if bodyField, ok := fieldByJSONName[bodyFieldKey]; ok {
					// Merge: annotate the body field with the path param name
					bodyField.PathParamName = param.Name
					// Store the mapping in the CRD for controller use
					if crd != nil {
						crd.IDFieldMappings = append(crd.IDFieldMappings, IDFieldMapping{
							PathParam: param.Name,
							BodyField: bodyFieldName,
						})
					}
					// Don't add the path param as a separate field
					continue
				}
			}

			// No merge - check if field already exists
			if existingFields[paramKey] {
				continue
			}

			// Add the path param as a new field
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

	// Collect unique query params from all operations (using sorted ops for consistent descriptions)
	queryParamsSeen := make(map[string]bool)
	for _, op := range sortedOps {
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
		// Check if DELETE method is available
		for _, op := range resource.Operations {
			if op.Method == "DELETE" {
				crd.HasDelete = true
				break
			}
		}
		// Check if POST method is available
		for _, op := range resource.Operations {
			if op.Method == "POST" {
				crd.HasPost = true
				break
			}
		}
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

		// Collect path params first so we can use them for action classification
		for _, p := range op.PathParams {
			mapping.PathParams = append(mapping.PathParams, p.Name)
		}
		for _, p := range op.QueryParams {
			mapping.QueryParams = append(mapping.QueryParams, p.Name)
		}

		// Map HTTP method to CRD action
		// POST is only "Create" if it has no path params (e.g., POST /pet)
		// POST with path params (e.g., POST /pet/{petId}) is typically an Update or Action
		switch op.Method {
		case "POST":
			if len(mapping.PathParams) == 0 {
				mapping.CRDAction = "Create"
			} else {
				mapping.CRDAction = "Update" // POST with path params is an update/action
			}
		case "PUT", "PATCH":
			mapping.CRDAction = "Update"
		case "DELETE":
			mapping.CRDAction = "Delete"
		case "GET":
			mapping.CRDAction = "Get"
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

	// Set required if in parent's required list (from OpenAPI spec)
	for _, req := range schema.Required {
		if req == name {
			field.Required = true
			field.OpenAPIRequired = true // Mark as OpenAPI-required for CEL validation
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
			// Check if property is required in OpenAPI spec
			for _, req := range schema.Required {
				if req == propName {
					propField.Required = true
					propField.OpenAPIRequired = true // Mark as OpenAPI-required for CEL validation
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
		// Arrays without item type use RawExtension for arbitrary JSON
		return "[]runtime.RawExtension"
	case "object":
		if len(schema.Properties) == 0 {
			// Objects without properties use RawExtension for arbitrary JSON
			// This is compatible with controller-gen (interface{} is not)
			return "*runtime.RawExtension"
		}
		return "struct"
	default:
		// Unknown types use RawExtension for arbitrary JSON
		return "*runtime.RawExtension"
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

// AggregateDefinition represents a Status Aggregator CRD definition (Option 4)
// This is a read-only CRD that observes and aggregates status from existing resources
type AggregateDefinition struct {
	APIGroup   string
	APIVersion string
	Kind       string
	Plural     string
	// ResourceKinds are the CRUD CRD kinds this aggregate can observe
	ResourceKinds []string
	// QueryKinds are the Query CRD kinds this aggregate can observe
	QueryKinds []string
	// ActionKinds are the Action CRD kinds this aggregate can observe
	ActionKinds []string
	// AllKinds is the combined list of all kinds (for iteration convenience)
	AllKinds []string
}

// ResourceSelector defines how to select resources to aggregate
type ResourceSelector struct {
	Group       string            // API group (e.g., "petstore.example.com")
	Kind        string            // Resource kind (e.g., "Pet")
	NamePattern string            // Regex pattern for resource names
	MatchLabels map[string]string // Label selector
}

// AggregationStrategy defines how to combine resource statuses
type AggregationStrategy string

const (
	// AllHealthy requires all resources to be healthy
	AllHealthy AggregationStrategy = "AllHealthy"
	// AnyHealthy requires at least one resource to be healthy
	AnyHealthy AggregationStrategy = "AnyHealthy"
	// Quorum requires a majority of resources to be healthy
	Quorum AggregationStrategy = "Quorum"
)

// MatchType defines how matchers are combined
type MatchType string

const (
	// AnyResourceMatchesAnyCondition - any resource matches any condition
	AnyResourceMatchesAnyCondition MatchType = "AnyResourceMatchesAnyCondition"
	// AnyResourceMatchesAllConditions - any resource matches all conditions
	AnyResourceMatchesAllConditions MatchType = "AnyResourceMatchesAllConditions"
	// AllResourcesMatchAnyCondition - all resources match any condition
	AllResourcesMatchAnyCondition MatchType = "AllResourcesMatchAnyCondition"
	// AllResourcesMatchAllConditions - all resources match all conditions (default)
	AllResourcesMatchAllConditions MatchType = "AllResourcesMatchAllConditions"
)

// CreateAggregateDefinition creates an aggregate CRD definition from existing CRDs
func (m *Mapper) CreateAggregateDefinition(crds []*CRDDefinition) *AggregateDefinition {
	// Collect resource kinds by type
	resourceKinds := make([]string, 0)
	queryKinds := make([]string, 0)
	actionKinds := make([]string, 0)
	allKinds := make([]string, 0)

	for _, crd := range crds {
		allKinds = append(allKinds, crd.Kind)
		if crd.IsQuery {
			queryKinds = append(queryKinds, crd.Kind)
		} else if crd.IsAction {
			actionKinds = append(actionKinds, crd.Kind)
		} else {
			resourceKinds = append(resourceKinds, crd.Kind)
		}
	}

	// Derive aggregate kind name from API group
	// e.g., "petstore.example.com" -> "PetstoreAggregate"
	appName := strings.Split(m.config.APIGroup, ".")[0]
	aggregateKind := strcase.ToCamel(appName) + "Aggregate"

	return &AggregateDefinition{
		APIGroup:      m.config.APIGroup,
		APIVersion:    m.config.APIVersion,
		Kind:          aggregateKind,
		Plural:        pluralize(aggregateKind),
		ResourceKinds: resourceKinds,
		QueryKinds:    queryKinds,
		ActionKinds:   actionKinds,
		AllKinds:      allKinds,
	}
}

// BundleDefinition represents an Inline Composition CRD definition (Option 2)
// This CRD creates and manages multiple child resources with dependency ordering
type BundleDefinition struct {
	APIGroup   string
	APIVersion string
	Kind       string
	Plural     string
	// ResourceKinds are the CRUD CRD kinds this bundle can create
	ResourceKinds []string
	// QueryKinds are the Query CRD kinds this bundle can create
	QueryKinds []string
	// ActionKinds are the Action CRD kinds this bundle can create
	ActionKinds []string
	// AllKinds is the combined list of all kinds (for iteration convenience)
	AllKinds []string
}

// CreateBundleDefinition creates a bundle CRD definition from existing CRDs
func (m *Mapper) CreateBundleDefinition(crds []*CRDDefinition) *BundleDefinition {
	// Collect resource kinds by type
	resourceKinds := make([]string, 0)
	queryKinds := make([]string, 0)
	actionKinds := make([]string, 0)
	allKinds := make([]string, 0)

	for _, crd := range crds {
		allKinds = append(allKinds, crd.Kind)
		if crd.IsQuery {
			queryKinds = append(queryKinds, crd.Kind)
		} else if crd.IsAction {
			actionKinds = append(actionKinds, crd.Kind)
		} else {
			resourceKinds = append(resourceKinds, crd.Kind)
		}
	}

	// Derive bundle kind name from API group
	// e.g., "petstore.example.com" -> "PetstoreBundle"
	appName := strings.Split(m.config.APIGroup, ".")[0]
	bundleKind := strcase.ToCamel(appName) + "Bundle"

	return &BundleDefinition{
		APIGroup:      m.config.APIGroup,
		APIVersion:    m.config.APIVersion,
		Kind:          bundleKind,
		Plural:        pluralize(bundleKind),
		ResourceKinds: resourceKinds,
		QueryKinds:    queryKinds,
		ActionKinds:   actionKinds,
		AllKinds:      allKinds,
	}
}
