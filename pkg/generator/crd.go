package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/bluecontainer/openapi-operator-gen/internal/config"
	"github.com/bluecontainer/openapi-operator-gen/pkg/mapper"
	"github.com/bluecontainer/openapi-operator-gen/pkg/templates"
)

// CRDGenerator generates CRD YAML manifests
type CRDGenerator struct {
	config *config.Config
}

// NewCRDGenerator creates a new CRD generator
func NewCRDGenerator(cfg *config.Config) *CRDGenerator {
	return &CRDGenerator{config: cfg}
}

// CRDYAMLData holds data for CRD YAML template
type CRDYAMLData struct {
	GeneratorVersion string
	APIGroup         string
	APIVersion       string
	Kind             string
	KindLower        string
	Plural           string
	ShortNames       []string
	Scope            string
	Spec             *CRDSpecData
}

// CRDSpecData holds spec data for CRD YAML
type CRDSpecData struct {
	Fields []CRDFieldData
}

// CRDFieldData holds field data for CRD YAML schema
type CRDFieldData struct {
	JSONName    string
	Description string
	SchemaType  string
	Format      string // OpenAPI format (e.g., int64, int32, double, float, date-time, byte)
	Required    bool
	Enum        []string
	MinLength   *int64
	MaxLength   *int64
	Minimum     *float64
	Maximum     *float64
	Pattern     string
	MinItems    *int64
	MaxItems    *int64
	Fields      []CRDFieldData // nested fields for object types
	ItemType    *CRDFieldData  // item schema for array types
}

// Generate generates CRD YAML files
func (g *CRDGenerator) Generate(crds []*mapper.CRDDefinition) error {
	outputDir := filepath.Join(g.config.OutputDir, "config", "crd", "bases")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Collect CRD filenames for kustomization.yaml
	crdFiles := make([]string, 0, len(crds))

	for _, crd := range crds {
		if err := g.generateCRD(outputDir, crd); err != nil {
			return fmt.Errorf("failed to generate CRD for %s: %w", crd.Kind, err)
		}
		crdFiles = append(crdFiles, fmt.Sprintf("%s_%s.yaml", g.config.APIGroup, crd.Plural))
	}

	// Generate kustomization.yaml for CRDs
	if err := g.generateKustomization(outputDir, crdFiles); err != nil {
		return fmt.Errorf("failed to generate CRD kustomization.yaml: %w", err)
	}

	return nil
}

