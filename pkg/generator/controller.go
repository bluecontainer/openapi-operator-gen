package generator

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/bluecontainer/openapi-operator-gen/internal/config"
	"github.com/bluecontainer/openapi-operator-gen/pkg/mapper"
	"github.com/bluecontainer/openapi-operator-gen/pkg/templates"
	"github.com/iancoleman/strcase"
)

// ControllerGenerator generates controller reconciliation logic
type ControllerGenerator struct {
	config *config.Config
}

// NewControllerGenerator creates a new controller generator
func NewControllerGenerator(cfg *config.Config) *ControllerGenerator {
	return &ControllerGenerator{config: cfg}
}

// ControllerTemplateData holds data for controller template
type ControllerTemplateData struct {
	Year             int
	GeneratorVersion string
	APIGroup         string
	APIVersion       string
	ModuleName       string
	Kind             string
	KindLower        string
	Plural           string
	BasePath         string
	IsQuery          bool                     // True if this is a query CRD
	QueryPath        string                   // Full query path for query CRDs
	QueryParams      []mapper.QueryParamField // Query parameters for building URL
	ResponseType     string                   // Go type for response (e.g., "[]Pet" or "[]PetFindByTagsResult")
	ResponseIsArray  bool                     // True if response is an array
	ResultItemType   string                   // Item type if ResponseIsArray (e.g., "Pet" or "PetFindByTagsResult")
	HasTypedResults  bool                     // True if we have typed results (not raw extension)
	UsesSharedType   bool                     // True if ResultItemType is a shared type from another CRD

	// Action endpoint fields
	IsAction          bool                     // True if this is an action CRD
	ActionPath        string                   // Full action path (e.g., /pet/{petId}/uploadImage)
	ActionMethod      string                   // HTTP method (POST, PUT, or GET)
	ParentResource    string                   // Parent resource kind (e.g., "Pet")
	ParentIDParam     string                   // Parent ID parameter name (e.g., "petId")
	ParentIDField     string                   // Go field name for parent ID (e.g., "PetId")
	HasParentID       bool                     // True if the action has a parent ID parameter
	ActionName        string                   // Action name (e.g., "uploadImage")
	PathParams        []ActionPathParam        // Path parameters other than parent ID
	RequestBodyFields []ActionRequestBodyField // Request body fields
	HasRequestBody    bool                     // True if there are request body fields

	// Resource endpoint fields (for standard CRUD resources)
	ResourcePathParams  []ActionPathParam    // Path parameters for resource endpoints
	ResourceQueryParams []ResourceQueryParam // Query parameters for resource endpoints
	HasResourceParams   bool                 // True if there are path or query params to handle

	// Integration test fields
	RequiredFields    []RequiredFieldInfo // Required fields that need sample values in tests
	HasRequiredFields bool                // True if there are required fields
}

// ActionPathParam represents a path parameter in action templates
type ActionPathParam struct {
	Name   string // Parameter name (e.g., "userId")
	GoName string // Go field name (e.g., "UserId")
	GoType string // Go type (e.g., "string", "int64")
}

// ActionRequestBodyField represents a request body field in action templates
type ActionRequestBodyField struct {
	JSONName string // JSON field name (e.g., "additionalMetadata")
	GoName   string // Go field name (e.g., "AdditionalMetadata")
}

// ResourceQueryParam represents a query parameter for resource endpoints
type ResourceQueryParam struct {
	Name     string // Parameter name as it appears in URL (e.g., "status")
	JSONName string // JSON field name (e.g., "status")
	GoName   string // Go field name (e.g., "Status")
	GoType   string // Go type (e.g., "string", "int64")
	IsArray  bool   // True if this is an array parameter
}

// RequiredFieldInfo holds information about a required field for test generation
type RequiredFieldInfo struct {
	GoName       string // Go field name (e.g., "PhotoUrls")
	GoType       string // Go type (e.g., "[]string")
	IsArray      bool   // True if field is an array type
	IsStringType bool   // True if field or array item is string type
}

