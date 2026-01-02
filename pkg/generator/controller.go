package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/example/openapi-operator-gen/internal/config"
	"github.com/example/openapi-operator-gen/pkg/mapper"
	"github.com/example/openapi-operator-gen/pkg/templates"
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
	Year       int
	APIGroup   string
	APIVersion string
	ModuleName string
	Kind       string
	KindLower  string
	Plural     string
	BasePath   string
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
	Kind string
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

	return nil
}

func (g *ControllerGenerator) generateController(outputDir string, crd *mapper.CRDDefinition) error {
	data := ControllerTemplateData{
		Year:       time.Now().Year(),
		APIGroup:   crd.APIGroup,
		APIVersion: crd.APIVersion,
		ModuleName: g.config.ModuleName,
		Kind:       crd.Kind,
		KindLower:  strings.ToLower(crd.Kind),
		Plural:     crd.Plural,
		BasePath:   crd.BasePath,
	}

	filename := fmt.Sprintf("%s_controller.go", strings.ToLower(crd.Kind))
	filepath := filepath.Join(outputDir, filename)

	tmpl, err := template.New("controller").Parse(templates.ControllerTemplate)
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
		data.CRDs = append(data.CRDs, CRDMainData{Kind: crd.Kind})
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

go 1.21

require (
	github.com/example/openapi-operator-gen v0.0.0
	k8s.io/api v0.29.0
	k8s.io/apimachinery v0.29.0
	k8s.io/client-go v0.29.0
	sigs.k8s.io/controller-runtime v0.17.0
)

// For local development, uncomment and adjust the path below:
// replace github.com/example/openapi-operator-gen => /path/to/openapi-operator-gen
`, g.config.ModuleName)

	filepath := filepath.Join(g.config.OutputDir, "go.mod")
	return os.WriteFile(filepath, []byte(content), 0644)
}

func (g *ControllerGenerator) generateDockerfile() error {
	content := `# Build stage
FROM golang:1.21-alpine AS builder

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

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%%-15s\033[0m %%s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: fmt vet ## Build manager binary.
	go build -o bin/manager cmd/manager/main.go

.PHONY: run
run: fmt vet ## Run a controller from your host.
	go run ./cmd/manager/main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

##@ Deployment

.PHONY: install
install: ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl apply -f config/crd/bases/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f config/crd/bases/

.PHONY: generate
generate: ## Generate code (deep copy, etc.)
	go generate ./...
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
	controller-gen crd paths="./..." output:crd:artifacts:config=config/crd/bases

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0)

define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(shell pwd)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef
`)

	filepath := filepath.Join(g.config.OutputDir, "Makefile")
	return os.WriteFile(filepath, []byte(content), 0644)
}
