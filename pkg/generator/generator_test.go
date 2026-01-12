package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluecontainer/openapi-operator-gen/internal/config"
	"github.com/bluecontainer/openapi-operator-gen/pkg/mapper"
)

// =============================================================================
// CRDGenerator Tests
// =============================================================================

func TestNewCRDGenerator(t *testing.T) {
	cfg := &config.Config{
		OutputDir:  "/tmp/test",
		APIGroup:   "test.example.com",
		APIVersion: "v1",
	}
	g := NewCRDGenerator(cfg)
	if g == nil {
		t.Fatal("expected non-nil generator")
	}
	if g.config != cfg {
		t.Error("expected config to be set")
	}
}

func TestCRDGenerator_MapToSchemaType(t *testing.T) {
	g := &CRDGenerator{config: &config.Config{}}

	tests := []struct {
		goType   string
		expected string
	}{
		{"string", "string"},
		{"*string", "string"},
		{"int", "integer"},
		{"int32", "integer"},
		{"int64", "integer"},
		{"*int64", "integer"},
		{"float32", "number"},
		{"float64", "number"},
		{"*float64", "number"},
		{"bool", "boolean"},
		{"*bool", "boolean"},
		{"runtime.RawExtension", "object"},
		{"metav1.Time", "string"},
		{"[]string", "array"},
		{"[]int", "array"},
		{"[]*SomeType", "array"},
		{"map[string]string", "object"},
		{"map[string]interface{}", "object"},
		{"CustomType", "object"},
		{"struct", "object"},
	}

	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			result := g.mapToSchemaType(tt.goType)
			if result != tt.expected {
				t.Errorf("mapToSchemaType(%q) = %q, expected %q", tt.goType, result, tt.expected)
			}
		})
	}
}

func TestCRDGenerator_ConvertFields(t *testing.T) {
	g := &CRDGenerator{config: &config.Config{}}

	fields := []*mapper.FieldDefinition{
		{
			Name:        "Name",
			JSONName:    "name",
			GoType:      "string",
			Description: "The name of the resource",
			Required:    true,
		},
		{
			Name:        "Count",
			JSONName:    "count",
			GoType:      "int64",
			Description: "Count of items",
			Required:    false,
		},
		{
			Name:     "Status",
			JSONName: "status",
			GoType:   "string",
			Enum:     []string{"active", "inactive"},
		},
	}

	result := g.convertFields(fields)

	if len(result) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(result))
	}

	// Check first field
	if result[0].JSONName != "name" {
		t.Errorf("expected JSONName 'name', got %q", result[0].JSONName)
	}
	if result[0].SchemaType != "string" {
		t.Errorf("expected SchemaType 'string', got %q", result[0].SchemaType)
	}
	if result[0].Description != "The name of the resource" {
		t.Errorf("expected Description, got %q", result[0].Description)
	}
	if !result[0].Required {
		t.Error("expected Required to be true")
	}

	// Check second field
	if result[1].SchemaType != "integer" {
		t.Errorf("expected SchemaType 'integer', got %q", result[1].SchemaType)
	}

	// Check third field with enum
	if len(result[2].Enum) != 2 {
		t.Errorf("expected 2 enum values, got %d", len(result[2].Enum))
	}
}

func TestCRDGenerator_Generate(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "test.example.com",
		APIVersion: "v1alpha1",
	}
	g := NewCRDGenerator(cfg)

	crds := []*mapper.CRDDefinition{
		{
			APIGroup:   "test.example.com",
			APIVersion: "v1alpha1",
			Kind:       "Widget",
			Plural:     "widgets",
			ShortNames: []string{"wg"},
			Scope:      "Namespaced",
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{
						Name:     "Name",
						JSONName: "name",
						GoType:   "string",
						Required: true,
					},
				},
			},
		},
	}

	err := g.Generate(crds)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check that file was created
	expectedPath := filepath.Join(tmpDir, "config", "crd", "bases", "test.example.com_widgets.yaml")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", expectedPath)
	}

	// Read and verify content
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "kind: CustomResourceDefinition") {
		t.Error("expected CRD kind in content")
	}
	if !strings.Contains(contentStr, "test.example.com") {
		t.Error("expected API group in content")
	}
	if !strings.Contains(contentStr, "widgets") {
		t.Error("expected plural name in content")
	}
}

