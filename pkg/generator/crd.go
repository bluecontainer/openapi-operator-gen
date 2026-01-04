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
	APIGroup   string
	APIVersion string
	Kind       string
	KindLower  string
	Plural     string
	ShortNames []string
	Scope      string
	Spec       *CRDSpecData
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
	Required    bool
	Enum        []string
}

// Generate generates CRD YAML files
func (g *CRDGenerator) Generate(crds []*mapper.CRDDefinition) error {
	outputDir := filepath.Join(g.config.OutputDir, "config", "crd", "bases")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, crd := range crds {
		if err := g.generateCRD(outputDir, crd); err != nil {
			return fmt.Errorf("failed to generate CRD for %s: %w", crd.Kind, err)
		}
	}

	return nil
}

func (g *CRDGenerator) generateCRD(outputDir string, crd *mapper.CRDDefinition) error {
	data := CRDYAMLData{
		APIGroup:   crd.APIGroup,
		APIVersion: crd.APIVersion,
		Kind:       crd.Kind,
		KindLower:  strings.ToLower(crd.Kind),
		Plural:     crd.Plural,
		ShortNames: crd.ShortNames,
		Scope:      crd.Scope,
	}

	if crd.Spec != nil {
		data.Spec = &CRDSpecData{
			Fields: g.convertFields(crd.Spec.Fields),
		}
	}

	// Generate CRD YAML
	filename := fmt.Sprintf("%s_%s.yaml", g.config.APIGroup, crd.Plural)
	filepath := filepath.Join(outputDir, filename)

	tmpl, err := template.New("crd").Parse(templates.CRDYAMLTemplate)
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
			Required:    f.Required,
			Enum:        f.Enum,
		}
		result = append(result, fd)
	}

	return result
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
