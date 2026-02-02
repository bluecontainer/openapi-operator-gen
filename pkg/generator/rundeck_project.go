package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/bluecontainer/openapi-operator-gen/internal/config"
	"github.com/bluecontainer/openapi-operator-gen/pkg/mapper"
	"github.com/bluecontainer/openapi-operator-gen/pkg/templates"
)

// RundeckProjectGenerator generates Rundeck project files with job definitions
type RundeckProjectGenerator struct {
	config *config.Config
}

// NewRundeckProjectGenerator creates a new Rundeck project generator
func NewRundeckProjectGenerator(cfg *config.Config) *RundeckProjectGenerator {
	return &RundeckProjectGenerator{config: cfg}
}

// RundeckTemplateData is the top-level data for the project template
type RundeckTemplateData struct {
	GeneratorVersion string
	APIGroup         string
	APIVersion       string
	APIName          string // e.g., "petstore"
	PluginName       string // e.g., "petstore" (kubectl plugin name)
	Namespace        string // e.g., "petstore-system"
}

// RundeckResourceInfo is a CRUD resource with spec fields
type RundeckResourceInfo struct {
	RundeckTemplateData
	Kind      string             // e.g., "Pet"
	KindLower string             // e.g., "pet"
	Plural    string             // e.g., "pets"
	Fields    []RundeckFieldInfo // top-level spec fields
}

// RundeckQueryInfo is a query endpoint with parameters
type RundeckQueryInfo struct {
	RundeckTemplateData
	Kind      string
	KindLower string
	Params    []RundeckFieldInfo // query parameters
}

// RundeckActionInfo is an action endpoint with parameters
type RundeckActionInfo struct {
	RundeckTemplateData
	Kind      string
	KindLower string
	Fields    []RundeckFieldInfo // spec fields (incl. parent ID params)
}

// RundeckFieldInfo maps an OpenAPI field to a Rundeck job option
type RundeckFieldInfo struct {
	Name        string // JSON name (e.g., "name", "status")
	Description string
	Required    bool
	GoType      string   // for type hint in description
	Enum        []string // enforced values list
	IsNested    bool     // true for objects/arrays (use JSON input)
}

// rundeckTemplateSet holds the template strings for a Rundeck project variant.
type rundeckTemplateSet struct {
	ProjectProperties string
	ResourceCreate    string
	ResourceGet       string
	ResourceDescribe  string
	Query             string
	Action            string
	Status            string
	Drift             string
	Cleanup           string
}

// nativeTemplates returns the template set for script-based execution.
func nativeTemplates() rundeckTemplateSet {
	return rundeckTemplateSet{
		ProjectProperties: templates.RundeckProjectPropertiesTemplate,
		ResourceCreate:    templates.RundeckResourceCreateJobTemplate,
		ResourceGet:       templates.RundeckResourceGetJobTemplate,
		ResourceDescribe:  templates.RundeckResourceDescribeJobTemplate,
		Query:             templates.RundeckQueryJobTemplate,
		Action:            templates.RundeckActionJobTemplate,
		Status:            templates.RundeckStatusJobTemplate,
		Drift:             templates.RundeckDriftJobTemplate,
		Cleanup:           templates.RundeckCleanupJobTemplate,
	}
}

// dockerTemplates returns the template set for Docker-based execution.
func dockerTemplates() rundeckTemplateSet {
	return rundeckTemplateSet{
		ProjectProperties: templates.RundeckDockerProjectPropertiesTemplate,
		ResourceCreate:    templates.RundeckDockerResourceCreateJobTemplate,
		ResourceGet:       templates.RundeckDockerResourceGetJobTemplate,
		ResourceDescribe:  templates.RundeckDockerResourceDescribeJobTemplate,
		Query:             templates.RundeckDockerQueryJobTemplate,
		Action:            templates.RundeckDockerActionJobTemplate,
		Status:            templates.RundeckDockerStatusJobTemplate,
		Drift:             templates.RundeckDockerDriftJobTemplate,
		Cleanup:           templates.RundeckDockerCleanupJobTemplate,
	}
}

// Generate generates the script-based Rundeck project files.
func (g *RundeckProjectGenerator) Generate(crds []*mapper.CRDDefinition) error {
	return g.generateProject(crds, "rundeck-project", nativeTemplates())
}