func TestCRDGenerator_Generate_MultipleCRDs(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "test.example.com",
		APIVersion: "v1",
	}
	g := NewCRDGenerator(cfg)

	crds := []*mapper.CRDDefinition{
		{
			APIGroup:   "test.example.com",
			APIVersion: "v1",
			Kind:       "User",
			Plural:     "users",
			Scope:      "Namespaced",
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{Name: "Name", JSONName: "name", GoType: "string"},
				},
			},
		},
		{
			APIGroup:   "test.example.com",
			APIVersion: "v1",
			Kind:       "Pet",
			Plural:     "pets",
			Scope:      "Namespaced",
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{Name: "Name", JSONName: "name", GoType: "string"},
				},
			},
		},
	}

	err := g.Generate(crds)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check both files were created
	for _, plural := range []string{"users", "pets"} {
		expectedPath := filepath.Join(tmpDir, "config", "crd", "bases", "test.example.com_"+plural+".yaml")
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", expectedPath)
		}
	}
}

func TestCRDGenerator_Generate_EmptyCRDs(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "test.example.com",
		APIVersion: "v1",
	}
	g := NewCRDGenerator(cfg)

	err := g.Generate([]*mapper.CRDDefinition{})
	if err != nil {
		t.Fatalf("Generate failed for empty CRDs: %v", err)
	}

	// Directory should still be created
	dirPath := filepath.Join(tmpDir, "config", "crd", "bases")
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}

// =============================================================================
// TypesGenerator Tests
// =============================================================================

func TestNewTypesGenerator(t *testing.T) {
	cfg := &config.Config{
		OutputDir:  "/tmp/test",
		APIGroup:   "test.example.com",
		APIVersion: "v1",
	}
	g := NewTypesGenerator(cfg)
	if g == nil {
		t.Fatal("expected non-nil generator")
	}
	if g.config != cfg {
		t.Error("expected config to be set")
	}
}

