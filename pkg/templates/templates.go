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

// ExampleCRAdoptTemplate is the template for generating example CR YAML files that adopt and modify existing resources
//
//go:embed example_cr_adopt.yaml.tmpl
var ExampleCRAdoptTemplate string

// KustomizationSamplesTemplate is the template for config/samples/kustomization.yaml
//
//go:embed kustomization_samples.yaml.tmpl
var KustomizationSamplesTemplate string

// ReadmeTemplate is the template for generating the README.md file
//
//go:embed readme.md.tmpl
var ReadmeTemplate string

// SuiteTestTemplate is the template for generating the envtest suite_test.go file
//
//go:embed suite_test.go.tmpl
var SuiteTestTemplate string

// IntegrationTestTemplate is the template for generating integration tests with envtest
//
//go:embed integration_test.go.tmpl
var IntegrationTestTemplate string

// AggregateControllerTemplate is the template for generating status aggregator controller
//
//go:embed aggregate_controller.go.tmpl
var AggregateControllerTemplate string

// AggregateTypesTemplate is the template for generating aggregate CRD types
//
//go:embed aggregate_types.go.tmpl
var AggregateTypesTemplate string

// ExampleAggregateCRTemplate is the template for generating example aggregate CR YAML files
//
//go:embed example_aggregate_cr.yaml.tmpl
var ExampleAggregateCRTemplate string

// BundleTypesTemplate is the template for generating bundle CRD types
//
//go:embed bundle_types.go.tmpl
var BundleTypesTemplate string

// BundleControllerTemplate is the template for generating bundle controller
//
//go:embed bundle_controller.go.tmpl
var BundleControllerTemplate string

// ExampleBundleCRTemplate is the template for generating example bundle CR YAML files
//
//go:embed example_bundle_cr.yaml.tmpl
var ExampleBundleCRTemplate string

// CELTestTemplate is the template for generating CEL expression unit tests
//
//go:embed cel_test.go.tmpl
var CELTestTemplate string

// CELTestDataTemplate is the template for generating CEL test data JSON file
//
//go:embed cel_testdata.json.tmpl
var CELTestDataTemplate string

// CELTestDataReadmeTemplate is the template for generating CEL test data README
//
//go:embed cel_testdata_readme.md.tmpl
var CELTestDataReadmeTemplate string

// ExampleResourcesCRTemplate is the template for generating example child resource CRs
// for use with cel-test --resources flag when testing aggregate/bundle expressions
//
//go:embed example_resources_cr.yaml.tmpl
var ExampleResourcesCRTemplate string

// ExampleAggregateWithStatusTemplate is the template for generating an example aggregate CR
// with populated status data for testing CEL expressions
//
//go:embed example_aggregate_with_status.yaml.tmpl
var ExampleAggregateWithStatusTemplate string

// ExampleBundleWithStatusTemplate is the template for generating an example bundle CR
// with populated status data for testing CEL expressions
//
//go:embed example_bundle_with_status.yaml.tmpl
var ExampleBundleWithStatusTemplate string

// ExampleAggregateCRTestdataTemplate is the template for generating an example aggregate CR
// without status data for the testdata directory
//
//go:embed example_aggregate_cr_testdata.yaml.tmpl
var ExampleAggregateCRTestdataTemplate string

// ExampleBundleCRTestdataTemplate is the template for generating an example bundle CR
// without status data for the testdata directory
//
//go:embed example_bundle_cr_testdata.yaml.tmpl
var ExampleBundleCRTestdataTemplate string

// Kubectl Plugin Templates

// KubectlPluginMainTemplate is the template for the kubectl plugin main.go
//
//go:embed kubectl_plugin/main.go.tmpl
var KubectlPluginMainTemplate string

// KubectlPluginRootCmdTemplate is the template for the kubectl plugin root command
//
//go:embed kubectl_plugin/root_cmd.go.tmpl
var KubectlPluginRootCmdTemplate string

// KubectlPluginStatusCmdTemplate is the template for the kubectl plugin status command
//
//go:embed kubectl_plugin/status_cmd.go.tmpl
var KubectlPluginStatusCmdTemplate string

// KubectlPluginGetCmdTemplate is the template for the kubectl plugin get command
//
//go:embed kubectl_plugin/get_cmd.go.tmpl
var KubectlPluginGetCmdTemplate string

// KubectlPluginDescribeCmdTemplate is the template for the kubectl plugin describe command
//
//go:embed kubectl_plugin/describe_cmd.go.tmpl
var KubectlPluginDescribeCmdTemplate string

// KubectlPluginClientTemplate is the template for the kubectl plugin Kubernetes client
//
//go:embed kubectl_plugin/client.go.tmpl
var KubectlPluginClientTemplate string

// KubectlPluginOutputTemplate is the template for the kubectl plugin output formatters
//
//go:embed kubectl_plugin/output.go.tmpl
var KubectlPluginOutputTemplate string

// KubectlPluginGoModTemplate is the template for the kubectl plugin go.mod file
//
//go:embed kubectl_plugin/go.mod.tmpl
var KubectlPluginGoModTemplate string

// KubectlPluginMakefileTemplate is the template for the kubectl plugin Makefile
//
//go:embed kubectl_plugin/makefile.tmpl
var KubectlPluginMakefileTemplate string

// Phase 2: Diagnostic Commands

// KubectlPluginCompareCmdTemplate is the template for the kubectl plugin compare command
//
//go:embed kubectl_plugin/compare_cmd.go.tmpl
var KubectlPluginCompareCmdTemplate string

// KubectlPluginDiagnoseCmdTemplate is the template for the kubectl plugin diagnose command
//
//go:embed kubectl_plugin/diagnose_cmd.go.tmpl
var KubectlPluginDiagnoseCmdTemplate string

// KubectlPluginDriftCmdTemplate is the template for the kubectl plugin drift command
//
//go:embed kubectl_plugin/drift_cmd.go.tmpl
var KubectlPluginDriftCmdTemplate string

// KubectlPluginPauseCmdTemplate is the template for the kubectl plugin pause/unpause commands
//
//go:embed kubectl_plugin/pause_cmd.go.tmpl
var KubectlPluginPauseCmdTemplate string

// KubectlPluginQueryCmdTemplate is the template for the kubectl plugin query command
//
//go:embed kubectl_plugin/query_cmd.go.tmpl
var KubectlPluginQueryCmdTemplate string

// KubectlPluginActionCmdTemplate is the template for the kubectl plugin action command
//
//go:embed kubectl_plugin/action_cmd.go.tmpl
var KubectlPluginActionCmdTemplate string

// KubectlPluginPatchCmdTemplate is the template for the kubectl plugin patch command
//
//go:embed kubectl_plugin/patch_cmd.go.tmpl
var KubectlPluginPatchCmdTemplate string

// KubectlPluginCleanupCmdTemplate is the template for the kubectl plugin cleanup command
//
//go:embed kubectl_plugin/cleanup_cmd.go.tmpl
var KubectlPluginCleanupCmdTemplate string

// KubectlPluginCreateCmdTemplate is the template for the kubectl plugin create command
//
//go:embed kubectl_plugin/create_cmd.go.tmpl
var KubectlPluginCreateCmdTemplate string