// GenerateDockerProject generates the Docker-based Rundeck project files.
func (g *RundeckProjectGenerator) GenerateDockerProject(crds []*mapper.CRDDefinition) error {
	return g.generateProject(crds, "rundeck-docker-project", dockerTemplates())
}

// GeneratePluginDockerfile generates the kubectl plugin Dockerfile.
func (g *RundeckProjectGenerator) GeneratePluginDockerfile() error {
	apiName := strings.Split(g.config.APIGroup, ".")[0]
	data := struct {
		GeneratorVersion string
		AppName          string
	}{
		GeneratorVersion: g.config.GeneratorVersion,
		AppName:          apiName,
	}

	outputPath := filepath.Join(g.config.OutputDir, "kubectl-plugin", "Dockerfile")
	return g.executeTemplate(templates.KubectlPluginDockerfileTemplate, data, outputPath)
}

// generateProject generates a Rundeck project using the given template set.
func (g *RundeckProjectGenerator) generateProject(crds []*mapper.CRDDefinition, dirName string, tmplSet rundeckTemplateSet) error {
	// Create directory structure
	rundeckDir := filepath.Join(g.config.OutputDir, dirName)
	dirs := []string{
		rundeckDir,
		filepath.Join(rundeckDir, "jobs", "resources"),
		filepath.Join(rundeckDir, "jobs", "queries"),
		filepath.Join(rundeckDir, "jobs", "actions"),
		filepath.Join(rundeckDir, "jobs", "operations"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Prepare base template data
	apiName := strings.Split(g.config.APIGroup, ".")[0]
	baseData := RundeckTemplateData{
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         g.config.APIGroup,
		APIVersion:       g.config.APIVersion,
		APIName:          apiName,
		PluginName:       apiName,
		Namespace:        apiName + "-system",
	}

	// Generate project.properties
	if err := g.executeTemplate(
		tmplSet.ProjectProperties,
		baseData,
		filepath.Join(rundeckDir, "project.properties"),
	); err != nil {
		return fmt.Errorf("failed to generate project.properties: %w", err)
	}

	// Generate tokens.properties for Rundeck API authentication
	// Format: username: token_string, role1, role2
	tokensContent := fmt.Sprintf("# Generated by openapi-operator-gen %s\n# Static API token for automated setup (admin group required for project/job management)\nadmin: letmein99, admin, user\n", g.config.GeneratorVersion)
	if err := os.WriteFile(filepath.Join(rundeckDir, "tokens.properties"), []byte(tokensContent), 0644); err != nil {
		return fmt.Errorf("failed to generate tokens.properties: %w", err)
	}

	// Categorize CRDs and generate per-kind job files
	for _, crd := range crds {
		if crd.IsQuery {
			queryInfo := RundeckQueryInfo{
				RundeckTemplateData: baseData,
				Kind:                crd.Kind,
				KindLower:           strings.ToLower(crd.Kind),
				Params:              g.mapQueryParams(crd),
			}
			if err := g.executeTemplate(
				tmplSet.Query,
				queryInfo,
				filepath.Join(rundeckDir, "jobs", "queries", strings.ToLower(crd.Kind)+".yaml"),
			); err != nil {
				return fmt.Errorf("failed to generate query job for %s: %w", crd.Kind, err)
			}
		} else if crd.IsAction {
			actionInfo := RundeckActionInfo{
				RundeckTemplateData: baseData,
				Kind:                crd.Kind,
				KindLower:           strings.ToLower(crd.Kind),
				Fields:              g.mapFields(crd.Spec),
			}
			if err := g.executeTemplate(
				tmplSet.Action,
				actionInfo,
				filepath.Join(rundeckDir, "jobs", "actions", strings.ToLower(crd.Kind)+".yaml"),
			); err != nil {
				return fmt.Errorf("failed to generate action job for %s: %w", crd.Kind, err)
			}
		} else {
			// CRUD resource — generate create, get, describe jobs
			resourceInfo := RundeckResourceInfo{
				RundeckTemplateData: baseData,
				Kind:                crd.Kind,
				KindLower:           strings.ToLower(crd.Kind),
				Plural:              crd.Plural,
				Fields:              g.mapFields(crd.Spec),
			}

			jobTemplates := []struct {
				tmpl     string
				filename string
			}{
				{tmplSet.ResourceCreate, "create-" + strings.ToLower(crd.Kind) + ".yaml"},
				{tmplSet.ResourceGet, "get-" + crd.Plural + ".yaml"},
				{tmplSet.ResourceDescribe, "describe-" + strings.ToLower(crd.Kind) + ".yaml"},
			}
			for _, jt := range jobTemplates {
				if err := g.executeTemplate(
					jt.tmpl,
					resourceInfo,
					filepath.Join(rundeckDir, "jobs", "resources", jt.filename),
				); err != nil {
					return fmt.Errorf("failed to generate %s: %w", jt.filename, err)
				}
			}
		}
	}

	// Generate operations jobs (status, drift, cleanup)
	operationsTemplates := []struct {
		tmpl     string
		filename string
	}{
		{tmplSet.Status, "status.yaml"},
		{tmplSet.Drift, "drift.yaml"},
		{tmplSet.Cleanup, "cleanup.yaml"},
	}
	for _, ot := range operationsTemplates {
		if err := g.executeTemplate(
			ot.tmpl,
			baseData,
			filepath.Join(rundeckDir, "jobs", "operations", ot.filename),
		); err != nil {
			return fmt.Errorf("failed to generate %s: %w", ot.filename, err)
		}
	}

	return nil
}

// mapFields converts a FieldDefinition's children to RundeckFieldInfo slice.
// Only includes top-level spec fields (not the Spec wrapper itself).
func (g *RundeckProjectGenerator) mapFields(spec *mapper.FieldDefinition) []RundeckFieldInfo {
	if spec == nil || len(spec.Fields) == 0 {
		return nil
	}

	fields := make([]RundeckFieldInfo, 0, len(spec.Fields))
	for _, f := range spec.Fields {
		// Skip operator-internal fields that aren't part of the API resource
		if isOperatorInternalField(f.JSONName) {
			continue
		}

		info := RundeckFieldInfo{
			Name:        f.JSONName,
			Description: f.Description,
			Required:    f.Required,
			GoType:      f.GoType,
			IsNested:    f.Fields != nil || f.ItemType != nil,
		}

		if len(f.Enum) > 0 {
			info.Enum = f.Enum
		}

		fields = append(fields, info)
	}

	return fields
}

// mapQueryParams extracts query parameters from a query CRD's spec fields.
func (g *RundeckProjectGenerator) mapQueryParams(crd *mapper.CRDDefinition) []RundeckFieldInfo {
	// Query CRDs store their parameters as spec fields
	if crd.Spec == nil || len(crd.Spec.Fields) == 0 {
		return nil
	}

	params := make([]RundeckFieldInfo, 0, len(crd.Spec.Fields))
	for _, f := range crd.Spec.Fields {
		if isOperatorInternalField(f.JSONName) {
			continue
		}

		info := RundeckFieldInfo{
			Name:        f.JSONName,
			Description: f.Description,
			Required:    f.Required,
			GoType:      f.GoType,
			IsNested:    f.Fields != nil || f.ItemType != nil,
		}

		if len(f.Enum) > 0 {
			info.Enum = f.Enum
		}

		params = append(params, info)
	}

	return params
}

// isOperatorInternalField returns true for fields that are added by the operator
// framework and not part of the user-facing API resource data.
func isOperatorInternalField(jsonName string) bool {
	internalFields := map[string]bool{
		"target":            true,
		"mergeOnUpdate":     true,
		"onDelete":          true,
		"reExecuteInterval": true,
		"externalIDRef":     true,
		"data":              true, // binary data field
		"dataFrom":          true,
		"dataURL":           true,
		"dataFromFile":      true,
		"contentType":       true,
		"refreshInterval":   true,
		"paused":            true,
		"triggerOnce":       true,
		"reconcileInterval": true,
		"deletionPolicy":    true,
		"retryPolicy":       true,
	}
	return internalFields[jsonName]
}

// executeTemplate parses and executes a template, writing the result to outputPath.
func (g *RundeckProjectGenerator) executeTemplate(tmplContent string, data interface{}, outputPath string) error {
	funcMap := template.FuncMap{
		"lower": strings.ToLower,
		"upper": strings.ToUpper,
		"join": func(items []string, sep string) string {
			return strings.Join(items, sep)
		},
	}

	tmpl, err := template.New("rundeck").Funcs(funcMap).Parse(tmplContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}