func TestTypesGenerator_ResolveGoType(t *testing.T) {
	g := &TypesGenerator{config: &config.Config{}}

	tests := []struct {
		name     string
		field    *mapper.FieldDefinition
		expected string
	}{
		{
			name:     "runtime.RawExtension",
			field:    &mapper.FieldDefinition{GoType: "runtime.RawExtension"},
			expected: "*runtime.RawExtension",
		},
		{
			name:     "metav1.Time",
			field:    &mapper.FieldDefinition{GoType: "metav1.Time"},
			expected: "*metav1.Time",
		},
		{
			name:     "[]metav1.Condition",
			field:    &mapper.FieldDefinition{GoType: "[]metav1.Condition"},
			expected: "[]metav1.Condition",
		},
		{
			name:     "required string",
			field:    &mapper.FieldDefinition{GoType: "string", Required: true},
			expected: "string",
		},
		{
			name:     "optional string",
			field:    &mapper.FieldDefinition{GoType: "string", Required: false},
			expected: "string",
		},
		{
			name:     "required int64",
			field:    &mapper.FieldDefinition{GoType: "int64", Required: true},
			expected: "int64",
		},
		{
			name:     "optional int64",
			field:    &mapper.FieldDefinition{GoType: "int64", Required: false},
			expected: "*int64",
		},
		{
			name:     "optional int32",
			field:    &mapper.FieldDefinition{GoType: "int32", Required: false},
			expected: "*int32",
		},
		{
			name:     "optional float64",
			field:    &mapper.FieldDefinition{GoType: "float64", Required: false},
			expected: "*float64",
		},
		{
			name:     "required bool",
			field:    &mapper.FieldDefinition{GoType: "bool", Required: true},
			expected: "bool",
		},
		{
			name:     "optional bool",
			field:    &mapper.FieldDefinition{GoType: "bool", Required: false},
			expected: "bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.resolveGoType(tt.field)
			if result != tt.expected {
				t.Errorf("resolveGoType() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestTypesGenerator_ConvertFieldsWithNestedTypes(t *testing.T) {
	g := &TypesGenerator{config: &config.Config{}}
	nestedTypes := make(map[string]NestedTypeData)

	fields := []*mapper.FieldDefinition{
		{
			Name:     "Name",
			JSONName: "name",
			GoType:   "string",
			Required: true,
		},
		{
			Name:     "Address",
			JSONName: "address",
			GoType:   "struct",
			Fields: []*mapper.FieldDefinition{
				{Name: "Street", JSONName: "street", GoType: "string"},
				{Name: "City", JSONName: "city", GoType: "string"},
			},
		},
	}

	result := g.convertFieldsWithNestedTypes(fields, "User", nestedTypes)

	if len(result) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(result))
	}

	// Check that nested type was created
	if len(nestedTypes) != 1 {
		t.Fatalf("expected 1 nested type, got %d", len(nestedTypes))
	}

	addressType, ok := nestedTypes["UserAddress"]
	if !ok {
		t.Fatal("expected UserAddress nested type")
	}
	if len(addressType.Fields) != 2 {
		t.Errorf("expected 2 fields in nested type, got %d", len(addressType.Fields))
	}

	// Check that the field references the nested type
	if result[1].GoType != "UserAddress" {
		t.Errorf("expected GoType 'UserAddress', got %q", result[1].GoType)
	}
}

func TestTypesGenerator_ConvertFieldsWithNestedTypes_ArrayOfStructs(t *testing.T) {
	g := &TypesGenerator{config: &config.Config{}}
	nestedTypes := make(map[string]NestedTypeData)

	fields := []*mapper.FieldDefinition{
		{
			Name:     "Items",
			JSONName: "items",
			GoType:   "[]struct",
			ItemType: &mapper.FieldDefinition{
				GoType: "struct",
				Fields: []*mapper.FieldDefinition{
					{Name: "Id", JSONName: "id", GoType: "int64"},
					{Name: "Name", JSONName: "name", GoType: "string"},
				},
			},
		},
	}

	result := g.convertFieldsWithNestedTypes(fields, "Order", nestedTypes)

	if len(result) != 1 {
		t.Fatalf("expected 1 field, got %d", len(result))
	}

	// Check that nested type was created for array item
	if len(nestedTypes) != 1 {
		t.Fatalf("expected 1 nested type, got %d", len(nestedTypes))
	}

	itemType, ok := nestedTypes["OrderItemsItem"]
	if !ok {
		t.Fatal("expected OrderItemsItem nested type")
	}
	if len(itemType.Fields) != 2 {
		t.Errorf("expected 2 fields in item type, got %d", len(itemType.Fields))
	}

	// Check that the field references the slice of nested type
	if result[0].GoType != "[]OrderItemsItem" {
		t.Errorf("expected GoType '[]OrderItemsItem', got %q", result[0].GoType)
	}
}

func TestTypesGenerator_Generate(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "test.example.com",
		APIVersion: "v1alpha1",
		ModuleName: "github.com/example/test-operator",
	}
	g := NewTypesGenerator(cfg)

	crds := []*mapper.CRDDefinition{
		{
			APIGroup:   "test.example.com",
			APIVersion: "v1alpha1",
			Kind:       "Widget",
			Plural:     "widgets",
			ShortNames: []string{"wg"},
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{
						Name:     "Name",
						JSONName: "name",
						GoType:   "string",
						Required: true,
					},
					{
						Name:     "Count",
						JSONName: "count",
						GoType:   "int64",
						Required: false,
					},
				},
			},
		},
	}

	err := g.Generate(crds)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check types.go was created
	typesPath := filepath.Join(tmpDir, "api", "v1alpha1", "types.go")
	if _, err := os.Stat(typesPath); os.IsNotExist(err) {
		t.Error("expected types.go to exist")
	}

	// Check groupversion_info.go was created
	gvPath := filepath.Join(tmpDir, "api", "v1alpha1", "groupversion_info.go")
	if _, err := os.Stat(gvPath); os.IsNotExist(err) {
		t.Error("expected groupversion_info.go to exist")
	}

	// Verify types.go content
	content, err := os.ReadFile(typesPath)
	if err != nil {
		t.Fatalf("failed to read types.go: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "type WidgetSpec struct") {
		t.Error("expected WidgetSpec struct in types.go")
	}
	if !strings.Contains(contentStr, "type WidgetStatus struct") {
		t.Error("expected WidgetStatus struct in types.go")
	}
}

