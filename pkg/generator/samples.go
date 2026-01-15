package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/bluecontainer/openapi-operator-gen/internal/config"
	"github.com/bluecontainer/openapi-operator-gen/pkg/mapper"
	"github.com/bluecontainer/openapi-operator-gen/pkg/templates"
)

// SamplesGenerator generates example CR YAML files
type SamplesGenerator struct {
	config *config.Config
}

// NewSamplesGenerator creates a new samples generator
func NewSamplesGenerator(cfg *config.Config) *SamplesGenerator {
	return &SamplesGenerator{config: cfg}
}

// ExampleCRData holds data for example CR template
type ExampleCRData struct {
	GeneratorVersion string
	APIGroup         string
	APIVersion       string
	Kind             string
	KindLower        string
	IsQuery          bool
	IsAction         bool
	SpecFields       []ExampleFieldData
}

// ExampleFieldData holds field data for example CR
type ExampleFieldData struct {
	JSONName     string
	ExampleValue string
	IsTargeting  bool
}

// ExampleAggregateCRData holds data for example aggregate CR template
type ExampleAggregateCRData struct {
	GeneratorVersion string
	APIGroup         string
	APIVersion       string
	Kind             string
	KindLower        string
	ResourceKinds    []string // CRUD resource kinds
	QueryKinds       []string // Query CRD kinds
	ActionKinds      []string // Action CRD kinds
	AllKinds         []string // All kinds combined
}

// Generate generates example CR YAML files for all CRDs
// aggregate is optional - pass nil if not generating aggregate CRD
func (g *SamplesGenerator) Generate(crds []*mapper.CRDDefinition, aggregate *mapper.AggregateDefinition) error {
	// Create samples directory
	samplesDir := filepath.Join(g.config.OutputDir, "config", "samples")
	if err := os.MkdirAll(samplesDir, 0755); err != nil {
		return fmt.Errorf("failed to create samples directory: %w", err)
	}

	sampleFiles := make([]string, 0, len(crds)*2)

	for _, crd := range crds {
		// Generate basic example CR
		if err := g.generateExampleCR(samplesDir, crd); err != nil {
			return fmt.Errorf("failed to generate example CR for %s: %w", crd.Kind, err)
		}
		sampleFiles = append(sampleFiles, fmt.Sprintf("%s_%s.yaml", g.config.APIVersion, strings.ToLower(crd.Kind)))

		// Generate example CR with externalIDRef (only for resource CRDs that need it)
		// ExternalIDRef is only needed when there are no path parameters to identify the resource
		if !crd.IsQuery && !crd.IsAction && crd.NeedsExternalIDRef {
			if err := g.generateExampleCRRef(samplesDir, crd); err != nil {
				return fmt.Errorf("failed to generate example CR ref for %s: %w", crd.Kind, err)
			}
			sampleFiles = append(sampleFiles, fmt.Sprintf("%s_%s_ref.yaml", g.config.APIVersion, strings.ToLower(crd.Kind)))
		}
	}

	// Generate aggregate sample if aggregate CRD is enabled
	if aggregate != nil {
		if err := g.generateExampleAggregateCR(samplesDir, aggregate); err != nil {
			return fmt.Errorf("failed to generate example aggregate CR: %w", err)
		}
		sampleFiles = append(sampleFiles, fmt.Sprintf("%s_%s.yaml", g.config.APIVersion, strings.ToLower(aggregate.Kind)))
	}

	// Generate kustomization.yaml for samples
	if err := g.generateSamplesKustomization(samplesDir, sampleFiles); err != nil {
		return fmt.Errorf("failed to generate samples kustomization.yaml: %w", err)
	}

	return nil
}

