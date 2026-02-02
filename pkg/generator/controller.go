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
	"github.com/bluecontainer/openapi-operator-gen/pkg/aggregate"
	"github.com/bluecontainer/openapi-operator-gen/pkg/mapper"
	"github.com/bluecontainer/openapi-operator-gen/pkg/templates"
	"github.com/iancoleman/strcase"
)

// pluralize converts a Kind name to its lowercase plural form for Kubernetes resource names.
// This is used as a template function to generate proper plural forms (e.g., query -> queries).
func pluralize(kind string) string {
	return aggregate.KindToResourceName(kind)
}

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
	Year               int
	GeneratorVersion   string
	APIGroup           string
	APIVersion         string
	ModuleName         string
	Kind               string
	KindLower          string
	Plural             string
	BasePath           string
	ResourcePath       string                   // Full path template with placeholders (e.g., /classes/{className}/variables/{variableName})
	IsQuery            bool                     // True if this is a query CRD
	QueryPath          string                   // Full query path for query CRDs
	QueryPathParams    []mapper.QueryParamField // Path parameters for query endpoints
	QueryParams        []mapper.QueryParamField // Query parameters for building URL
	ResponseType       string                   // Go type for response (e.g., "[]Pet" or "[]PetFindByTagsResult")
	ResponseIsArray    bool                     // True if response is an array
	ResultItemType     string                   // Item type if ResponseIsArray (e.g., "Pet" or "PetFindByTagsResult")
	HasTypedResults    bool                     // True if we have typed results (not raw extension)
	UsesSharedType     bool                     // True if ResultItemType is a shared type from another CRD
	IsPrimitiveArray   bool                     // True if response is a primitive array ([]string, []int, etc.)
	PrimitiveArrayType string                   // Base type for primitive arrays (e.g., "string", "int64")

	// Action endpoint fields
	IsAction          bool                     // True if this is an action CRD
	ActionPath        string                   // Full action path (e.g., /pet/{petId}/uploadImage)
	ActionMethod      string                   // HTTP method (POST, PUT, or GET)
	ParentResource    string                   // Parent resource kind (e.g., "Pet")
	ParentIDParam     string                   // Parent ID parameter name (e.g., "petId")
	ParentIDField     string                   // Go field name for parent ID (e.g., "PetId")
	ParentIDGoType    string                   // Go type for parent ID (e.g., "int64", "string")
	HasParentID       bool                     // True if the action has a parent ID parameter
	ActionName        string                   // Action name (e.g., "uploadImage")
	PathParams        []ActionPathParam        // Path parameters other than parent ID
	RequestBodyFields []ActionRequestBodyField // Request body fields
	HasRequestBody    bool                     // True if there are request body fields
	HasBinaryBody     bool                     // True if the action accepts binary data uploads
	BinaryContentType string                   // Content type for binary data

	// Resource endpoint fields (for standard CRUD resources)
	ResourcePathParams  []ActionPathParam    // Path parameters for resource endpoints
	ResourceQueryParams []ResourceQueryParam // Query parameters for resource endpoints
	HasResourceParams   bool                 // True if there are path or query params to handle

	// HTTP method availability
	HasDelete bool // True if DELETE method is available for this resource
	HasPost   bool // True if POST method is available for this resource
	HasPut    bool // True if PUT method is available for this resource
	HasPatch  bool // True if PATCH method is available for this resource

	// UpdateWithPost enables using POST for updates when PUT is not available.
	// This is set when --update-with-post flag is used AND HasPut is false AND HasPost is true.
	UpdateWithPost bool

	// Per-method paths (when different methods use different paths)
	GetPath    string // Path for GET operations (e.g., /pet/{petId})
	PutPath    string // Path for PUT operations (e.g., /pet - when ID is in body)
	DeletePath string // Path for DELETE operations (e.g., /pet/{petId})

	// PutPathDiffers is true when PUT uses a different path than GET (e.g., PUT /pet vs GET /pet/{petId})
	PutPathDiffers bool

	// ExternalIDRef handling
	NeedsExternalIDRef bool // True if externalIDRef field is needed (no path params to identify resource)

	// Integration test fields
	RequiredFields    []RequiredFieldInfo // Required fields that need sample values in tests
	HasRequiredFields bool                // True if there are required fields
}

// ActionPathParam represents a path parameter in action templates
type ActionPathParam struct {
	Name      string // Parameter name (e.g., "userId")
	GoName    string // Go field name (e.g., "UserId")
	GoType    string // Go type (e.g., "string", "int64")
	IsPointer bool   // True if this is a pointer type (e.g., *int64)
	BaseType  string // Base type without pointer (e.g., "int64" for "*int64")
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
	APIGroup         string
	ModuleName       string
	AppName          string
	CRDs             []CRDMainData
	HasAggregate     bool   // True if aggregate CRD is generated
	AggregateKind    string // Kind name of the aggregate CRD (e.g., "StatusAggregate")
	HasBundle        bool   // True if bundle CRD is generated
	BundleKind       string // Kind name of the bundle CRD (e.g., "PetstoreBundle")
	// Version info for the generated operator
	OperatorVersion string // Pseudo-version for go.mod (e.g., v0.0.8-0.20260115203556-d5024c8e6620)
	CommitHash      string // Git commit hash (12 chars)
	CommitTimestamp string // Commit timestamp in YYYYMMDDHHMMSS format (UTC)
}