func TestTypesGenerator_Generate_NestedTypes(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "test.example.com",
		APIVersion: "v1",
		ModuleName: "github.com/example/test-operator",
	}
	g := NewTypesGenerator(cfg)

	crds := []*mapper.CRDDefinition{
		{
			APIGroup:   "test.example.com",
			APIVersion: "v1",
			Kind:       "User",
			Plural:     "users",
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{Name: "Name", JSONName: "name", GoType: "string"},
					{
						Name:     "Address",
						JSONName: "address",
						GoType:   "struct",
						Fields: []*mapper.FieldDefinition{
							{Name: "Street", JSONName: "street", GoType: "string"},
							{Name: "City", JSONName: "city", GoType: "string"},
						},
					},
				},
			},
		},
	}

	err := g.Generate(crds)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify types.go has nested type
	content, err := os.ReadFile(filepath.Join(tmpDir, "api", "v1", "types.go"))
	if err != nil {
		t.Fatalf("failed to read types.go: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "UserAddress") {
		t.Error("expected UserAddress nested type in types.go")
	}
}

// =============================================================================
// ControllerGenerator Tests
// =============================================================================

func TestNewControllerGenerator(t *testing.T) {
	cfg := &config.Config{
		OutputDir:  "/tmp/test",
		APIGroup:   "test.example.com",
		APIVersion: "v1",
	}
	g := NewControllerGenerator(cfg)
	if g == nil {
		t.Fatal("expected non-nil generator")
	}
	if g.config != cfg {
		t.Error("expected config to be set")
	}
}

func TestControllerGenerator_Generate(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "test.example.com",
		APIVersion: "v1alpha1",
		ModuleName: "github.com/example/test-operator",
	}
	g := NewControllerGenerator(cfg)

	crds := []*mapper.CRDDefinition{
		{
			APIGroup:   "test.example.com",
			APIVersion: "v1alpha1",
			Kind:       "Widget",
			Plural:     "widgets",
			BasePath:   "/widgets",
		},
	}

	err := g.Generate(crds, nil)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check controller was created
	controllerPath := filepath.Join(tmpDir, "internal", "controller", "widget_controller.go")
	if _, err := os.Stat(controllerPath); os.IsNotExist(err) {
		t.Error("expected widget_controller.go to exist")
	}

	// Check main.go was created
	mainPath := filepath.Join(tmpDir, "cmd", "manager", "main.go")
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		t.Error("expected main.go to exist")
	}

	// Check go.mod was created
	goModPath := filepath.Join(tmpDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		t.Error("expected go.mod to exist")
	}

	// Check Dockerfile was created
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		t.Error("expected Dockerfile to exist")
	}

	// Check Makefile was created
	makefilePath := filepath.Join(tmpDir, "Makefile")
	if _, err := os.Stat(makefilePath); os.IsNotExist(err) {
		t.Error("expected Makefile to exist")
	}

	// Check boilerplate was created
	boilerplatePath := filepath.Join(tmpDir, "hack", "boilerplate.go.txt")
	if _, err := os.Stat(boilerplatePath); os.IsNotExist(err) {
		t.Error("expected boilerplate.go.txt to exist")
	}
}