func (g *SamplesGenerator) generateExampleCR(samplesDir string, crd *mapper.CRDDefinition) error {
	data := ExampleCRData{
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         crd.APIGroup,
		APIVersion:       crd.APIVersion,
		Kind:             crd.Kind,
		KindLower:        strings.ToLower(crd.Kind),
		IsQuery:          crd.IsQuery,
		IsAction:         crd.IsAction,
		SpecFields:       g.convertToExampleFields(crd.Spec),
	}

	tmpl, err := template.New("example").Parse(templates.ExampleCRTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.yaml", g.config.APIVersion, strings.ToLower(crd.Kind))
	examplePath := filepath.Join(samplesDir, filename)

	file, err := os.Create(examplePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func (g *SamplesGenerator) generateExampleCRRef(samplesDir string, crd *mapper.CRDDefinition) error {
	data := ExampleCRData{
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         crd.APIGroup,
		APIVersion:       crd.APIVersion,
		Kind:             crd.Kind,
		KindLower:        strings.ToLower(crd.Kind),
		IsQuery:          crd.IsQuery,
		IsAction:         crd.IsAction,
	}

	tmpl, err := template.New("example-ref").Parse(templates.ExampleCRRefTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	filename := fmt.Sprintf("%s_%s_ref.yaml", g.config.APIVersion, strings.ToLower(crd.Kind))
	examplePath := filepath.Join(samplesDir, filename)

	file, err := os.Create(examplePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func (g *SamplesGenerator) generateSamplesKustomization(samplesDir string, sampleFiles []string) error {
	data := struct {
		GeneratorVersion string
		SampleFiles      []string
	}{
		GeneratorVersion: g.config.GeneratorVersion,
		SampleFiles:      sampleFiles,
	}

	tmpl, err := template.New("samples-kustomization").Parse(templates.KustomizationSamplesTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	kustomizationPath := filepath.Join(samplesDir, "kustomization.yaml")
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

func (g *SamplesGenerator) convertToExampleFields(spec *mapper.FieldDefinition) []ExampleFieldData {
	if spec == nil {
		return nil
	}

	result := make([]ExampleFieldData, 0, len(spec.Fields))
	for _, f := range spec.Fields {
		// Skip targeting fields - they'll be shown as comments
		isTargeting := g.isTargetingField(f.JSONName)

		result = append(result, ExampleFieldData{
			JSONName:     f.JSONName,
			ExampleValue: g.generateExampleValue(f),
			IsTargeting:  isTargeting,
		})
	}
	return result
}

func (g *SamplesGenerator) isTargetingField(jsonName string) bool {
	targetingFields := map[string]bool{
		"targetNamespace":   true,
		"targetStatefulSet": true,
		"targetDeployment":  true,
		"targetHelmRelease": true,
		"targetPodOrdinal":  true,
	}
	return targetingFields[jsonName]
}

func (g *SamplesGenerator) generateExampleValue(f *mapper.FieldDefinition) string {
	// If there's an enum, use the first value
	if len(f.Enum) > 0 {
		return fmt.Sprintf("%q", f.Enum[0])
	}

	// Remove pointer prefix
	goType := strings.TrimPrefix(f.GoType, "*")

	switch goType {
	case "string":
		return fmt.Sprintf("%q", "example-"+f.JSONName)
	case "int", "int32", "int64":
		return "1"
	case "float32", "float64":
		return "1.0"
	case "bool":
		return "true"
	case "[]string":
		return fmt.Sprintf("[%q]", "item1")
	case "[]int", "[]int32", "[]int64":
		return "[1, 2, 3]"
	case "metav1.Time":
		// Generate a valid RFC 3339 timestamp for date-time fields
		return fmt.Sprintf("%q", time.Now().UTC().Format(time.RFC3339))
	default:
		if strings.HasPrefix(goType, "[]") {
			return "[]"
		}
		if strings.HasPrefix(goType, "map[") {
			return "{}"
		}
		// For complex types, return empty object
		return "{}"
	}
}

func (g *SamplesGenerator) generateExampleAggregateCR(samplesDir string, aggregate *mapper.AggregateDefinition) error {
	data := ExampleAggregateCRData{
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         aggregate.APIGroup,
		APIVersion:       aggregate.APIVersion,
		Kind:             aggregate.Kind,
		KindLower:        strings.ToLower(aggregate.Kind),
		ResourceKinds:    aggregate.ResourceKinds,
		QueryKinds:       aggregate.QueryKinds,
		ActionKinds:      aggregate.ActionKinds,
		AllKinds:         aggregate.AllKinds,
	}

	tmpl, err := template.New("example-aggregate").Funcs(template.FuncMap{
		"lower": strings.ToLower,
	}).Parse(templates.ExampleAggregateCRTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.yaml", g.config.APIVersion, strings.ToLower(aggregate.Kind))
	examplePath := filepath.Join(samplesDir, filename)

	file, err := os.Create(examplePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