// MainTemplateData holds data for main.go template
type MainTemplateData struct {
	Year             int
	GeneratorVersion string
	APIVersion       string
	ModuleName       string
	AppName          string
	CRDs             []CRDMainData
	HasAggregate     bool   // True if aggregate CRD is generated
	AggregateKind    string // Kind name of the aggregate CRD (e.g., "StatusAggregate")
}

// CRDMainData holds CRD data for main.go
type CRDMainData struct {
	Kind     string
	IsQuery  bool
	IsAction bool
}

// Generate generates controller files
// aggregate is optional - pass nil if not generating aggregate CRD
func (g *ControllerGenerator) Generate(crds []*mapper.CRDDefinition, aggregate *mapper.AggregateDefinition) error {
	controllerDir := filepath.Join(g.config.OutputDir, "internal", "controller")
	if err := os.MkdirAll(controllerDir, 0755); err != nil {
		return fmt.Errorf("failed to create controller directory: %w", err)
	}

	// Generate a controller for each CRD
	for _, crd := range crds {
		if err := g.generateController(controllerDir, crd); err != nil {
			return fmt.Errorf("failed to generate controller for %s: %w", crd.Kind, err)
		}
		// Generate test file for the controller
		if err := g.generateControllerTest(controllerDir, crd); err != nil {
			return fmt.Errorf("failed to generate controller test for %s: %w", crd.Kind, err)
		}
		// Generate integration test file for the controller
		if err := g.generateIntegrationTest(controllerDir, crd); err != nil {
			return fmt.Errorf("failed to generate integration test for %s: %w", crd.Kind, err)
		}
	}

	// Generate suite_test.go for envtest (only once, not per CRD)
	if err := g.generateSuiteTest(controllerDir); err != nil {
		return fmt.Errorf("failed to generate suite_test.go: %w", err)
	}

	// Generate main.go (with optional aggregate info)
	if err := g.generateMain(crds, aggregate); err != nil {
		return fmt.Errorf("failed to generate main.go: %w", err)
	}

	// Generate go.mod for the generated operator
	if err := g.generateGoMod(aggregate != nil); err != nil {
		return fmt.Errorf("failed to generate go.mod: %w", err)
	}

	// Generate Dockerfile
	if err := g.generateDockerfile(); err != nil {
		return fmt.Errorf("failed to generate Dockerfile: %w", err)
	}

	// Generate Makefile
	if err := g.generateMakefile(); err != nil {
		return fmt.Errorf("failed to generate Makefile: %w", err)
	}

	// Generate README.md
	if err := g.generateReadme(crds, aggregate != nil); err != nil {
		return fmt.Errorf("failed to generate README: %w", err)
	}

	// Generate hack/boilerplate.go.txt for controller-gen
	if err := g.generateBoilerplate(); err != nil {
		return fmt.Errorf("failed to generate boilerplate: %w", err)
	}

	// Generate deployment manifests (namespace, service account, deployment, role binding)
	if err := g.generateDeploymentManifests(); err != nil {
		return fmt.Errorf("failed to generate deployment manifests: %w", err)
	}

	// Copy the OpenAPI spec file to the output directory
	if err := g.copySpecFile(); err != nil {
		return fmt.Errorf("failed to copy spec file: %w", err)
	}

	return nil
}