func TestControllerGenerator_Generate_MultipleCRDs(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "test.example.com",
		APIVersion: "v1",
		ModuleName: "github.com/example/test-operator",
	}
	g := NewControllerGenerator(cfg)

	crds := []*mapper.CRDDefinition{
		{
			APIGroup:   "test.example.com",
			APIVersion: "v1",
			Kind:       "User",
			Plural:     "users",
			BasePath:   "/users",
		},
		{
			APIGroup:   "test.example.com",
			APIVersion: "v1",
			Kind:       "Pet",
			Plural:     "pets",
			BasePath:   "/pets",
		},
		{
			APIGroup:   "test.example.com",
			APIVersion: "v1",
			Kind:       "Order",
			Plural:     "orders",
			BasePath:   "/orders",
		},
	}

	err := g.Generate(crds, nil)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check all controllers were created
	for _, kind := range []string{"user", "pet", "order"} {
		controllerPath := filepath.Join(tmpDir, "internal", "controller", kind+"_controller.go")
		if _, err := os.Stat(controllerPath); os.IsNotExist(err) {
			t.Errorf("expected %s_controller.go to exist", kind)
		}
	}

	// Verify main.go references all CRDs
	content, err := os.ReadFile(filepath.Join(tmpDir, "cmd", "manager", "main.go"))
	if err != nil {
		t.Fatalf("failed to read main.go: %v", err)
	}
	contentStr := string(content)
	for _, kind := range []string{"User", "Pet", "Order"} {
		if !strings.Contains(contentStr, kind+"Reconciler") {
			t.Errorf("expected %sReconciler in main.go", kind)
		}
	}
}

func TestControllerGenerator_GenerateGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		ModuleName: "github.com/example/my-operator",
	}
	g := NewControllerGenerator(cfg)

	err := g.generateGoMod()
	if err != nil {
		t.Fatalf("generateGoMod failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "go.mod"))
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "module github.com/example/my-operator") {
		t.Error("expected module name in go.mod")
	}
	if !strings.Contains(contentStr, "go 1.25") {
		t.Error("expected go version in go.mod")
	}
	if !strings.Contains(contentStr, "sigs.k8s.io/controller-runtime") {
		t.Error("expected controller-runtime dependency")
	}
}