func (g *CRDGenerator) generateKustomization(outputDir string, crdFiles []string) error {
	data := struct {
		GeneratorVersion string
		CRDFiles         []string
	}{
		GeneratorVersion: g.config.GeneratorVersion,
		CRDFiles:         crdFiles,
	}

	tmpl, err := template.New("kustomization").Parse(templates.KustomizationCRDTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	kustomizationPath := filepath.Join(outputDir, "kustomization.yaml")
	file, err := os.Create(kustomizationPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func (g *CRDGenerator) generateCRD(outputDir string, crd *mapper.CRDDefinition) error {
	data := CRDYAMLData{
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         crd.APIGroup,
		APIVersion:       crd.APIVersion,
		Kind:             crd.Kind,
		KindLower:        strings.ToLower(crd.Kind),
		Plural:           crd.Plural,
		ShortNames:       crd.ShortNames,
		Scope:            crd.Scope,
	}

	if crd.Spec != nil {
		data.Spec = &CRDSpecData{
			Fields: g.convertFields(crd.Spec.Fields),
		}
	}

	// Generate CRD YAML
	filename := fmt.Sprintf("%s_%s.yaml", g.config.APIGroup, crd.Plural)
	filepath := filepath.Join(outputDir, filename)

	funcMap := template.FuncMap{
		"fieldSchema": renderFieldSchema,
	}

	tmpl, err := template.New("crd").Funcs(funcMap).Parse(templates.CRDYAMLTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func (g *CRDGenerator) convertFields(fields []*mapper.FieldDefinition) []CRDFieldData {
	result := make([]CRDFieldData, 0, len(fields))

	for _, f := range fields {
		fd := CRDFieldData{
			JSONName:    f.JSONName,
			Description: f.Description,
			SchemaType:  g.mapToSchemaType(f.GoType),
			Format:      g.mapToSchemaFormat(f.GoType),
			Required:    f.Required,
			Enum:        f.Enum,
		}

		// Copy validation rules
		if f.Validation != nil {
			fd.MinLength = f.Validation.MinLength
			fd.MaxLength = f.Validation.MaxLength
			fd.Minimum = f.Validation.Minimum
			fd.Maximum = f.Validation.Maximum
			fd.Pattern = f.Validation.Pattern
			fd.MinItems = f.Validation.MinItems
			fd.MaxItems = f.Validation.MaxItems
		}

		// Convert nested fields for object types
		if len(f.Fields) > 0 {
			fd.Fields = g.convertFields(f.Fields)
		}

		// Convert item type for array types
		if f.ItemType != nil {
			itemFields := g.convertFields([]*mapper.FieldDefinition{f.ItemType})
			if len(itemFields) > 0 {
				item := itemFields[0]
				fd.ItemType = &item
			}
		}

		result = append(result, fd)
	}

	return result
}

func (g *CRDGenerator) mapToSchemaFormat(goType string) string {
	goType = strings.TrimPrefix(goType, "*")
	switch goType {
	case "int64":
		return "int64"
	case "int32":
		return "int32"
	case "float64":
		return "double"
	case "float32":
		return "float"
	case "metav1.Time":
		return "date-time"
	case "[]byte":
		return "byte"
	default:
		return ""
	}
}

func (g *CRDGenerator) mapToSchemaType(goType string) string {
	// Remove pointer prefix
	goType = strings.TrimPrefix(goType, "*")

	switch goType {
	case "string":
		return "string"
	case "int", "int32", "int64":
		return "integer"
	case "float32", "float64":
		return "number"
	case "bool":
		return "boolean"
	case "runtime.RawExtension":
		return "object"
	case "metav1.Time":
		return "string"
	default:
		if strings.HasPrefix(goType, "[]") {
			return "array"
		}
		if strings.HasPrefix(goType, "map[") {
			return "object"
		}
		return "object"
	}
}

// renderFieldSchema renders a CRDFieldData as an indented YAML schema block.
// indent is the number of spaces for the field name line.
func renderFieldSchema(field CRDFieldData, indent int) string {
	var b strings.Builder
	pad := strings.Repeat(" ", indent)
	inner := pad + "  "

	b.WriteString(pad + field.JSONName + ":\n")

	if field.Description != "" {
		b.WriteString(inner + "description: " + field.Description + "\n")
	}

	b.WriteString(inner + "type: " + field.SchemaType + "\n")

	if field.Format != "" {
		b.WriteString(inner + "format: " + field.Format + "\n")
	}

	// Validation rules
	if field.MinLength != nil {
		b.WriteString(fmt.Sprintf("%sminLength: %d\n", inner, *field.MinLength))
	}
	if field.MaxLength != nil {
		b.WriteString(fmt.Sprintf("%smaxLength: %d\n", inner, *field.MaxLength))
	}
	if field.Minimum != nil {
		b.WriteString(fmt.Sprintf("%sminimum: %v\n", inner, *field.Minimum))
	}
	if field.Maximum != nil {
		b.WriteString(fmt.Sprintf("%smaximum: %v\n", inner, *field.Maximum))
	}
	if field.Pattern != "" {
		b.WriteString(inner + "pattern: " + field.Pattern + "\n")
	}
	if field.MinItems != nil {
		b.WriteString(fmt.Sprintf("%sminItems: %d\n", inner, *field.MinItems))
	}
	if field.MaxItems != nil {
		b.WriteString(fmt.Sprintf("%smaxItems: %d\n", inner, *field.MaxItems))
	}
	if len(field.Enum) > 0 {
		b.WriteString(inner + "enum:\n")
		for _, e := range field.Enum {
			b.WriteString(inner + "- " + e + "\n")
		}
	}

	// Nested object properties
	if len(field.Fields) > 0 {
		b.WriteString(inner + "properties:\n")
		for _, nested := range field.Fields {
			b.WriteString(renderFieldSchema(nested, indent+4))
		}
	}

	// Array items
	if field.SchemaType == "array" && field.ItemType != nil {
		b.WriteString(inner + "items:\n")
		itemInner := inner + "  "
		b.WriteString(itemInner + "type: " + field.ItemType.SchemaType + "\n")
		if field.ItemType.Format != "" {
			b.WriteString(itemInner + "format: " + field.ItemType.Format + "\n")
		}
		if len(field.ItemType.Enum) > 0 {
			b.WriteString(itemInner + "enum:\n")
			for _, e := range field.ItemType.Enum {
				b.WriteString(itemInner + "- " + e + "\n")
			}
		}
		if len(field.ItemType.Fields) > 0 {
			b.WriteString(itemInner + "properties:\n")
			for _, nested := range field.ItemType.Fields {
				b.WriteString(renderFieldSchema(nested, indent+6))
			}
		}
	}

	return b.String()
}
