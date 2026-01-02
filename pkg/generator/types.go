package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/example/openapi-operator-gen/internal/config"
	"github.com/example/openapi-operator-gen/pkg/mapper"
	"github.com/example/openapi-operator-gen/pkg/templates"
)

// TypesGenerator generates Go type definitions for CRDs
type TypesGenerator struct {
	config *config.Config
}

// NewTypesGenerator creates a new types generator
func NewTypesGenerator(cfg *config.Config) *TypesGenerator {
	return &TypesGenerator{config: cfg}
}

// TypesTemplateData holds data for the types template
type TypesTemplateData struct {
	Year        int
	APIVersion  string
	APIGroup    string
	ModuleName  string
	CRDs        []CRDTypeData
	NestedTypes []NestedTypeData // Nested types to generate (for Category, Tag, etc.)
}

// CRDTypeData holds CRD-specific data for template
type CRDTypeData struct {
	Kind       string
	Plural     string
	ShortNames []string
	Spec       *SpecData
}

// SpecData holds spec field data
type SpecData struct {
	Fields []FieldData
}

// FieldData holds field information for template
type FieldData struct {
	Name        string
	JSONName    string
	GoType      string
	Description string
	Required    bool
	Validation  *mapper.ValidationRules
	Enum        []string
	Fields      []FieldData  // nested fields for struct types
	ItemType    *FieldData   // item type for array types
}

// NestedTypeData holds information about a nested type to generate
type NestedTypeData struct {
	Name   string
	Fields []FieldData
}

// Generate generates the types.go file
func (g *TypesGenerator) Generate(crds []*mapper.CRDDefinition) error {
	outputDir := filepath.Join(g.config.OutputDir, "api", g.config.APIVersion)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Track nested types for generation
	nestedTypes := make(map[string]NestedTypeData)

	// Prepare template data
	data := TypesTemplateData{
		Year:       time.Now().Year(),
		APIVersion: g.config.APIVersion,
		APIGroup:   g.config.APIGroup,
		ModuleName: g.config.ModuleName,
		CRDs:       make([]CRDTypeData, 0, len(crds)),
	}

	for _, crd := range crds {
		crdData := CRDTypeData{
			Kind:       crd.Kind,
			Plural:     crd.Plural,
			ShortNames: crd.ShortNames,
		}

		if crd.Spec != nil {
			crdData.Spec = &SpecData{
				Fields: g.convertFieldsWithNestedTypes(crd.Spec.Fields, crd.Kind, nestedTypes),
			}
		}

		data.CRDs = append(data.CRDs, crdData)
	}

	// Convert nested types map to sorted slice for deterministic output
	nestedTypeNames := make([]string, 0, len(nestedTypes))
	for name := range nestedTypes {
		nestedTypeNames = append(nestedTypeNames, name)
	}
	sort.Strings(nestedTypeNames)
	for _, name := range nestedTypeNames {
		data.NestedTypes = append(data.NestedTypes, nestedTypes[name])
	}

	// Generate types.go
	if err := g.generateFile(
		filepath.Join(outputDir, "types.go"),
		templates.TypesTemplate,
		data,
	); err != nil {
		return fmt.Errorf("failed to generate types.go: %w", err)
	}

	// Generate groupversion_info.go
	gvData := struct {
		Year       int
		APIVersion string
		APIGroup   string
		GroupName  string
	}{
		Year:       time.Now().Year(),
		APIVersion: g.config.APIVersion,
		APIGroup:   g.config.APIGroup,
		GroupName:  strings.Split(g.config.APIGroup, ".")[0],
	}

	if err := g.generateFile(
		filepath.Join(outputDir, "groupversion_info.go"),
		templates.GroupVersionInfoTemplate,
		gvData,
	); err != nil {
		return fmt.Errorf("failed to generate groupversion_info.go: %w", err)
	}

	return nil
}

// convertFieldsWithNestedTypes converts fields and extracts nested struct types
// into separate named types for controller-gen compatibility
func (g *TypesGenerator) convertFieldsWithNestedTypes(fields []*mapper.FieldDefinition, prefix string, nestedTypes map[string]NestedTypeData) []FieldData {
	result := make([]FieldData, 0, len(fields))

	for _, f := range fields {
		fd := FieldData{
			Name:        f.Name,
			JSONName:    f.JSONName,
			Description: f.Description,
			Required:    f.Required,
			Validation:  f.Validation,
			Enum:        f.Enum,
		}

		// Handle nested struct types - create named types instead of inline structs
		if f.GoType == "struct" && len(f.Fields) > 0 {
			// Create a named type for this nested struct
			typeName := prefix + f.Name
			if _, exists := nestedTypes[typeName]; !exists {
				nestedTypes[typeName] = NestedTypeData{
					Name:   typeName,
					Fields: g.convertFieldsWithNestedTypes(f.Fields, typeName, nestedTypes),
				}
			}
			fd.GoType = typeName
		} else if f.GoType == "[]struct" && f.ItemType != nil && len(f.ItemType.Fields) > 0 {
			// Create a named type for array item type
			typeName := prefix + f.Name + "Item"
			if _, exists := nestedTypes[typeName]; !exists {
				nestedTypes[typeName] = NestedTypeData{
					Name:   typeName,
					Fields: g.convertFieldsWithNestedTypes(f.ItemType.Fields, typeName, nestedTypes),
				}
			}
			fd.GoType = "[]" + typeName
		} else {
			fd.GoType = g.resolveGoType(f)
		}

		result = append(result, fd)
	}

	return result
}

func (g *TypesGenerator) resolveGoType(f *mapper.FieldDefinition) string {
	goType := f.GoType

	// Handle special types
	switch goType {
	case "runtime.RawExtension":
		return "*runtime.RawExtension"
	case "metav1.Time":
		return "*metav1.Time"
	case "[]metav1.Condition":
		return "[]metav1.Condition"
	}

	// Handle optional types (add pointer for non-required primitive types)
	if !f.Required {
		switch goType {
		case "string", "bool":
			// Keep as-is, will use omitempty
		case "int", "int32", "int64", "float32", "float64":
			goType = "*" + goType
		}
	}

	return goType
}

func (g *TypesGenerator) generateFile(path, tmplContent string, data interface{}) error {
	tmpl, err := template.New("template").Parse(tmplContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