func TestControllerGenerator_GenerateDockerfile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir: tmpDir,
	}
	g := NewControllerGenerator(cfg)

	err := g.generateDockerfile()
	if err != nil {
		t.Fatalf("generateDockerfile failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "Dockerfile"))
	if err != nil {
		t.Fatalf("failed to read Dockerfile: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "FROM golang:1.25") {
		t.Error("expected Go 1.25 in Dockerfile")
	}
	if !strings.Contains(contentStr, "gcr.io/distroless/static:nonroot") {
		t.Error("expected distroless base image")
	}
	if !strings.Contains(contentStr, "CGO_ENABLED=0") {
		t.Error("expected CGO_ENABLED=0 in Dockerfile")
	}
}

func TestControllerGenerator_GenerateMakefile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir: tmpDir,
	}
	g := NewControllerGenerator(cfg)

	err := g.generateMakefile()
	if err != nil {
		t.Fatalf("generateMakefile failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "Makefile"))
	if err != nil {
		t.Fatalf("failed to read Makefile: %v", err)
	}

	contentStr := string(content)

	// Check key targets
	targets := []string{
		".PHONY: build",
		".PHONY: manifests",
		".PHONY: generate",
		".PHONY: test",
		".PHONY: docker-build",
		".PHONY: install",
		"controller-gen",
	}
	for _, target := range targets {
		if !strings.Contains(contentStr, target) {
			t.Errorf("expected %q in Makefile", target)
		}
	}

	// Check controller-gen version
	if !strings.Contains(contentStr, "CONTROLLER_TOOLS_VERSION ?= v0.17.0") {
		t.Error("expected controller-tools version in Makefile")
	}
}

func TestControllerGenerator_GenerateBoilerplate(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir: tmpDir,
	}
	g := NewControllerGenerator(cfg)

	err := g.generateBoilerplate()
	if err != nil {
		t.Fatalf("generateBoilerplate failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "hack", "boilerplate.go.txt"))
	if err != nil {
		t.Fatalf("failed to read boilerplate.go.txt: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Copyright") {
		t.Error("expected Copyright in boilerplate")
	}
	if !strings.Contains(contentStr, "Apache License") {
		t.Error("expected Apache License in boilerplate")
	}
	if !strings.Contains(contentStr, "openapi-operator-gen") {
		t.Error("expected openapi-operator-gen reference in boilerplate")
	}
}

func TestControllerGenerator_ControllerContent(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "pets.example.com",
		APIVersion: "v1beta1",
		ModuleName: "github.com/example/pet-operator",
	}
	g := NewControllerGenerator(cfg)

	crds := []*mapper.CRDDefinition{
		{
			APIGroup:   "pets.example.com",
			APIVersion: "v1beta1",
			Kind:       "Cat",
			Plural:     "cats",
			BasePath:   "/cats",
		},
	}

	err := g.Generate(crds, nil)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "internal", "controller", "cat_controller.go"))
	if err != nil {
		t.Fatalf("failed to read controller: %v", err)
	}

	contentStr := string(content)

	// Check imports
	if !strings.Contains(contentStr, "github.com/example/pet-operator") {
		t.Error("expected module import in controller")
	}

	// Check struct
	if !strings.Contains(contentStr, "CatReconciler") {
		t.Error("expected CatReconciler struct")
	}

	// Check Reconcile method
	if !strings.Contains(contentStr, "func (r *CatReconciler) Reconcile") {
		t.Error("expected Reconcile method")
	}
}

// =============================================================================
// Edge Cases and Error Handling
// =============================================================================

func TestCRDGenerator_Generate_NilSpec(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "test.example.com",
		APIVersion: "v1",
	}
	g := NewCRDGenerator(cfg)

	crds := []*mapper.CRDDefinition{
		{
			APIGroup:   "test.example.com",
			APIVersion: "v1",
			Kind:       "Widget",
			Plural:     "widgets",
			Scope:      "Namespaced",
			Spec:       nil, // nil spec
		},
	}

	// CRD template requires Spec, so this should return an error
	err := g.Generate(crds)
	if err == nil {
		t.Error("expected error for nil spec")
	}
}

func TestCRDGenerator_Generate_EmptySpecFields(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "test.example.com",
		APIVersion: "v1",
	}
	g := NewCRDGenerator(cfg)

	crds := []*mapper.CRDDefinition{
		{
			APIGroup:   "test.example.com",
			APIVersion: "v1",
			Kind:       "Widget",
			Plural:     "widgets",
			Scope:      "Namespaced",
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{}, // empty fields
			},
		},
	}

	err := g.Generate(crds)
	if err != nil {
		t.Fatalf("Generate should handle empty spec fields: %v", err)
	}
}

func TestTypesGenerator_Generate_EmptyCRDs(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "test.example.com",
		APIVersion: "v1",
		ModuleName: "github.com/example/test",
	}
	g := NewTypesGenerator(cfg)

	err := g.Generate([]*mapper.CRDDefinition{})
	if err != nil {
		t.Fatalf("Generate should handle empty CRDs: %v", err)
	}

	// Files should still be created
	typesPath := filepath.Join(tmpDir, "api", "v1", "types.go")
	if _, err := os.Stat(typesPath); os.IsNotExist(err) {
		t.Error("expected types.go to exist even with no CRDs")
	}
}