func (g *ControllerGenerator) generateController(outputDir string, crd *mapper.CRDDefinition) error {
	data := ControllerTemplateData{
		Year:             time.Now().Year(),
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         crd.APIGroup,
		APIVersion:       crd.APIVersion,
		ModuleName:       g.config.ModuleName,
		Kind:             crd.Kind,
		KindLower:        strings.ToLower(crd.Kind),
		Plural:           crd.Plural,
		BasePath:         crd.BasePath,
		IsQuery:          crd.IsQuery,
		QueryPath:        crd.QueryPath,
		QueryParams:      crd.QueryParams,
		ResponseType:     crd.ResponseType,
		ResponseIsArray:  crd.ResponseIsArray,
		ResultItemType:   crd.ResultItemType,
		HasTypedResults:  len(crd.ResultFields) > 0 || crd.UsesSharedType,
		UsesSharedType:   crd.UsesSharedType,
		// Action fields
		IsAction:       crd.IsAction,
		ActionPath:     crd.ActionPath,
		ActionMethod:   crd.ActionMethod,
		ParentResource: crd.ParentResource,
		ParentIDParam:  crd.ParentIDParam,
		ParentIDField:  strcase.ToCamel(crd.ParentIDParam),
		HasParentID:    crd.ParentIDParam != "",
		ActionName:     crd.ActionName,
	}

	// Populate path params (excluding parent ID)
	if crd.IsAction && crd.Spec != nil {
		for _, field := range crd.Spec.Fields {
			// Skip the parent ID field - already handled separately
			if strings.EqualFold(field.JSONName, strcase.ToLowerCamel(crd.ParentIDParam)) {
				continue
			}
			// Skip targeting fields
			if field.JSONName == "targetPodOrdinal" || field.JSONName == "targetHelmRelease" ||
				field.JSONName == "targetStatefulSet" || field.JSONName == "targetDeployment" ||
				field.JSONName == "targetNamespace" {
				continue
			}
			// Check if this is a path param by looking at the action path
			if strings.Contains(crd.ActionPath, "{"+field.JSONName+"}") {
				data.PathParams = append(data.PathParams, ActionPathParam{
					Name:   field.JSONName,
					GoName: field.Name,
				})
			} else {
				// It's a request body field
				data.RequestBodyFields = append(data.RequestBodyFields, ActionRequestBodyField{
					JSONName: field.JSONName,
					GoName:   field.Name,
				})
			}
		}
		data.HasRequestBody = len(data.RequestBodyFields) > 0
	}

	// Populate path and query params for resource endpoints (non-query, non-action)
	if !crd.IsQuery && !crd.IsAction {
		// Collect unique path params from operations
		pathParamsSeen := make(map[string]bool)
		queryParamsSeen := make(map[string]bool)

		for _, op := range crd.Operations {
			for _, paramName := range op.PathParams {
				if pathParamsSeen[paramName] {
					continue
				}
				pathParamsSeen[paramName] = true
				// Find the field in spec to get type info
				goType := "string" // default
				if crd.Spec != nil {
					for _, field := range crd.Spec.Fields {
						if strings.EqualFold(field.JSONName, strcase.ToLowerCamel(paramName)) {
							goType = field.GoType
							break
						}
					}
				}
				data.ResourcePathParams = append(data.ResourcePathParams, ActionPathParam{
					Name:   paramName,
					GoName: strcase.ToCamel(paramName),
					GoType: goType,
				})
			}
			for _, paramName := range op.QueryParams {
				if queryParamsSeen[paramName] {
					continue
				}
				queryParamsSeen[paramName] = true
				// Find the field in spec to get type info
				isArray := false
				goType := "string" // default
				if crd.Spec != nil {
					for _, field := range crd.Spec.Fields {
						if strings.EqualFold(field.JSONName, strcase.ToLowerCamel(paramName)) {
							goType = field.GoType
							isArray = strings.HasPrefix(field.GoType, "[]")
							break
						}
					}
				}
				data.ResourceQueryParams = append(data.ResourceQueryParams, ResourceQueryParam{
					Name:     paramName,
					JSONName: strcase.ToLowerCamel(paramName),
					GoName:   strcase.ToCamel(paramName),
					GoType:   goType,
					IsArray:  isArray,
				})
			}
		}
		data.HasResourceParams = len(data.ResourcePathParams) > 0 || len(data.ResourceQueryParams) > 0
	}

	filename := fmt.Sprintf("%s_controller.go", strings.ToLower(crd.Kind))
	fp := filepath.Join(outputDir, filename)

	// Choose appropriate template based on CRD type
	var tmplContent string
	if crd.IsQuery {
		tmplContent = templates.QueryControllerTemplate
	} else if crd.IsAction {
		tmplContent = templates.ActionControllerTemplate
	} else {
		tmplContent = templates.ControllerTemplate
	}

	tmpl, err := template.New("controller").Parse(tmplContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(fp)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func (g *ControllerGenerator) generateControllerTest(outputDir string, crd *mapper.CRDDefinition) error {
	data := ControllerTemplateData{
		Year:            time.Now().Year(),
		APIGroup:        crd.APIGroup,
		APIVersion:      crd.APIVersion,
		ModuleName:      g.config.ModuleName,
		Kind:            crd.Kind,
		KindLower:       strings.ToLower(crd.Kind),
		Plural:          crd.Plural,
		BasePath:        crd.BasePath,
		IsQuery:         crd.IsQuery,
		QueryPath:       crd.QueryPath,
		IsAction:        crd.IsAction,
		ActionPath:      crd.ActionPath,
		ActionMethod:    crd.ActionMethod,
		ResponseIsArray: crd.ResponseIsArray,

		ParentResource: crd.ParentResource,
		ParentIDParam:  crd.ParentIDParam,
		ParentIDField:  strcase.ToCamel(crd.ParentIDParam),
		HasParentID:    crd.ParentIDParam != "",
		ActionName:     crd.ActionName,
	}

	filename := fmt.Sprintf("%s_controller_test.go", strings.ToLower(crd.Kind))
	fp := filepath.Join(outputDir, filename)

	tmpl, err := template.New("controller_test").Parse(templates.ControllerTestTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(fp)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// SuiteTestTemplateData holds data for the suite_test.go template
type SuiteTestTemplateData struct {
	Year             int
	GeneratorVersion string
	APIVersion       string
	ModuleName       string
	KubeVersion      string
}

func (g *ControllerGenerator) generateSuiteTest(outputDir string) error {
	data := SuiteTestTemplateData{
		Year:             time.Now().Year(),
		GeneratorVersion: g.config.GeneratorVersion,
		APIVersion:       g.config.APIVersion,
		ModuleName:       g.config.ModuleName,
		KubeVersion:      "1.29.0",
	}

	fp := filepath.Join(outputDir, "suite_test.go")

	tmpl, err := template.New("suite_test").Parse(templates.SuiteTestTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(fp)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func (g *ControllerGenerator) generateIntegrationTest(outputDir string, crd *mapper.CRDDefinition) error {
	// Extract required fields from the CRD spec
	var requiredFields []RequiredFieldInfo
	if crd.Spec != nil {
		for _, field := range crd.Spec.Fields {
			if field.Required {
				isArray := strings.HasPrefix(field.GoType, "[]")
				itemType := strings.TrimPrefix(field.GoType, "[]")
				isStringType := itemType == "string" || field.GoType == "string"
				requiredFields = append(requiredFields, RequiredFieldInfo{
					GoName:       strcase.ToCamel(field.Name),
					GoType:       field.GoType,
					IsArray:      isArray,
					IsStringType: isStringType,
				})
			}
		}
	}

	data := ControllerTemplateData{
		Year:            time.Now().Year(),
		APIGroup:        crd.APIGroup,
		APIVersion:      crd.APIVersion,
		ModuleName:      g.config.ModuleName,
		Kind:            crd.Kind,
		KindLower:       strings.ToLower(crd.Kind),
		Plural:          crd.Plural,
		BasePath:        crd.BasePath,
		IsQuery:         crd.IsQuery,
		QueryPath:       crd.QueryPath,
		IsAction:        crd.IsAction,
		ActionPath:      crd.ActionPath,
		ActionMethod:    crd.ActionMethod,
		ResponseIsArray: crd.ResponseIsArray,

		ParentResource: crd.ParentResource,
		ParentIDParam:  crd.ParentIDParam,
		ParentIDField:  strcase.ToCamel(crd.ParentIDParam),
		HasParentID:    crd.ParentIDParam != "",
		ActionName:     crd.ActionName,

		RequiredFields:    requiredFields,
		HasRequiredFields: len(requiredFields) > 0,
	}

	filename := fmt.Sprintf("%s_integration_test.go", strings.ToLower(crd.Kind))
	fp := filepath.Join(outputDir, filename)

	tmpl, err := template.New("integration_test").Parse(templates.IntegrationTestTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(fp)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func (g *ControllerGenerator) generateMain(crds []*mapper.CRDDefinition, aggregate *mapper.AggregateDefinition) error {
	cmdDir := filepath.Join(g.config.OutputDir, "cmd", "manager")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		return fmt.Errorf("failed to create cmd directory: %w", err)
	}

	data := MainTemplateData{
		Year:             time.Now().Year(),
		GeneratorVersion: g.config.GeneratorVersion,
		APIVersion:       g.config.APIVersion,
		ModuleName:       g.config.ModuleName,
		AppName:          strings.Split(g.config.APIGroup, ".")[0],
		CRDs:             make([]CRDMainData, 0, len(crds)),
	}

	for _, crd := range crds {
		data.CRDs = append(data.CRDs, CRDMainData{Kind: crd.Kind, IsQuery: crd.IsQuery, IsAction: crd.IsAction})
	}

	// Add aggregate info if provided
	if aggregate != nil {
		data.HasAggregate = true
		data.AggregateKind = aggregate.Kind
	}

	filepath := filepath.Join(cmdDir, "main.go")

	tmpl, err := template.New("main").Parse(templates.MainTemplate)
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

func (g *ControllerGenerator) generateGoMod(hasAggregate bool) error {
	// Use the generator version for the dependency
	// Only use clean semver versions (vX.Y.Z), otherwise fall back to v0.0.0
	// which will be resolved to latest by go mod tidy
	version := g.config.GeneratorVersion
	if !isValidSemver(version) {
		version = "v0.0.0"
	}

	data := struct {
		ModuleName       string
		GeneratorVersion string
		HasAggregate     bool
	}{
		ModuleName:       g.config.ModuleName,
		GeneratorVersion: version,
		HasAggregate:     hasAggregate,
	}
	outputPath := filepath.Join(g.config.OutputDir, "go.mod")
	return g.executeTemplate(templates.GoModTemplate, data, outputPath)
}

// isValidSemver checks if a version string is a clean semver (vX.Y.Z)
// Returns false for dev builds, dirty versions, or versions with extra commits
func isValidSemver(version string) bool {
	if version == "" || version == "dev" {
		return false
	}
	// Must start with 'v'
	if !strings.HasPrefix(version, "v") {
		return false
	}
	// Check for dirty or extra commit indicators (e.g., v0.0.6-1-g783aaa8-dirty)
	if strings.Contains(version, "-") {
		return false
	}
	// Basic check: should be vX.Y.Z format
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) != 3 {
		return false
	}
	// Each part should be numeric
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

func (g *ControllerGenerator) generateDockerfile() error {
	data := struct {
		GeneratorVersion string
	}{
		GeneratorVersion: g.config.GeneratorVersion,
	}
	outputPath := filepath.Join(g.config.OutputDir, "Dockerfile")
	return g.executeTemplate(templates.DockerfileTemplate, data, outputPath)
}

func (g *ControllerGenerator) generateMakefile() error {
	data := struct {
		AppName          string
		GeneratorVersion string
	}{
		AppName:          strings.Split(g.config.APIGroup, ".")[0],
		GeneratorVersion: g.config.GeneratorVersion,
	}
	outputPath := filepath.Join(g.config.OutputDir, "Makefile")
	return g.executeTemplate(templates.MakefileTemplate, data, outputPath)
}

func (g *ControllerGenerator) generateReadme(crds []*mapper.CRDDefinition, hasAggregate bool) error {
	// Build CRD info for template
	type CRDInfo struct {
		Kind     string
		IsQuery  bool
		IsAction bool
	}
	crdInfos := make([]CRDInfo, 0, len(crds))
	for _, crd := range crds {
		crdInfos = append(crdInfos, CRDInfo{
			Kind:     crd.Kind,
			IsQuery:  crd.IsQuery,
			IsAction: crd.IsAction,
		})
	}

	appName := strings.Split(g.config.APIGroup, ".")[0]
	// Capitalize first letter for title
	titleAppName := appName
	if len(appName) > 0 {
		titleAppName = strings.ToUpper(appName[:1]) + appName[1:]
	}

	// Build generator command line
	generatorCmd := fmt.Sprintf("openapi-operator-gen generate \\\n  --spec %s \\\n  --output %s \\\n  --group %s \\\n  --version %s \\\n  --module %s",
		g.config.SpecPath,
		g.config.OutputDir,
		g.config.APIGroup,
		g.config.APIVersion,
		g.config.ModuleName,
	)
	if hasAggregate {
		generatorCmd += " \\\n  --aggregate"
	}

	data := struct {
		AppName          string
		TitleAppName     string
		APIGroup         string
		APIVersion       string
		CRDs             []CRDInfo
		GeneratorCmd     string
		HasAggregate     bool
		GeneratorVersion string
	}{
		AppName:          appName,
		TitleAppName:     titleAppName,
		APIGroup:         g.config.APIGroup,
		APIVersion:       g.config.APIVersion,
		CRDs:             crdInfos,
		GeneratorCmd:     generatorCmd,
		HasAggregate:     hasAggregate,
		GeneratorVersion: g.config.GeneratorVersion,
	}
	outputPath := filepath.Join(g.config.OutputDir, "README.md")
	return g.executeTemplate(templates.ReadmeTemplate, data, outputPath)
}

func (g *ControllerGenerator) generateBoilerplate() error {
	hackDir := filepath.Join(g.config.OutputDir, "hack")
	if err := os.MkdirAll(hackDir, 0755); err != nil {
		return fmt.Errorf("failed to create hack directory: %w", err)
	}

	data := struct {
		Year             int
		GeneratorVersion string
	}{
		Year:             time.Now().Year(),
		GeneratorVersion: g.config.GeneratorVersion,
	}
	outputPath := filepath.Join(hackDir, "boilerplate.go.txt")
	return g.executeTemplate(templates.BoilerplateTemplate, data, outputPath)
}

// DeploymentManifestData holds data for generating deployment YAML manifests
type DeploymentManifestData struct {
	Namespace        string
	AppName          string
	GeneratorVersion string
}

func (g *ControllerGenerator) generateDeploymentManifests() error {
	// Derive namespace from API group (e.g., petstore.example.com -> petstore-system)
	data := DeploymentManifestData{
		Namespace:        strings.Split(g.config.APIGroup, ".")[0] + "-system",
		AppName:          strings.Split(g.config.APIGroup, ".")[0],
		GeneratorVersion: g.config.GeneratorVersion,
	}

	// Create config directories
	managerDir := filepath.Join(g.config.OutputDir, "config", "manager")
	if err := os.MkdirAll(managerDir, 0755); err != nil {
		return fmt.Errorf("failed to create manager directory: %w", err)
	}

	rbacDir := filepath.Join(g.config.OutputDir, "config", "rbac")
	if err := os.MkdirAll(rbacDir, 0755); err != nil {
		return fmt.Errorf("failed to create rbac directory: %w", err)
	}

	// Generate namespace.yaml
	if err := g.executeTemplate(templates.NamespaceYAMLTemplate, data,
		filepath.Join(g.config.OutputDir, "config", "namespace.yaml")); err != nil {
		return fmt.Errorf("failed to generate namespace.yaml: %w", err)
	}

	// Generate config/rbac/service_account.yaml
	if err := g.executeTemplate(templates.ServiceAccountYAMLTemplate, data,
		filepath.Join(rbacDir, "service_account.yaml")); err != nil {
		return fmt.Errorf("failed to generate service_account.yaml: %w", err)
	}

	// Generate config/rbac/role_binding.yaml
	if err := g.executeTemplate(templates.RoleBindingYAMLTemplate, data,
		filepath.Join(rbacDir, "role_binding.yaml")); err != nil {
		return fmt.Errorf("failed to generate role_binding.yaml: %w", err)
	}

	// Generate config/manager/manager.yaml (Deployment)
	if err := g.executeTemplate(templates.ManagerYAMLTemplate, data,
		filepath.Join(managerDir, "manager.yaml")); err != nil {
		return fmt.Errorf("failed to generate manager.yaml: %w", err)
	}

	// Generate config/manager/kustomization.yaml
	if err := g.executeTemplate(templates.KustomizationManagerTemplate, data,
		filepath.Join(managerDir, "kustomization.yaml")); err != nil {
		return fmt.Errorf("failed to generate manager kustomization.yaml: %w", err)
	}

	// Generate config/rbac/kustomization.yaml
	if err := g.executeTemplate(templates.KustomizationRBACTemplate, data,
		filepath.Join(rbacDir, "kustomization.yaml")); err != nil {
		return fmt.Errorf("failed to generate rbac kustomization.yaml: %w", err)
	}

	// Create config directory
	defaultDir := filepath.Join(g.config.OutputDir, "config")
	if err := os.MkdirAll(defaultDir, 0755); err != nil {
		return fmt.Errorf("failed to create default directory: %w", err)
	}

	// Generate config/kustomization.yaml
	if err := g.executeTemplate(templates.KustomizationDefaultTemplate, data,
		filepath.Join(defaultDir, "kustomization.yaml")); err != nil {
		return fmt.Errorf("failed to generate default kustomization.yaml: %w", err)
	}

	return nil
}

func (g *ControllerGenerator) executeTemplate(tmplContent string, data interface{}, outputPath string) error {
	tmpl, err := template.New("yaml").Parse(tmplContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}

// AggregateControllerTemplateData holds data for aggregate controller template
type AggregateControllerTemplateData struct {
	Year             int
	GeneratorVersion string
	APIGroup         string
	APIVersion       string
	ModuleName       string
	Kind             string
	KindLower        string
	Plural           string
	ResourceKinds    []string
}

// GenerateAggregateController generates the aggregate controller
func (g *ControllerGenerator) GenerateAggregateController(aggregate *mapper.AggregateDefinition) error {
	controllerDir := filepath.Join(g.config.OutputDir, "internal", "controller")
	if err := os.MkdirAll(controllerDir, 0755); err != nil {
		return fmt.Errorf("failed to create controller directory: %w", err)
	}

	data := AggregateControllerTemplateData{
		Year:             time.Now().Year(),
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         aggregate.APIGroup,
		APIVersion:       aggregate.APIVersion,
		ModuleName:       g.config.ModuleName,
		Kind:             aggregate.Kind,
		KindLower:        strings.ToLower(aggregate.Kind),
		Plural:           aggregate.Plural,
		ResourceKinds:    aggregate.ResourceKinds,
	}

	filename := fmt.Sprintf("%s_controller.go", strings.ToLower(aggregate.Kind))
	fp := filepath.Join(controllerDir, filename)

	tmpl, err := template.New("aggregate_controller").Funcs(template.FuncMap{
		"lower": strings.ToLower,
	}).Parse(templates.AggregateControllerTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(fp)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// copySpecFile copies the OpenAPI spec file to the output directory.
// If the spec is a URL, it downloads the content. If it's a local file, it copies it.
func (g *ControllerGenerator) copySpecFile() error {
	specPath := g.config.SpecPath
	if specPath == "" {
		// No spec file path configured, skip copy
		return nil
	}
	var destFilename string
	var content []byte

	if strings.HasPrefix(specPath, "http://") || strings.HasPrefix(specPath, "https://") {
		// Download from URL
		parsedURL, err := url.Parse(specPath)
		if err != nil {
			return fmt.Errorf("failed to parse spec URL: %w", err)
		}

		// Get filename from URL path
		destFilename = path.Base(parsedURL.Path)
		if destFilename == "" || destFilename == "/" || destFilename == "." {
			// Use a default name if we can't extract one
			destFilename = "openapi-spec.yaml"
		}

		// Download the file
		resp, err := http.Get(specPath)
		if err != nil {
			return fmt.Errorf("failed to download spec from URL: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to download spec: HTTP %d", resp.StatusCode)
		}

		content, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read spec content: %w", err)
		}
	} else {
		// Copy local file
		destFilename = filepath.Base(specPath)

		var err error
		content, err = os.ReadFile(specPath)
		if err != nil {
			return fmt.Errorf("failed to read spec file: %w", err)
		}
	}

	// Write to output directory
	destPath := filepath.Join(g.config.OutputDir, destFilename)
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write spec file: %w", err)
	}

	return nil
}
