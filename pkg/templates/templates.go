package templates

import (
	_ "embed"
)

// TypesTemplate is the template for generating CRD types
//
//go:embed types.go.tmpl
var TypesTemplate string

// GroupVersionInfoTemplate is the template for groupversion_info.go
//
//go:embed groupversion_info.go.tmpl
var GroupVersionInfoTemplate string

// ControllerTemplate is the template for generating controller reconciliation logic
//
//go:embed controller.go.tmpl
var ControllerTemplate string

// QueryControllerTemplate is the template for generating query-only controller reconciliation logic
//
//go:embed query_controller.go.tmpl
var QueryControllerTemplate string

// ActionControllerTemplate is the template for generating action controller reconciliation logic
//
//go:embed action_controller.go.tmpl
var ActionControllerTemplate string

// CRDYAMLTemplate is the template for generating CRD YAML manifests
//
//go:embed crd.yaml.tmpl
var CRDYAMLTemplate string

// MainTemplate is the template for the main.go of generated operator
//
//go:embed main.go.tmpl
var MainTemplate string

// ControllerTestTemplate is the template for generating controller test files
//
//go:embed controller_test.go.tmpl
var ControllerTestTemplate string

// NamespaceYAMLTemplate is the template for generating namespace.yaml
//
//go:embed namespace.yaml.tmpl
var NamespaceYAMLTemplate string

// ServiceAccountYAMLTemplate is the template for generating service_account.yaml
//
//go:embed service_account.yaml.tmpl
var ServiceAccountYAMLTemplate string

// RoleBindingYAMLTemplate is the template for generating role_binding.yaml
//
//go:embed role_binding.yaml.tmpl
var RoleBindingYAMLTemplate string

// ManagerYAMLTemplate is the template for generating manager.yaml (Deployment)
//
//go:embed manager.yaml.tmpl
var ManagerYAMLTemplate string

// KustomizationManagerTemplate is the template for config/manager/kustomization.yaml
//
//go:embed kustomization_manager.yaml.tmpl
var KustomizationManagerTemplate string

// KustomizationRBACTemplate is the template for config/rbac/kustomization.yaml
//
//go:embed kustomization_rbac.yaml.tmpl
var KustomizationRBACTemplate string

// KustomizationCRDTemplate is the template for config/crd/bases/kustomization.yaml
//
//go:embed kustomization_crd.yaml.tmpl
var KustomizationCRDTemplate string

// KustomizationDefaultTemplate is the template for config/default/kustomization.yaml
//
//go:embed kustomization_default.yaml.tmpl
var KustomizationDefaultTemplate string

// DockerfileTemplate is the template for generating the Dockerfile
//
//go:embed dockerfile.tmpl
var DockerfileTemplate string

// MakefileTemplate is the template for generating the Makefile
//
//go:embed makefile.tmpl
var MakefileTemplate string

// GoModTemplate is the template for generating the go.mod file
//
//go:embed go.mod.tmpl
var GoModTemplate string

// BoilerplateTemplate is the template for generating hack/boilerplate.go.txt
//
//go:embed boilerplate.go.txt.tmpl
var BoilerplateTemplate string

// ExampleCRTemplate is the template for generating example CR YAML files
//
//go:embed example_cr.yaml.tmpl
var ExampleCRTemplate string

// ExampleCRRefTemplate is the template for generating example CR YAML files with externalIDRef
//
//go:embed example_cr_ref.yaml.tmpl
var ExampleCRRefTemplate string

// KustomizationSamplesTemplate is the template for config/samples/kustomization.yaml
//
//go:embed kustomization_samples.yaml.tmpl
var KustomizationSamplesTemplate string