// CRDMainData holds CRD data for main.go
type CRDMainData struct {
	Kind     string
	IsQuery  bool
	IsAction bool
}

// Generate generates controller files
// aggregate and bundle are optional - pass nil if not generating those CRDs
func (g *ControllerGenerator) Generate(crds []*mapper.CRDDefinition, aggregate *mapper.AggregateDefinition, bundle *mapper.BundleDefinition) error {
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

	// Note: controller utility functions (ValuesEqual, GetExternalIDIfPresent, etc.)
	// are now in the shared library github.com/bluecontainer/openapi-operator-gen/pkg/controller

	// Generate main.go (with optional aggregate and bundle info)
	if err := g.generateMain(crds, aggregate, bundle); err != nil {
		return fmt.Errorf("failed to generate main.go: %w", err)
	}

	// Generate go.mod for the generated operator
	if err := g.generateGoMod(aggregate != nil, bundle != nil); err != nil {
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
	if err := g.generateReadme(crds, aggregate != nil, bundle != nil); err != nil {
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
		Year:               time.Now().Year(),
		GeneratorVersion:   g.config.GeneratorVersion,
		APIGroup:           crd.APIGroup,
		APIVersion:         crd.APIVersion,
		ModuleName:         g.config.ModuleName,
		Kind:               crd.Kind,
		KindLower:          strings.ToLower(crd.Kind),
		Plural:             crd.Plural,
		BasePath:           crd.BasePath,
		ResourcePath:       crd.ResourcePath,
		IsQuery:            crd.IsQuery,
		QueryPath:          crd.QueryPath,
		QueryPathParams:    crd.QueryPathParams,
		QueryParams:        crd.QueryParams,
		ResponseType:       crd.ResponseType,
		ResponseIsArray:    crd.ResponseIsArray,
		ResultItemType:     crd.ResultItemType,
		HasTypedResults:    len(crd.ResultFields) > 0 || crd.UsesSharedType || crd.IsPrimitiveArray,
		UsesSharedType:     crd.UsesSharedType,
		IsPrimitiveArray:   crd.IsPrimitiveArray,
		PrimitiveArrayType: crd.PrimitiveArrayType,
		// Action fields
		IsAction:          crd.IsAction,
		ActionPath:        crd.ActionPath,
		ActionMethod:      crd.ActionMethod,
		ParentResource:    crd.ParentResource,
		ParentIDParam:     crd.ParentIDParam,
		ParentIDField:     strcase.ToCamel(crd.ParentIDParam),
		ParentIDGoType:    crd.ParentIDGoType,
		HasParentID:       crd.ParentIDParam != "",
		ActionName:        crd.ActionName,
		HasBinaryBody:     crd.HasBinaryBody,
		BinaryContentType: crd.BinaryContentType,
		// HTTP method availability
		HasDelete:      crd.HasDelete,
		HasPost:        crd.HasPost,
		HasPut:         crd.HasPut,
		HasPatch:       crd.HasPatch,
		UpdateWithPost: crd.UpdateWithPost,
		// Per-method paths
		GetPath:        crd.GetPath,
		PutPath:        crd.PutPath,
		DeletePath:     crd.DeletePath,
		PutPathDiffers: crd.PutPath != "" && crd.GetPath != "" && crd.PutPath != crd.GetPath,
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

		// Build a map of path param -> merged field name from IDFieldMappings
		pathParamToFieldName := make(map[string]string)
		for _, mapping := range crd.IDFieldMappings {
			pathParamToFieldName[mapping.PathParam] = mapping.BodyField
		}

		for _, op := range crd.Operations {
			for _, paramName := range op.PathParams {
				if pathParamsSeen[paramName] {
					continue
				}
				pathParamsSeen[paramName] = true

				// Check if this path param is merged with a body field
				mergedFieldName := pathParamToFieldName[paramName]

				// Find the field in spec to get type info and the correct GoName
				goType := "string" // default
				goName := strcase.ToCamel(paramName)
				isPointer := false
				baseType := goType
				if crd.Spec != nil {
					for _, field := range crd.Spec.Fields {
						// If merged, look for the body field; otherwise look for the path param field
						targetField := paramName
						if mergedFieldName != "" {
							targetField = mergedFieldName
						}
						if strings.EqualFold(field.JSONName, strcase.ToLowerCamel(targetField)) {
							goType = field.GoType
							goName = field.Name // Use the actual field name (e.g., "Id" not "OrderId")

							// Apply the same pointer logic as resolveGoType in types.go:
							// Non-required primitive numeric types become pointers in the generated code
							if !field.Required {
								switch goType {
								case "int", "int32", "int64", "float32", "float64":
									goType = "*" + goType
								}
							}

							// Check if this is a pointer type
							if strings.HasPrefix(goType, "*") {
								isPointer = true
								baseType = strings.TrimPrefix(goType, "*")
							} else {
								baseType = goType
							}
							break
						}
					}
				}
				data.ResourcePathParams = append(data.ResourcePathParams, ActionPathParam{
					Name:      paramName,
					GoName:    goName,
					GoType:    goType,
					IsPointer: isPointer,
					BaseType:  baseType,
				})
			}
			// Only collect query params from the Create operation (POST to base path)
			// Query params from other operations (like updatePetWithForm on /pet/{petId})
			// should not be included in buildResourceURLForCreate
			if op.CRDAction != "Create" {
				continue
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

		// Use the NeedsExternalIDRef value from the CRD (set by mapper based on ResourcePath)
		// This is true when there are no path parameters to identify the resource
		data.NeedsExternalIDRef = crd.NeedsExternalIDRef
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

	funcMap := template.FuncMap{
		"sub": func(a, b int) int {
			return a - b
		},
	}

	tmpl, err := template.New("controller").Funcs(funcMap).Parse(tmplContent)
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
		Year:               time.Now().Year(),
		APIGroup:           crd.APIGroup,
		APIVersion:         crd.APIVersion,
		ModuleName:         g.config.ModuleName,
		Kind:               crd.Kind,
		KindLower:          strings.ToLower(crd.Kind),
		Plural:             crd.Plural,
		BasePath:           crd.BasePath,
		ResourcePath:       crd.ResourcePath,
		IsQuery:            crd.IsQuery,
		QueryPath:          crd.QueryPath,
		QueryPathParams:    crd.QueryPathParams,
		IsAction:           crd.IsAction,
		ActionPath:         crd.ActionPath,
		ActionMethod:       crd.ActionMethod,
		ResponseIsArray:    crd.ResponseIsArray,
		IsPrimitiveArray:   crd.IsPrimitiveArray,
		PrimitiveArrayType: crd.PrimitiveArrayType,

		ParentResource:    crd.ParentResource,
		ParentIDParam:     crd.ParentIDParam,
		ParentIDField:     strcase.ToCamel(crd.ParentIDParam),
		ParentIDGoType:    crd.ParentIDGoType,
		HasParentID:       crd.ParentIDParam != "",
		ActionName:        crd.ActionName,
		HasBinaryBody:     crd.HasBinaryBody,
		BinaryContentType: crd.BinaryContentType,
		HasDelete:         crd.HasDelete,
		HasPost:           crd.HasPost,
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
		ResourcePath:    crd.ResourcePath,
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
		HasDelete:      crd.HasDelete,
		HasPost:        crd.HasPost,

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

func (g *ControllerGenerator) generateMain(crds []*mapper.CRDDefinition, aggregate *mapper.AggregateDefinition, bundle *mapper.BundleDefinition) error {
	cmdDir := filepath.Join(g.config.OutputDir, "cmd", "manager")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		return fmt.Errorf("failed to create cmd directory: %w", err)
	}

	// Build version info for the generated operator
	operatorVersion := g.config.GeneratorVersion
	if !isValidSemver(operatorVersion) {
		operatorVersion = g.buildPseudoVersion()
	}

	// Prepare commit hash (ensure 12 chars)
	commitHash := g.config.CommitHash
	if len(commitHash) > 12 {
		commitHash = commitHash[:12]
	}

	// Normalize timestamp to YYYYMMDDHHMMSS format
	timestamp := normalizeTimestamp(g.config.CommitTimestamp)

	data := MainTemplateData{
		Year:             time.Now().Year(),
		GeneratorVersion: g.config.GeneratorVersion,
		APIVersion:       g.config.APIVersion,
		APIGroup:         g.config.APIGroup,
		ModuleName:       g.config.ModuleName,
		AppName:          strings.Split(g.config.APIGroup, ".")[0],
		CRDs:             make([]CRDMainData, 0, len(crds)),
		OperatorVersion:  operatorVersion,
		CommitHash:       commitHash,
		CommitTimestamp:  timestamp,
	}

	for _, crd := range crds {
		data.CRDs = append(data.CRDs, CRDMainData{Kind: crd.Kind, IsQuery: crd.IsQuery, IsAction: crd.IsAction})
	}

	// Add aggregate info if provided
	if aggregate != nil {
		data.HasAggregate = true
		data.AggregateKind = aggregate.Kind
	}

	// Add bundle info if provided
	if bundle != nil {
		data.HasBundle = true
		data.BundleKind = bundle.Kind
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

func (g *ControllerGenerator) generateGoMod(hasAggregate bool, hasBundle bool) error {
	// Determine the module version to use in go.mod require directive
	// If version is a clean semver (vX.Y.Z), use it as-is
	// Otherwise, construct a proper Go module pseudo-version
	moduleVersion := g.config.GeneratorVersion
	if !isValidSemver(moduleVersion) {
		moduleVersion = g.buildPseudoVersion()
	}

	data := struct {
		ModuleName       string
		GeneratorVersion string // Original generator version for the comment (e.g., v0.0.7-10-gd5024c8-dirty)
		ModuleVersion    string // Valid Go module version for require directive (e.g., v0.0.8-0.20260115203556-d5024c8e6620)
		HasAggregate     bool
		HasBundle        bool
	}{
		ModuleName:       g.config.ModuleName,
		GeneratorVersion: g.config.GeneratorVersion, // Original version for comment
		ModuleVersion:    moduleVersion,             // Pseudo-version for dependency
		HasAggregate:     hasAggregate,
		HasBundle:        hasBundle,
	}
	outputPath := filepath.Join(g.config.OutputDir, "go.mod")
	return g.executeTemplate(templates.GoModTemplate, data, outputPath)
}

// buildPseudoVersion constructs a Go module pseudo-version from config fields.
// Format: vX.Y.(Z+1)-0.YYYYMMDDHHMMSS-COMMIT12
// Example: v0.0.8-0.20260115203556-d5024c8e6620
func (g *ControllerGenerator) buildPseudoVersion() string {
	// Extract base version from GeneratorVersion (e.g., v0.0.7 from v0.0.7-10-gd5024c8-dirty)
	baseVersion := extractBaseSemver(g.config.GeneratorVersion)

	// If we have commit hash and timestamp, build a proper pseudo-version
	commitHash := g.config.CommitHash
	timestamp := g.config.CommitTimestamp

	// Ensure commit hash is at least 12 characters
	if len(commitHash) > 0 && len(timestamp) > 0 {
		// Pad commit hash to 12 chars if needed
		if len(commitHash) < 12 {
			commitHash = commitHash + strings.Repeat("0", 12-len(commitHash))
		} else if len(commitHash) > 12 {
			commitHash = commitHash[:12]
		}

		// Convert timestamp to YYYYMMDDHHMMSS format if needed
		// Input may be ISO format (2026-01-15T20:35:56Z) or already YYYYMMDDHHMMSS
		timestamp = normalizeTimestamp(timestamp)

		// Increment patch version for pseudo-version
		// e.g., v0.0.7 -> v0.0.8
		incrementedVersion := incrementPatchVersion(baseVersion)

		// Format: vX.Y.Z-0.YYYYMMDDHHMMSS-COMMIT12
		return fmt.Sprintf("%s-0.%s-%s", incrementedVersion, timestamp, commitHash)
	}

	// Fallback to base version if no commit info available
	return baseVersion
}

// normalizeTimestamp converts a timestamp to YYYYMMDDHHMMSS format.
// Handles ISO format (2026-01-15T20:35:56Z) or passes through if already in target format.
func normalizeTimestamp(ts string) string {
	// If already in YYYYMMDDHHMMSS format (14 digits), return as-is
	if len(ts) == 14 && isAllDigits(ts) {
		return ts
	}

	// Try parsing ISO format
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		// Try parsing without timezone
		t, err = time.Parse("2006-01-02T15:04:05", ts)
		if err != nil {
			// Return original if can't parse
			return ts
		}
	}

	return t.UTC().Format("20060102150405")
}

// isAllDigits checks if a string contains only digits
func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// incrementPatchVersion increments the patch version of a semver string.
// e.g., v0.0.7 -> v0.0.8, v1.2.3 -> v1.2.4
func incrementPatchVersion(version string) string {
	v := strings.TrimPrefix(version, "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return version
	}

	// Parse and increment patch version
	patch := 0
	for _, c := range parts[2] {
		if c >= '0' && c <= '9' {
			patch = patch*10 + int(c-'0')
		} else {
			break
		}
	}
	patch++

	return fmt.Sprintf("v%s.%s.%d", parts[0], parts[1], patch)
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

// extractBaseSemver extracts the base semantic version from a git describe version.
// For example:
//   - "v0.0.7" -> "v0.0.7"
//   - "v0.0.7-10-gd5024c8-dirty" -> "v0.0.7"
//   - "v0.0.7-dirty" -> "v0.0.7"
//
// Returns "v0.0.0" if no valid base version can be extracted.
func extractBaseSemver(version string) string {
	if version == "" || version == "dev" {
		return "v0.0.0"
	}

	// Remove leading 'v' for processing
	v := strings.TrimPrefix(version, "v")

	// Split on '-' to get the base version part
	// e.g., "0.0.7-10-gd5024c8-dirty" -> ["0.0.7", "10", "gd5024c8", "dirty"]
	parts := strings.Split(v, "-")
	if len(parts) == 0 {
		return "v0.0.0"
	}

	// The first part should be X.Y.Z
	baseParts := strings.Split(parts[0], ".")
	if len(baseParts) != 3 {
		return "v0.0.0"
	}

	// Validate each part is numeric
	for _, part := range baseParts {
		if part == "" {
			return "v0.0.0"
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return "v0.0.0"
			}
		}
	}

	return "v" + parts[0]
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

func (g *ControllerGenerator) generateReadme(crds []*mapper.CRDDefinition, hasAggregate bool, hasBundle bool) error {
	// Build CRD info for template
	type CRDInfo struct {
		Kind               string
		IsQuery            bool
		IsAction           bool
		NeedsExternalIDRef bool
	}
	crdInfos := make([]CRDInfo, 0, len(crds))
	for _, crd := range crds {
		crdInfos = append(crdInfos, CRDInfo{
			Kind:               crd.Kind,
			IsQuery:            crd.IsQuery,
			IsAction:           crd.IsAction,
			NeedsExternalIDRef: crd.NeedsExternalIDRef,
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
	if hasBundle {
		generatorCmd += " \\\n  --bundle"
	}

	data := struct {
		AppName          string
		TitleAppName     string
		APIGroup         string
		APIVersion       string
		CRDs             []CRDInfo
		GeneratorCmd     string
		HasAggregate     bool
		HasBundle        bool
		GeneratorVersion string
	}{
		AppName:          appName,
		TitleAppName:     titleAppName,
		APIGroup:         g.config.APIGroup,
		APIVersion:       g.config.APIVersion,
		CRDs:             crdInfos,
		GeneratorCmd:     generatorCmd,
		HasAggregate:     hasAggregate,
		HasBundle:        hasBundle,
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

	// Generate config/rbac/leader_election_role.yaml
	if err := g.executeTemplate(templates.LeaderElectionRoleTemplate, data,
		filepath.Join(rbacDir, "leader_election_role.yaml")); err != nil {
		return fmt.Errorf("failed to generate leader_election_role.yaml: %w", err)
	}

	// Generate config/rbac/leader_election_role_binding.yaml
	if err := g.executeTemplate(templates.LeaderElectionRoleBindingTemplate, data,
		filepath.Join(rbacDir, "leader_election_role_binding.yaml")); err != nil {
		return fmt.Errorf("failed to generate leader_election_role_binding.yaml: %w", err)
	}

	// Generate config/rbac/plugin_service_account.yaml (for kubectl plugin ephemeral pods)
	if err := g.executeTemplate(templates.PluginServiceAccountTemplate, data,
		filepath.Join(rbacDir, "plugin_service_account.yaml")); err != nil {
		return fmt.Errorf("failed to generate plugin_service_account.yaml: %w", err)
	}

	// Generate config/rbac/plugin_role_binding.yaml
	if err := g.executeTemplate(templates.PluginRoleBindingTemplate, data,
		filepath.Join(rbacDir, "plugin_role_binding.yaml")); err != nil {
		return fmt.Errorf("failed to generate plugin_role_binding.yaml: %w", err)
	}

	// Generate config/rbac/plugin_runner_role.yaml (pod management permissions for kubectl run)
	if err := g.executeTemplate(templates.PluginRunnerRoleTemplate, data,
		filepath.Join(rbacDir, "plugin_runner_role.yaml")); err != nil {
		return fmt.Errorf("failed to generate plugin_runner_role.yaml: %w", err)
	}

	// Generate config/rbac/plugin_runner_role_binding.yaml
	if err := g.executeTemplate(templates.PluginRunnerRoleBindingTemplate, data,
		filepath.Join(rbacDir, "plugin_runner_role_binding.yaml")); err != nil {
		return fmt.Errorf("failed to generate plugin_runner_role_binding.yaml: %w", err)
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

// targetAPITemplateData holds shared template data for target API generation.
type targetAPITemplateData struct {
	GeneratorVersion  string
	AppName           string
	Namespace         string
	TargetAPIImage    string
	HasTargetAPI      bool
	ContainerPort     int
	BasePath          string
	HasKubectlPlugin  bool
	HasRundeckProject bool
}

// resolveTargetAPIData computes the shared template data used by both
// GenerateTargetAPIDeployment and GenerateDockerCompose.
func (g *ControllerGenerator) resolveTargetAPIData() targetAPITemplateData {
	appName := strings.Split(g.config.APIGroup, ".")[0]
	namespace := appName + "-system"

	// Extract base path and port from spec's server URL
	basePath := ""
	containerPort := 8080
	if g.config.SpecBaseURL != "" {
		if parsed, err := url.Parse(g.config.SpecBaseURL); err == nil {
			if parsed.Path != "" && parsed.Path != "/" {
				basePath = strings.TrimSuffix(parsed.Path, "/")
			}
			if parsed.Port() != "" {
				if p, err := fmt.Sscanf(parsed.Port(), "%d", &containerPort); p != 1 || err != nil {
					containerPort = 8080
				}
			}
		}
	}

	// --target-api-port overrides the spec URL port and the default
	if g.config.TargetAPIPort > 0 {
		containerPort = g.config.TargetAPIPort
	}

	return targetAPITemplateData{
		GeneratorVersion:  g.config.GeneratorVersion,
		AppName:           appName,
		Namespace:         namespace,
		TargetAPIImage:    g.config.TargetAPIImage,
		HasTargetAPI:      g.config.TargetAPIImage != "",
		ContainerPort:     containerPort,
		BasePath:          basePath,
		HasKubectlPlugin:  g.config.GenerateKubectlPlugin,
		HasRundeckProject: g.config.GenerateRundeckProject,
	}
}

// GenerateTargetAPIDeployment generates a Deployment+Service manifest for the target REST API.
// This is only called when --target-api-image is provided.
func (g *ControllerGenerator) GenerateTargetAPIDeployment() error {
	data := g.resolveTargetAPIData()

	targetAPIDir := filepath.Join(g.config.OutputDir, "config", "target-api")
	if err := os.MkdirAll(targetAPIDir, 0755); err != nil {
		return fmt.Errorf("failed to create target-api directory: %w", err)
	}

	return g.executeTemplate(templates.TargetAPIDeploymentTemplate, data,
		filepath.Join(targetAPIDir, "deployment.yaml"))
}

// GenerateDockerCompose generates a docker-compose.yaml for local development and testing.
// Always generated; target API services are conditionally included when --target-api-image is set.
func (g *ControllerGenerator) GenerateDockerCompose() error {
	data := g.resolveTargetAPIData()

	return g.executeTemplate(templates.DockerComposeTemplate, data,
		filepath.Join(g.config.OutputDir, "docker-compose.yaml"))
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
	ResourceKinds    []string // CRUD resource kinds
	QueryKinds       []string // Query CRD kinds
	ActionKinds      []string // Action CRD kinds
	AllKinds         []string // All kinds combined
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
		QueryKinds:       aggregate.QueryKinds,
		ActionKinds:      aggregate.ActionKinds,
		AllKinds:         aggregate.AllKinds,
	}

	filename := fmt.Sprintf("%s_controller.go", strings.ToLower(aggregate.Kind))
	fp := filepath.Join(controllerDir, filename)

	tmpl, err := template.New("aggregate_controller").Funcs(template.FuncMap{
		"lower":     strings.ToLower,
		"pluralize": pluralize,
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

// BundleControllerTemplateData holds data for bundle controller template
type BundleControllerTemplateData struct {
	Year             int
	GeneratorVersion string
	APIGroup         string
	APIVersion       string
	ModuleName       string
	Kind             string
	KindLower        string
	Plural           string
	ResourceKinds    []string // CRUD resource kinds
	QueryKinds       []string // Query CRD kinds
	ActionKinds      []string // Action CRD kinds
	AllKinds         []string // All kinds combined
}

// GenerateBundleController generates the bundle controller
func (g *ControllerGenerator) GenerateBundleController(bundle *mapper.BundleDefinition) error {
	controllerDir := filepath.Join(g.config.OutputDir, "internal", "controller")
	if err := os.MkdirAll(controllerDir, 0755); err != nil {
		return fmt.Errorf("failed to create controller directory: %w", err)
	}

	data := BundleControllerTemplateData{
		Year:             time.Now().Year(),
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         bundle.APIGroup,
		APIVersion:       bundle.APIVersion,
		ModuleName:       g.config.ModuleName,
		Kind:             bundle.Kind,
		KindLower:        strings.ToLower(bundle.Kind),
		Plural:           bundle.Plural,
		ResourceKinds:    bundle.ResourceKinds,
		QueryKinds:       bundle.QueryKinds,
		ActionKinds:      bundle.ActionKinds,
		AllKinds:         bundle.AllKinds,
	}

	filename := fmt.Sprintf("%s_controller.go", strings.ToLower(bundle.Kind))
	fp := filepath.Join(controllerDir, filename)

	tmpl, err := template.New("bundle_controller").Funcs(template.FuncMap{
		"lower":     strings.ToLower,
		"pluralize": pluralize,
	}).Parse(templates.BundleControllerTemplate)
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

// CELTestTemplateData holds data for the CEL test template
type CELTestTemplateData struct {
	Year             int
	GeneratorVersion string
	AllKinds         []string
}

// GenerateCELTest generates the CEL expression unit test file
func (g *ControllerGenerator) GenerateCELTest(allKinds []string) error {
	controllerDir := filepath.Join(g.config.OutputDir, "internal", "controller")
	if err := os.MkdirAll(controllerDir, 0755); err != nil {
		return fmt.Errorf("failed to create controller directory: %w", err)
	}

	data := CELTestTemplateData{
		Year:             time.Now().Year(),
		GeneratorVersion: g.config.GeneratorVersion,
		AllKinds:         allKinds,
	}

	fp := filepath.Join(controllerDir, "cel_test.go")

	tmpl, err := template.New("cel_test").Funcs(template.FuncMap{
		"lower":     strings.ToLower,
		"pluralize": pluralize,
	}).Parse(templates.CELTestTemplate)
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

// CELTestDataTemplateData holds data for the CEL test data templates
type CELTestDataTemplateData struct {
	AppName       string
	APIGroup      string
	APIVersion    string
	AggregateKind string
	BundleKind    string
	ResourceKinds []string
	QueryKinds    []string
	ActionKinds   []string
	AllKinds      []string
}

// GenerateCELTestData generates the CEL test data JSON file and README
func (g *ControllerGenerator) GenerateCELTestData(resourceKinds, queryKinds, actionKinds, allKinds []string, aggregateKind, bundleKind string, crds []*mapper.CRDDefinition) error {
	testdataDir := filepath.Join(g.config.OutputDir, "testdata")
	if err := os.MkdirAll(testdataDir, 0755); err != nil {
		return fmt.Errorf("failed to create testdata directory: %w", err)
	}

	appName := strings.Split(g.config.APIGroup, ".")[0]

	data := CELTestDataTemplateData{
		AppName:       appName,
		APIGroup:      g.config.APIGroup,
		APIVersion:    g.config.APIVersion,
		AggregateKind: aggregateKind,
		BundleKind:    bundleKind,
		ResourceKinds: resourceKinds,
		QueryKinds:    queryKinds,
		ActionKinds:   actionKinds,
		AllKinds:      allKinds,
	}

	funcMap := template.FuncMap{
		"lower":     strings.ToLower,
		"pluralize": pluralize,
		"add": func(a, b int) int {
			return a + b
		},
	}

	// Generate cel-test-data.json
	jsonTmpl, err := template.New("cel_testdata").Funcs(funcMap).Parse(templates.CELTestDataTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse CEL test data template: %w", err)
	}

	jsonFile, err := os.Create(filepath.Join(testdataDir, "cel-test-data.json"))
	if err != nil {
		return fmt.Errorf("failed to create cel-test-data.json: %w", err)
	}
	defer jsonFile.Close()

	if err := jsonTmpl.Execute(jsonFile, data); err != nil {
		return fmt.Errorf("failed to execute CEL test data template: %w", err)
	}

	// Generate README.md
	readmeTmpl, err := template.New("cel_testdata_readme").Funcs(funcMap).Parse(templates.CELTestDataReadmeTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse CEL test data README template: %w", err)
	}

	readmeFile, err := os.Create(filepath.Join(testdataDir, "README.md"))
	if err != nil {
		return fmt.Errorf("failed to create testdata README.md: %w", err)
	}
	defer readmeFile.Close()

	if err := readmeTmpl.Execute(readmeFile, data); err != nil {
		return fmt.Errorf("failed to execute CEL test data README template: %w", err)
	}

	// Generate resources.yaml with example child CRs for cel-test --resources
	if aggregateKind != "" || bundleKind != "" {
		parentKind := aggregateKind
		if parentKind == "" {
			parentKind = bundleKind
		}
		if err := g.generateExampleResourcesCRs(testdataDir, crds, parentKind); err != nil {
			return fmt.Errorf("failed to generate example resources CRs: %w", err)
		}
	}

	// Generate aggregate CRs for testing CEL expressions
	if aggregateKind != "" {
		// With status data
		if err := g.generateAggregateWithStatus(testdataDir, aggregateKind, resourceKinds); err != nil {
			return fmt.Errorf("failed to generate aggregate-with-status.yaml: %w", err)
		}
		// Without status data (spec only)
		if err := g.generateAggregateCRTestdata(testdataDir, aggregateKind, resourceKinds); err != nil {
			return fmt.Errorf("failed to generate aggregate.yaml: %w", err)
		}
	}

	// Generate bundle CRs for testing CEL expressions
	if bundleKind != "" {
		// With status data
		if err := g.generateBundleWithStatus(testdataDir, bundleKind, resourceKinds); err != nil {
			return fmt.Errorf("failed to generate bundle-with-status.yaml: %w", err)
		}
		// Without status data (spec only)
		if err := g.generateBundleCRTestdata(testdataDir, bundleKind, resourceKinds); err != nil {
			return fmt.Errorf("failed to generate bundle.yaml: %w", err)
		}
	}

	return nil
}

// resourceFieldData holds field data for example resource CRs
type resourceFieldData struct {
	JSONName      string
	ExampleValue1 string
	ExampleValue2 string
}

// resourceData holds data for a single resource kind in resources.yaml
type resourceData struct {
	APIGroup   string
	APIVersion string
	Kind       string
	NameLower  string
	SpecFields []resourceFieldData
}

// generateExampleResourcesCRs generates example child resource CRs for use with cel-test --resources
func (g *ControllerGenerator) generateExampleResourcesCRs(testdataDir string, crds []*mapper.CRDDefinition, aggregateKind string) error {
	// Filter to only CRUD resources (not queries or actions)
	var resourceCRDs []*mapper.CRDDefinition
	for _, crd := range crds {
		if !crd.IsQuery && !crd.IsAction {
			resourceCRDs = append(resourceCRDs, crd)
		}
	}

	if len(resourceCRDs) == 0 {
		return nil
	}

	// Build resource data for the template
	resources := make([]resourceData, 0, len(resourceCRDs))
	for _, crd := range resourceCRDs {
		resource := resourceData{
			APIGroup:   crd.APIGroup,
			APIVersion: crd.APIVersion,
			Kind:       crd.Kind,
			NameLower:  strings.ToLower(crd.Kind),
			SpecFields: g.convertToResourceExampleFields(crd.Spec),
		}
		resources = append(resources, resource)
	}

	data := struct {
		GeneratorVersion string
		AggregateKind    string
		Resources        []resourceData
	}{
		GeneratorVersion: g.config.GeneratorVersion,
		AggregateKind:    aggregateKind,
		Resources:        resources,
	}

	tmpl, err := template.New("example-resources").Parse(templates.ExampleResourcesCRTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	examplePath := filepath.Join(testdataDir, "resources.yaml")
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

// convertToResourceExampleFields converts CRD spec fields to example field data with two example values
func (g *ControllerGenerator) convertToResourceExampleFields(spec *mapper.FieldDefinition) []resourceFieldData {
	if spec == nil {
		return nil
	}

	targetingFields := map[string]bool{
		"targetNamespace":   true,
		"targetStatefulSet": true,
		"targetDeployment":  true,
		"targetHelmRelease": true,
		"targetPodOrdinal":  true,
	}

	result := make([]resourceFieldData, 0, len(spec.Fields))

	for _, f := range spec.Fields {
		// Skip targeting fields - they're not relevant for resource CRs
		if targetingFields[f.JSONName] {
			continue
		}

		result = append(result, resourceFieldData{
			JSONName:      f.JSONName,
			ExampleValue1: g.generateExampleValueWithSeed(f, 1),
			ExampleValue2: g.generateExampleValueWithSeed(f, 2),
		})
	}
	return result
}

// generateExampleValueWithSeed generates an example value for a field using a seed for variation
func (g *ControllerGenerator) generateExampleValueWithSeed(f *mapper.FieldDefinition, seed int) string {
	// If there's an enum, use different values for different seeds if available
	if len(f.Enum) > 0 {
		idx := (seed - 1) % len(f.Enum)
		return fmt.Sprintf("%q", f.Enum[idx])
	}

	// Remove pointer prefix
	goType := strings.TrimPrefix(f.GoType, "*")

	switch goType {
	case "string":
		return fmt.Sprintf("%q", fmt.Sprintf("example-%s-%d", f.JSONName, seed))
	case "int", "int32", "int64":
		return fmt.Sprintf("%d", seed*100+seed)
	case "float32", "float64":
		return fmt.Sprintf("%.1f", float64(seed)*10.5)
	case "bool":
		return fmt.Sprintf("%t", seed%2 == 1)
	case "[]string":
		return fmt.Sprintf("[%q]", fmt.Sprintf("item%d", seed))
	case "[]int", "[]int32", "[]int64":
		return fmt.Sprintf("[%d, %d, %d]", seed, seed+1, seed+2)
	case "metav1.Time":
		// Generate a valid RFC 3339 timestamp with seed offset
		t := time.Now().UTC().AddDate(0, 0, seed)
		return fmt.Sprintf("%q", t.Format(time.RFC3339))
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

// generateAggregateWithStatus generates an example aggregate CR with populated status for CEL testing
func (g *ControllerGenerator) generateAggregateWithStatus(testdataDir, aggregateKind string, resourceKinds []string) error {
	appName := strings.Split(g.config.APIGroup, ".")[0]

	// Calculate total resources (6 per kind as generated in resources.yaml)
	totalResources := len(resourceKinds) * 6

	data := struct {
		GeneratorVersion string
		APIGroup         string
		APIVersion       string
		AppName          string
		AggregateKind    string
		ResourceKinds    []string
		TotalResources   int
		Timestamp        string
	}{
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         g.config.APIGroup,
		APIVersion:       g.config.APIVersion,
		AppName:          appName,
		AggregateKind:    aggregateKind,
		ResourceKinds:    resourceKinds,
		TotalResources:   totalResources,
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
	}

	funcMap := template.FuncMap{
		"lower": strings.ToLower,
		"add": func(a, b int) int {
			return a + b
		},
	}

	tmpl, err := template.New("aggregate-with-status").Funcs(funcMap).Parse(templates.ExampleAggregateWithStatusTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	filePath := filepath.Join(testdataDir, "aggregate-with-status.yaml")
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// generateBundleWithStatus generates an example bundle CR with populated status for CEL testing
func (g *ControllerGenerator) generateBundleWithStatus(testdataDir, bundleKind string, resourceKinds []string) error {
	appName := strings.Split(g.config.APIGroup, ".")[0]

	data := struct {
		GeneratorVersion string
		APIGroup         string
		APIVersion       string
		AppName          string
		BundleKind       string
		ResourceKinds    []string
		Timestamp        string
	}{
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         g.config.APIGroup,
		APIVersion:       g.config.APIVersion,
		AppName:          appName,
		BundleKind:       bundleKind,
		ResourceKinds:    resourceKinds,
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
	}

	funcMap := template.FuncMap{
		"lower": strings.ToLower,
	}

	tmpl, err := template.New("bundle-with-status").Funcs(funcMap).Parse(templates.ExampleBundleWithStatusTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	filePath := filepath.Join(testdataDir, "bundle-with-status.yaml")
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// generateAggregateCRTestdata generates an example aggregate CR without status for testdata
func (g *ControllerGenerator) generateAggregateCRTestdata(testdataDir, aggregateKind string, resourceKinds []string) error {
	appName := strings.Split(g.config.APIGroup, ".")[0]

	data := struct {
		GeneratorVersion string
		APIGroup         string
		APIVersion       string
		AppName          string
		AggregateKind    string
		ResourceKinds    []string
	}{
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         g.config.APIGroup,
		APIVersion:       g.config.APIVersion,
		AppName:          appName,
		AggregateKind:    aggregateKind,
		ResourceKinds:    resourceKinds,
	}

	funcMap := template.FuncMap{
		"lower": strings.ToLower,
	}

	tmpl, err := template.New("aggregate-testdata").Funcs(funcMap).Parse(templates.ExampleAggregateCRTestdataTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	filePath := filepath.Join(testdataDir, "aggregate.yaml")
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// generateBundleCRTestdata generates an example bundle CR without status for testdata
func (g *ControllerGenerator) generateBundleCRTestdata(testdataDir, bundleKind string, resourceKinds []string) error {
	appName := strings.Split(g.config.APIGroup, ".")[0]

	data := struct {
		GeneratorVersion string
		APIGroup         string
		APIVersion       string
		AppName          string
		BundleKind       string
		ResourceKinds    []string
	}{
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         g.config.APIGroup,
		APIVersion:       g.config.APIVersion,
		AppName:          appName,
		BundleKind:       bundleKind,
		ResourceKinds:    resourceKinds,
	}

	funcMap := template.FuncMap{
		"lower": strings.ToLower,
	}

	tmpl, err := template.New("bundle-testdata").Funcs(funcMap).Parse(templates.ExampleBundleCRTestdataTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	filePath := filepath.Join(testdataDir, "bundle.yaml")
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
