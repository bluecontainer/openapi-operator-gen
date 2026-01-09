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
	content := fmt.Sprintf(`module %s

go 1.25

require (
	github.com/bluecontainer/openapi-operator-gen v0.0.1
	k8s.io/api v0.32.0
	k8s.io/apimachinery v0.32.0
	k8s.io/client-go v0.32.0
	sigs.k8s.io/controller-runtime v0.20.0
)

// For local development, uncomment and adjust the path below:
// replace github.com/bluecontainer/openapi-operator-gen => /path/to/openapi-operator-gen
`, g.config.ModuleName)

	filepath := filepath.Join(g.config.OutputDir, "go.mod")
	return os.WriteFile(filepath, []byte(content), 0644)
}

func (g *ControllerGenerator) generateDockerfile() error {
	content := `# Build stage
FROM golang:1.25 AS builder

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager cmd/manager/main.go

# Runtime stage
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
`

	filepath := filepath.Join(g.config.OutputDir, "Dockerfile")
	return os.WriteFile(filepath, []byte(content), 0644)
}

func (g *ControllerGenerator) generateMakefile() error {
	content := fmt.Sprintf(`# Image URL to use all building/pushing image targets
IMG ?= controller:latest

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.29.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%%-15s\033[0m %%s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate CRD manifests.
	$(CONTROLLER_GEN) crd paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -buildvcs=false -o bin/manager cmd/manager/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/manager/main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

##@ Deployment

.PHONY: install
install: manifests ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl apply -f config/crd/bases/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f config/crd/bases/

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen

## Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.17.0

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	@test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)
`)

	filepath := filepath.Join(g.config.OutputDir, "Makefile")
	return os.WriteFile(filepath, []byte(content), 0644)
}

func (g *ControllerGenerator) generateBoilerplate() error {
	hackDir := filepath.Join(g.config.OutputDir, "hack")
	if err := os.MkdirAll(hackDir, 0755); err != nil {
		return fmt.Errorf("failed to create hack directory: %w", err)
	}

	content := fmt.Sprintf(`/*
Copyright %d Generated by openapi-operator-gen.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
`, time.Now().Year())

	filepath := filepath.Join(hackDir, "boilerplate.go.txt")
	return os.WriteFile(filepath, []byte(content), 0644)
}
