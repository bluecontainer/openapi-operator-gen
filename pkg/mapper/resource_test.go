package mapper

import (
	"testing"

	"github.com/bluecontainer/openapi-operator-gen/internal/config"
	"github.com/bluecontainer/openapi-operator-gen/pkg/parser"
)

func TestNewMapper(t *testing.T) {
	cfg := &config.Config{
		APIGroup:   "test.example.com",
		APIVersion: "v1",
	}
	m := NewMapper(cfg)
	if m == nil {
		t.Fatal("expected non-nil mapper")
	}
	if m.config != cfg {
		t.Error("expected config to be set")
	}
}

// =============================================================================
// mapType Tests
// =============================================================================

func TestMapType_String(t *testing.T) {
	m := &Mapper{config: &config.Config{}}

	tests := []struct {
		name     string
		schema   *parser.Schema
		expected string
	}{
		{
			name:     "plain string",
			schema:   &parser.Schema{Type: "string"},
			expected: "string",
		},
		{
			name:     "date-time format",
			schema:   &parser.Schema{Type: "string", Format: "date-time"},
			expected: "metav1.Time",
		},
		{
			name:     "byte format",
			schema:   &parser.Schema{Type: "string", Format: "byte"},
			expected: "[]byte",
		},
		{
			name:     "unknown string format",
			schema:   &parser.Schema{Type: "string", Format: "email"},
			expected: "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.mapType(tt.schema)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMapType_Integer(t *testing.T) {
	m := &Mapper{config: &config.Config{}}

	tests := []struct {
		name     string
		schema   *parser.Schema
		expected string
	}{
		{
			name:     "plain integer",
			schema:   &parser.Schema{Type: "integer"},
			expected: "int",
		},
		{
			name:     "int32 format",
			schema:   &parser.Schema{Type: "integer", Format: "int32"},
			expected: "int32",
		},
		{
			name:     "int64 format",
			schema:   &parser.Schema{Type: "integer", Format: "int64"},
			expected: "int64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.mapType(tt.schema)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMapType_Number(t *testing.T) {
	m := &Mapper{config: &config.Config{}}

	tests := []struct {
		name     string
		schema   *parser.Schema
		expected string
	}{
		{
			name:     "plain number",
			schema:   &parser.Schema{Type: "number"},
			expected: "float64",
		},
		{
			name:     "float format",
			schema:   &parser.Schema{Type: "number", Format: "float"},
			expected: "float32",
		},
		{
			name:     "double format",
			schema:   &parser.Schema{Type: "number", Format: "double"},
			expected: "float64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.mapType(tt.schema)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMapType_Boolean(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	schema := &parser.Schema{Type: "boolean"}
	result := m.mapType(schema)
	if result != "bool" {
		t.Errorf("expected 'bool', got %q", result)
	}
}

func TestMapType_Array(t *testing.T) {
	m := &Mapper{config: &config.Config{}}

	tests := []struct {
		name     string
		schema   *parser.Schema
		expected string
	}{
		{
			name: "array of strings",
			schema: &parser.Schema{
				Type:  "array",
				Items: &parser.Schema{Type: "string"},
			},
			expected: "[]string",
		},
		{
			name: "array of integers",
			schema: &parser.Schema{
				Type:  "array",
				Items: &parser.Schema{Type: "integer", Format: "int64"},
			},
			expected: "[]int64",
		},
		{
			name:     "array without items",
			schema:   &parser.Schema{Type: "array"},
			expected: "[]interface{}",
		},
		{
			name: "nested array",
			schema: &parser.Schema{
				Type: "array",
				Items: &parser.Schema{
					Type:  "array",
					Items: &parser.Schema{Type: "string"},
				},
			},
			expected: "[][]string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.mapType(tt.schema)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMapType_Object(t *testing.T) {
	m := &Mapper{config: &config.Config{}}

	tests := []struct {
		name     string
		schema   *parser.Schema
		expected string
	}{
		{
			name:     "object without properties",
			schema:   &parser.Schema{Type: "object"},
			expected: "map[string]interface{}",
		},
		{
			name: "object with properties",
			schema: &parser.Schema{
				Type: "object",
				Properties: map[string]*parser.Schema{
					"name": {Type: "string"},
				},
			},
			expected: "struct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.mapType(tt.schema)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMapType_Unknown(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	schema := &parser.Schema{Type: "unknown"}
	result := m.mapType(schema)
	if result != "interface{}" {
		t.Errorf("expected 'interface{}', got %q", result)
	}
}

func TestMapType_EmptyType(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	schema := &parser.Schema{}
	result := m.mapType(schema)
	if result != "interface{}" {
		t.Errorf("expected 'interface{}', got %q", result)
	}
}

// =============================================================================
// mapOperations Tests
// =============================================================================

func TestMapOperations_HTTPMethods(t *testing.T) {
	m := &Mapper{config: &config.Config{}}

	tests := []struct {
		method         string
		expectedAction string
	}{
		{"POST", "Create"},
		{"PUT", "Update"},
		{"PATCH", "Update"},
		{"DELETE", "Delete"},
		{"GET", "Get"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			ops := []parser.Operation{{Method: tt.method, Path: "/test"}}
			result := m.mapOperations(ops)
			if len(result) != 1 {
				t.Fatalf("expected 1 mapping, got %d", len(result))
			}
			if result[0].CRDAction != tt.expectedAction {
				t.Errorf("expected action %q, got %q", tt.expectedAction, result[0].CRDAction)
			}
			if result[0].HTTPMethod != tt.method {
				t.Errorf("expected method %q, got %q", tt.method, result[0].HTTPMethod)
			}
		})
	}
}

func TestMapOperations_Parameters(t *testing.T) {
	m := &Mapper{config: &config.Config{}}

	ops := []parser.Operation{
		{
			Method: "GET",
			Path:   "/users/{id}",
			PathParams: []parser.Parameter{
				{Name: "id", In: "path"},
			},
			QueryParams: []parser.Parameter{
				{Name: "limit", In: "query"},
				{Name: "offset", In: "query"},
			},
		},
	}

	result := m.mapOperations(ops)
	if len(result) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(result))
	}

	if len(result[0].PathParams) != 1 {
		t.Errorf("expected 1 path param, got %d", len(result[0].PathParams))
	}
	if result[0].PathParams[0] != "id" {
		t.Errorf("expected path param 'id', got %q", result[0].PathParams[0])
	}

	if len(result[0].QueryParams) != 2 {
		t.Errorf("expected 2 query params, got %d", len(result[0].QueryParams))
	}
}

func TestMapOperations_EmptyList(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	result := m.mapOperations(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestMapOperations_UnknownMethod(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	ops := []parser.Operation{{Method: "OPTIONS", Path: "/test"}}
	result := m.mapOperations(ops)
	if len(result) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(result))
	}
	if result[0].CRDAction != "" {
		t.Errorf("expected empty action for unknown method, got %q", result[0].CRDAction)
	}
}

// =============================================================================
// schemaToFieldDefinition Tests
// =============================================================================

func TestSchemaToFieldDefinition_NilSchema(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	result := m.schemaToFieldDefinition("test", nil, false)
	if result != nil {
		t.Error("expected nil result for nil schema")
	}
}

func TestSchemaToFieldDefinition_NameConversion(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	schema := &parser.Schema{Type: "string"}

	tests := []struct {
		input        string
		expectedName string
		expectedJSON string
	}{
		{"userName", "UserName", "userName"},
		{"user_name", "UserName", "userName"},
		{"id", "Id", "id"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := m.schemaToFieldDefinition(tt.input, schema, false)
			if result.Name != tt.expectedName {
				t.Errorf("expected Name %q, got %q", tt.expectedName, result.Name)
			}
			if result.JSONName != tt.expectedJSON {
				t.Errorf("expected JSONName %q, got %q", tt.expectedJSON, result.JSONName)
			}
		})
	}
}

func TestSchemaToFieldDefinition_Description(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	schema := &parser.Schema{
		Type:        "string",
		Description: "A test field",
	}
	result := m.schemaToFieldDefinition("test", schema, false)
	if result.Description != "A test field" {
		t.Errorf("expected description 'A test field', got %q", result.Description)
	}
}

func TestSchemaToFieldDefinition_ValidationRules(t *testing.T) {
	m := &Mapper{config: &config.Config{}}

	minLen := int64(1)
	maxLen := int64(100)
	min := float64(0)
	max := float64(1000)
	minItems := int64(1)
	maxItems := int64(10)

	schema := &parser.Schema{
		Type:      "string",
		MinLength: &minLen,
		MaxLength: &maxLen,
		Minimum:   &min,
		Maximum:   &max,
		Pattern:   "^[a-z]+$",
		MinItems:  &minItems,
		MaxItems:  &maxItems,
	}

	result := m.schemaToFieldDefinition("test", schema, false)

	if result.Validation == nil {
		t.Fatal("expected validation rules to be set")
	}
	if *result.Validation.MinLength != 1 {
		t.Errorf("expected MinLength 1, got %d", *result.Validation.MinLength)
	}
	if *result.Validation.MaxLength != 100 {
		t.Errorf("expected MaxLength 100, got %d", *result.Validation.MaxLength)
	}
	if *result.Validation.Minimum != 0 {
		t.Errorf("expected Minimum 0, got %f", *result.Validation.Minimum)
	}
	if *result.Validation.Maximum != 1000 {
		t.Errorf("expected Maximum 1000, got %f", *result.Validation.Maximum)
	}
	if result.Validation.Pattern != "^[a-z]+$" {
		t.Errorf("expected Pattern '^[a-z]+$', got %q", result.Validation.Pattern)
	}
	if *result.Validation.MinItems != 1 {
		t.Errorf("expected MinItems 1, got %d", *result.Validation.MinItems)
	}
	if *result.Validation.MaxItems != 10 {
		t.Errorf("expected MaxItems 10, got %d", *result.Validation.MaxItems)
	}
}

func TestSchemaToFieldDefinition_NoValidation(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	schema := &parser.Schema{Type: "string"}
	result := m.schemaToFieldDefinition("test", schema, false)
	if result.Validation != nil {
		t.Error("expected no validation rules for plain schema")
	}
}

func TestSchemaToFieldDefinition_EnumInValidation(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	schema := &parser.Schema{
		Type: "string",
		Enum: []interface{}{"pending", "active", "completed", 123}, // 123 should be ignored
	}

	result := m.schemaToFieldDefinition("status", schema, false)

	if result.Validation == nil {
		t.Fatal("expected validation rules for enum")
	}
	if len(result.Validation.Enum) != 3 {
		t.Errorf("expected 3 enum values (strings only), got %d", len(result.Validation.Enum))
	}
}

func TestSchemaToFieldDefinition_FieldEnum(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	schema := &parser.Schema{
		Type: "string",
		Enum: []interface{}{"red", "green", "blue"},
	}

	result := m.schemaToFieldDefinition("color", schema, false)

	if len(result.Enum) != 3 {
		t.Errorf("expected 3 enum values, got %d", len(result.Enum))
	}
	expected := []string{"red", "green", "blue"}
	for i, v := range expected {
		if result.Enum[i] != v {
			t.Errorf("expected enum[%d] to be %q, got %q", i, v, result.Enum[i])
		}
	}
}

func TestSchemaToFieldDefinition_NestedProperties(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	schema := &parser.Schema{
		Type: "object",
		Properties: map[string]*parser.Schema{
			"name":  {Type: "string"},
			"age":   {Type: "integer"},
			"email": {Type: "string"},
		},
		Required: []string{"name", "email"},
	}

	result := m.schemaToFieldDefinition("user", schema, false)

	if len(result.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(result.Fields))
	}

	// Fields should be sorted alphabetically
	expectedOrder := []string{"Age", "Email", "Name"}
	for i, name := range expectedOrder {
		if result.Fields[i].Name != name {
			t.Errorf("expected field[%d] to be %q, got %q", i, name, result.Fields[i].Name)
		}
	}

	// Check required fields
	for _, f := range result.Fields {
		if f.Name == "Name" || f.Name == "Email" {
			if !f.Required {
				t.Errorf("expected %s to be required", f.Name)
			}
		}
		if f.Name == "Age" {
			if f.Required {
				t.Errorf("expected Age to not be required")
			}
		}
	}
}

func TestSchemaToFieldDefinition_ArrayItems(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	schema := &parser.Schema{
		Type: "array",
		Items: &parser.Schema{
			Type: "object",
			Properties: map[string]*parser.Schema{
				"id": {Type: "integer"},
			},
		},
	}

	result := m.schemaToFieldDefinition("items", schema, false)

	if result.ItemType == nil {
		t.Fatal("expected ItemType to be set for array")
	}
	if result.ItemType.GoType != "struct" {
		t.Errorf("expected ItemType.GoType 'struct', got %q", result.ItemType.GoType)
	}
}

// =============================================================================
// generateShortNames Tests
// =============================================================================

func TestGenerateShortNames(t *testing.T) {
	m := &Mapper{config: &config.Config{}}

	tests := []struct {
		kind     string
		expected []string
	}{
		{"Pet", []string{"pe", "pet"}},
		{"User", []string{"us", "use"}},
		{"DatabaseConnection", []string{"da", "dat"}},
		{"AB", []string{"ab"}},
		{"A", []string{}},
		{"", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			result := m.generateShortNames(tt.kind)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d short names, got %d", len(tt.expected), len(result))
				return
			}
			for i, v := range tt.expected {
				if result[i] != v {
					t.Errorf("expected shortName[%d] to be %q, got %q", i, v, result[i])
				}
			}
		})
	}
}

// =============================================================================
// createGenericSpec Tests
// =============================================================================

func TestCreateGenericSpec(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	result := m.createGenericSpec()

	if result.Name != "Spec" {
		t.Errorf("expected Name 'Spec', got %q", result.Name)
	}
	if result.JSONName != "spec" {
		t.Errorf("expected JSONName 'spec', got %q", result.JSONName)
	}
	if result.GoType != "struct" {
		t.Errorf("expected GoType 'struct', got %q", result.GoType)
	}
	if len(result.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(result.Fields))
	}
	if result.Fields[0].Name != "Data" {
		t.Errorf("expected field name 'Data', got %q", result.Fields[0].Name)
	}
	if result.Fields[0].GoType != "runtime.RawExtension" {
		t.Errorf("expected GoType 'runtime.RawExtension', got %q", result.Fields[0].GoType)
	}
}

// =============================================================================
// createStatusDefinition Tests
// =============================================================================

func TestCreateStatusDefinition(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	result := m.createStatusDefinition()

	if result.Name != "Status" {
		t.Errorf("expected Name 'Status', got %q", result.Name)
	}
	if result.JSONName != "status" {
		t.Errorf("expected JSONName 'status', got %q", result.JSONName)
	}

	expectedFields := map[string]string{
		"State":              "string",
		"LastSyncTime":       "metav1.Time",
		"ExternalID":         "string",
		"Message":            "string",
		"Conditions":         "[]metav1.Condition",
		"ObservedGeneration": "int64",
		"Response":           "runtime.RawExtension",
	}

	if len(result.Fields) != len(expectedFields) {
		t.Fatalf("expected %d fields, got %d", len(expectedFields), len(result.Fields))
	}

	for _, f := range result.Fields {
		expected, ok := expectedFields[f.Name]
		if !ok {
			t.Errorf("unexpected field %q", f.Name)
			continue
		}
		if f.GoType != expected {
			t.Errorf("expected %s.GoType to be %q, got %q", f.Name, expected, f.GoType)
		}
	}
}

// =============================================================================
// MapResources Integration Tests
// =============================================================================

func TestMapResources_PerResourceMode(t *testing.T) {
	cfg := &config.Config{
		APIGroup:    "test.example.com",
		APIVersion:  "v1alpha1",
		MappingMode: config.PerResource,
	}
	m := NewMapper(cfg)

	spec := &parser.ParsedSpec{
		Resources: []*parser.Resource{
			{
				Name:       "User",
				PluralName: "Users",
				Path:       "/users",
				Schema: &parser.Schema{
					Type: "object",
					Properties: map[string]*parser.Schema{
						"name":  {Type: "string"},
						"email": {Type: "string"},
					},
				},
				Operations: []parser.Operation{
					{Method: "GET", Path: "/users"},
					{Method: "POST", Path: "/users"},
				},
			},
			{
				Name:       "Pet",
				PluralName: "Pets",
				Path:       "/pets",
				Operations: []parser.Operation{
					{Method: "GET", Path: "/pets"},
				},
			},
		},
	}

	crds, err := m.MapResources(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(crds) != 2 {
		t.Fatalf("expected 2 CRDs, got %d", len(crds))
	}

	// Check User CRD
	var userCRD *CRDDefinition
	for _, crd := range crds {
		if crd.Kind == "User" {
			userCRD = crd
			break
		}
	}
	if userCRD == nil {
		t.Fatal("User CRD not found")
	}

	if userCRD.APIGroup != "test.example.com" {
		t.Errorf("expected APIGroup 'test.example.com', got %q", userCRD.APIGroup)
	}
	if userCRD.APIVersion != "v1alpha1" {
		t.Errorf("expected APIVersion 'v1alpha1', got %q", userCRD.APIVersion)
	}
	if userCRD.Plural != "users" {
		t.Errorf("expected Plural 'users', got %q", userCRD.Plural)
	}
	if userCRD.Scope != "Namespaced" {
		t.Errorf("expected Scope 'Namespaced', got %q", userCRD.Scope)
	}
	if len(userCRD.Operations) != 2 {
		t.Errorf("expected 2 operations, got %d", len(userCRD.Operations))
	}
	if userCRD.Spec == nil {
		t.Error("expected Spec to be set")
	}
	if userCRD.Status == nil {
		t.Error("expected Status to be set")
	}
}

func TestMapResources_PerResourceMode_NoSchema(t *testing.T) {
	cfg := &config.Config{
		APIGroup:    "test.example.com",
		APIVersion:  "v1",
		MappingMode: config.PerResource,
	}
	m := NewMapper(cfg)

	spec := &parser.ParsedSpec{
		Resources: []*parser.Resource{
			{
				Name:       "Widget",
				PluralName: "Widgets",
				Path:       "/widgets",
				Schema:     nil, // No schema
			},
		},
	}

	crds, err := m.MapResources(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(crds) != 1 {
		t.Fatalf("expected 1 CRD, got %d", len(crds))
	}

	// Should fall back to generic spec
	if crds[0].Spec == nil {
		t.Fatal("expected Spec to be set")
	}
	if len(crds[0].Spec.Fields) != 1 {
		t.Errorf("expected generic spec with 1 field, got %d", len(crds[0].Spec.Fields))
	}
	if crds[0].Spec.Fields[0].Name != "Data" {
		t.Errorf("expected generic spec field 'Data', got %q", crds[0].Spec.Fields[0].Name)
	}
}

func TestMapResources_SingleCRDMode(t *testing.T) {
	cfg := &config.Config{
		APIGroup:    "api.example.com",
		APIVersion:  "v1beta1",
		MappingMode: config.SingleCRD,
	}
	m := NewMapper(cfg)

	spec := &parser.ParsedSpec{
		Description: "Test API",
		Resources: []*parser.Resource{
			{
				Name:       "User",
				PluralName: "Users",
				Path:       "/users",
				Operations: []parser.Operation{
					{Method: "GET", Path: "/users"},
					{Method: "POST", Path: "/users"},
				},
			},
			{
				Name:       "Pet",
				PluralName: "Pets",
				Path:       "/pets",
				Operations: []parser.Operation{
					{Method: "GET", Path: "/pets"},
				},
			},
		},
	}

	crds, err := m.MapResources(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(crds) != 1 {
		t.Fatalf("expected 1 CRD in single-crd mode, got %d", len(crds))
	}

	crd := crds[0]
	if crd.Kind != "APIResource" {
		t.Errorf("expected Kind 'APIResource', got %q", crd.Kind)
	}
	if crd.Plural != "apiresources" {
		t.Errorf("expected Plural 'apiresources', got %q", crd.Plural)
	}
	if len(crd.ShortNames) != 1 || crd.ShortNames[0] != "apir" {
		t.Errorf("expected ShortNames ['apir'], got %v", crd.ShortNames)
	}
	if crd.Description != "Test API" {
		t.Errorf("expected Description 'Test API', got %q", crd.Description)
	}

	// Should have all operations from both resources
	if len(crd.Operations) != 3 {
		t.Errorf("expected 3 operations total, got %d", len(crd.Operations))
	}

	// Check spec fields
	if crd.Spec == nil {
		t.Fatal("expected Spec to be set")
	}
	if len(crd.Spec.Fields) != 4 {
		t.Errorf("expected 4 spec fields, got %d", len(crd.Spec.Fields))
	}

	// Check ResourceType enum (should be sorted)
	var resourceTypeField *FieldDefinition
	for _, f := range crd.Spec.Fields {
		if f.Name == "ResourceType" {
			resourceTypeField = f
			break
		}
	}
	if resourceTypeField == nil {
		t.Fatal("ResourceType field not found")
	}
	if len(resourceTypeField.Enum) != 2 {
		t.Errorf("expected 2 resource types in enum, got %d", len(resourceTypeField.Enum))
	}
	// Should be sorted: Pet, User
	if resourceTypeField.Enum[0] != "Pet" || resourceTypeField.Enum[1] != "User" {
		t.Errorf("expected sorted enum [Pet, User], got %v", resourceTypeField.Enum)
	}
}

func TestMapResources_DefaultToPerResource(t *testing.T) {
	cfg := &config.Config{
		APIGroup:    "test.example.com",
		APIVersion:  "v1",
		MappingMode: "", // Empty should default to per-resource
	}
	m := NewMapper(cfg)

	spec := &parser.ParsedSpec{
		Resources: []*parser.Resource{
			{Name: "Foo", PluralName: "Foos", Path: "/foo"},
			{Name: "Bar", PluralName: "Bars", Path: "/bar"},
		},
	}

	crds, err := m.MapResources(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should create 2 CRDs (per-resource mode)
	if len(crds) != 2 {
		t.Errorf("expected 2 CRDs with default mode, got %d", len(crds))
	}
}

func TestMapResources_EmptySpec(t *testing.T) {
	cfg := &config.Config{
		APIGroup:    "test.example.com",
		APIVersion:  "v1",
		MappingMode: config.PerResource,
	}
	m := NewMapper(cfg)

	spec := &parser.ParsedSpec{
		Resources: []*parser.Resource{},
	}

	crds, err := m.MapResources(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(crds) != 0 {
		t.Errorf("expected 0 CRDs for empty spec, got %d", len(crds))
	}
}

// =============================================================================
// Edge Cases and Complex Scenarios
// =============================================================================

func TestSchemaToFieldDefinition_DeeplyNestedObjects(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	schema := &parser.Schema{
		Type: "object",
		Properties: map[string]*parser.Schema{
			"level1": {
				Type: "object",
				Properties: map[string]*parser.Schema{
					"level2": {
						Type: "object",
						Properties: map[string]*parser.Schema{
							"value": {Type: "string"},
						},
					},
				},
			},
		},
	}

	result := m.schemaToFieldDefinition("root", schema, false)

	if len(result.Fields) != 1 {
		t.Fatalf("expected 1 field at root, got %d", len(result.Fields))
	}

	level1 := result.Fields[0]
	if level1.Name != "Level1" {
		t.Errorf("expected field name 'Level1', got %q", level1.Name)
	}
	if len(level1.Fields) != 1 {
		t.Fatalf("expected 1 field at level1, got %d", len(level1.Fields))
	}

	level2 := level1.Fields[0]
	if level2.Name != "Level2" {
		t.Errorf("expected field name 'Level2', got %q", level2.Name)
	}
	if len(level2.Fields) != 1 {
		t.Fatalf("expected 1 field at level2, got %d", len(level2.Fields))
	}

	value := level2.Fields[0]
	if value.Name != "Value" {
		t.Errorf("expected field name 'Value', got %q", value.Name)
	}
	if value.GoType != "string" {
		t.Errorf("expected GoType 'string', got %q", value.GoType)
	}
}

func TestSchemaToFieldDefinition_ArrayOfObjects(t *testing.T) {
	m := &Mapper{config: &config.Config{}}
	schema := &parser.Schema{
		Type: "array",
		Items: &parser.Schema{
			Type: "object",
			Properties: map[string]*parser.Schema{
				"id":   {Type: "integer"},
				"name": {Type: "string"},
			},
		},
	}

	result := m.schemaToFieldDefinition("items", schema, false)

	if result.GoType != "[]struct" {
		t.Errorf("expected GoType '[]struct', got %q", result.GoType)
	}
	if result.ItemType == nil {
		t.Fatal("expected ItemType to be set")
	}
	if len(result.ItemType.Fields) != 2 {
		t.Errorf("expected 2 fields in item type, got %d", len(result.ItemType.Fields))
	}
}

func TestMapOperations_MultipleOperationsSamePath(t *testing.T) {
	m := &Mapper{config: &config.Config{}}

	ops := []parser.Operation{
		{Method: "GET", Path: "/users/{id}"},
		{Method: "PUT", Path: "/users/{id}"},
		{Method: "DELETE", Path: "/users/{id}"},
	}

	result := m.mapOperations(ops)

	if len(result) != 3 {
		t.Fatalf("expected 3 mappings, got %d", len(result))
	}

	actions := make(map[string]bool)
	for _, r := range result {
		actions[r.CRDAction] = true
	}

	if !actions["Get"] || !actions["Update"] || !actions["Delete"] {
		t.Errorf("expected Get, Update, Delete actions, got %v", actions)
	}
}