func TestControllerGenerator_Generate_EmptyCRDs(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "test.example.com",
		APIVersion: "v1",
		ModuleName: "github.com/example/test",
	}
	g := NewControllerGenerator(cfg)

	err := g.Generate([]*mapper.CRDDefinition{}, nil)
	if err != nil {
		t.Fatalf("Generate should handle empty CRDs: %v", err)
	}

	// main.go should still be created
	mainPath := filepath.Join(tmpDir, "cmd", "manager", "main.go")
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		t.Error("expected main.go to exist even with no CRDs")
	}
}

func TestCRDGenerator_ConvertFields_EmptyFields(t *testing.T) {
	g := &CRDGenerator{config: &config.Config{}}
	result := g.convertFields(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result for nil fields, got %d", len(result))
	}

	result = g.convertFields([]*mapper.FieldDefinition{})
	if len(result) != 0 {
		t.Errorf("expected empty result for empty fields, got %d", len(result))
	}
}

func TestTypesGenerator_ConvertFieldsWithNestedTypes_NoNestedTypes(t *testing.T) {
	g := &TypesGenerator{config: &config.Config{}}
	nestedTypes := make(map[string]NestedTypeData)

	fields := []*mapper.FieldDefinition{
		{Name: "Name", JSONName: "name", GoType: "string"},
		{Name: "Age", JSONName: "age", GoType: "int32"},
	}

	result := g.convertFieldsWithNestedTypes(fields, "User", nestedTypes)

	if len(result) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(result))
	}
	if len(nestedTypes) != 0 {
		t.Errorf("expected no nested types, got %d", len(nestedTypes))
	}
}

// =============================================================================
// Integration Test
// =============================================================================

func TestFullGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "petstore.example.com",
		APIVersion: "v1alpha1",
		ModuleName: "github.com/example/petstore-operator",
	}

	crds := []*mapper.CRDDefinition{
		{
			APIGroup:   "petstore.example.com",
			APIVersion: "v1alpha1",
			Kind:       "Pet",
			Plural:     "pets",
			ShortNames: []string{"pe", "pet"},
			Scope:      "Namespaced",
			BasePath:   "/pet",
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{Name: "Name", JSONName: "name", GoType: "string", Required: true},
					{Name: "Status", JSONName: "status", GoType: "string", Enum: []string{"available", "pending", "sold"}},
					{
						Name:     "Category",
						JSONName: "category",
						GoType:   "struct",
						Fields: []*mapper.FieldDefinition{
							{Name: "Id", JSONName: "id", GoType: "int64"},
							{Name: "Name", JSONName: "name", GoType: "string"},
						},
					},
				},
			},
		},
	}

	// Generate all files
	typesGen := NewTypesGenerator(cfg)
	if err := typesGen.Generate(crds); err != nil {
		t.Fatalf("TypesGenerator.Generate failed: %v", err)
	}

	crdGen := NewCRDGenerator(cfg)
	if err := crdGen.Generate(crds); err != nil {
		t.Fatalf("CRDGenerator.Generate failed: %v", err)
	}

	controllerGen := NewControllerGenerator(cfg)
	if err := controllerGen.Generate(crds, nil); err != nil {
		t.Fatalf("ControllerGenerator.Generate failed: %v", err)
	}

	// Verify all expected files exist
	expectedFiles := []string{
		"api/v1alpha1/types.go",
		"api/v1alpha1/groupversion_info.go",
		"config/crd/bases/petstore.example.com_pets.yaml",
		"internal/controller/pet_controller.go",
		"cmd/manager/main.go",
		"go.mod",
		"Dockerfile",
		"Makefile",
		"hack/boilerplate.go.txt",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(tmpDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", file)
		}
	}

	// Verify types.go has nested Category type
	content, _ := os.ReadFile(filepath.Join(tmpDir, "api", "v1alpha1", "types.go"))
	if !strings.Contains(string(content), "PetCategory") {
		t.Error("expected PetCategory nested type in types.go")
	}
}
