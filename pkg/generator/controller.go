package generator

import (
	"bytes"
	"fmt"
	"os"
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
	Year            int
	APIGroup        string
	APIVersion      string
	ModuleName      string
	Kind            string
	KindLower       string
	Plural          string
	BasePath        string
	IsQuery         bool                     // True if this is a query CRD
	QueryPath       string                   // Full query path for query CRDs
	QueryParams     []mapper.QueryParamField // Query parameters for building URL
	ResponseType    string                   // Go type for response (e.g., "[]Pet" or "[]PetFindByTagsResult")
	ResponseIsArray bool                     // True if response is an array
	ResultItemType  string                   // Item type if ResponseIsArray (e.g., "Pet" or "PetFindByTagsResult")
	HasTypedResults bool                     // True if we have typed results (not raw extension)
	UsesSharedType  bool                     // True if ResultItemType is a shared type from another CRD

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

// MainTemplateData holds data for main.go template
type MainTemplateData struct {
	Year       int
	APIVersion string
	ModuleName string
	CRDs       []CRDMainData
}

// CRDMainData holds CRD data for main.go
type CRDMainData struct {
	Kind     string
	IsQuery  bool
	IsAction bool
}

// Generate generates controller files
func (g *ControllerGenerator) Generate(crds []*mapper.CRDDefinition) error {
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
	}

	// Generate main.go
	if err := g.generateMain(crds); err != nil {
		return fmt.Errorf("failed to generate main.go: %w", err)
	}

	// Generate go.mod for the generated operator
	if err := g.generateGoMod(); err != nil {
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

	// Generate hack/boilerplate.go.txt for controller-gen
	if err := g.generateBoilerplate(); err != nil {
		return fmt.Errorf("failed to generate boilerplate: %w", err)
	}

	// Generate deployment manifests (namespace, service account, deployment, role binding)
	if err := g.generateDeploymentManifests(); err != nil {
		return fmt.Errorf("failed to generate deployment manifests: %w", err)
	}

	return nil
}

func (g *ControllerGenerator) generateController(outputDir string, crd *mapper.CRDDefinition) error {
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
		QueryParams:     crd.QueryParams,
		ResponseType:    crd.ResponseType,
		ResponseIsArray: crd.ResponseIsArray,
		ResultItemType:  crd.ResultItemType,
		HasTypedResults: len(crd.ResultFields) > 0 || crd.UsesSharedType,
		UsesSharedType:  crd.UsesSharedType,
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

func (g *ControllerGenerator) generateMain(crds []*mapper.CRDDefinition) error {
	cmdDir := filepath.Join(g.config.OutputDir, "cmd", "manager")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		return fmt.Errorf("failed to create cmd directory: %w", err)
	}

	data := MainTemplateData{
		Year:       time.Now().Year(),
		APIVersion: g.config.APIVersion,
		ModuleName: g.config.ModuleName,
		CRDs:       make([]CRDMainData, 0, len(crds)),
	}

	for _, crd := range crds {
		data.CRDs = append(data.CRDs, CRDMainData{Kind: crd.Kind, IsQuery: crd.IsQuery, IsAction: crd.IsAction})
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

func (g *ControllerGenerator) generateGoMod() error {
	data := struct {
		ModuleName string
	}{
		ModuleName: g.config.ModuleName,
	}
	outputPath := filepath.Join(g.config.OutputDir, "go.mod")
	return g.executeTemplate(templates.GoModTemplate, data, outputPath)
}

func (g *ControllerGenerator) generateDockerfile() error {
	outputPath := filepath.Join(g.config.OutputDir, "Dockerfile")
	return g.executeTemplate(templates.DockerfileTemplate, nil, outputPath)
}

func (g *ControllerGenerator) generateMakefile() error {
	data := struct {
		AppName string
	}{
		AppName: strings.Split(g.config.APIGroup, ".")[0],
	}
	outputPath := filepath.Join(g.config.OutputDir, "Makefile")
	return g.executeTemplate(templates.MakefileTemplate, data, outputPath)
}

func (g *ControllerGenerator) generateBoilerplate() error {
	hackDir := filepath.Join(g.config.OutputDir, "hack")
	if err := os.MkdirAll(hackDir, 0755); err != nil {
		return fmt.Errorf("failed to create hack directory: %w", err)
	}

	data := struct {
		Year int
	}{
		Year: time.Now().Year(),
	}
	outputPath := filepath.Join(hackDir, "boilerplate.go.txt")
	return g.executeTemplate(templates.BoilerplateTemplate, data, outputPath)
}

// DeploymentManifestData holds data for generating deployment YAML manifests
type DeploymentManifestData struct {
	Namespace string
	AppName   string
}

func (g *ControllerGenerator) generateDeploymentManifests() error {
	// Derive namespace from API group (e.g., petstore.example.com -> petstore-system)
	data := DeploymentManifestData{
		Namespace: strings.Split(g.config.APIGroup, ".")[0] + "-system",
		AppName:   strings.Split(g.config.APIGroup, ".")[0],
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
