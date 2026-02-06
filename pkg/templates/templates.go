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

// LeaderElectionRoleTemplate is the template for generating leader_election_role.yaml
//
//go:embed leader_election_role.yaml.tmpl
var LeaderElectionRoleTemplate string

// LeaderElectionRoleBindingTemplate is the template for generating leader_election_role_binding.yaml
//
//go:embed leader_election_role_binding.yaml.tmpl
var LeaderElectionRoleBindingTemplate string

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

// KubectlPluginTargetingTemplate is the template for the kubectl plugin shared targeting helpers
//
//go:embed kubectl_plugin/targeting.go.tmpl
var KubectlPluginTargetingTemplate string

// KubectlPluginNodesCmdTemplate is the template for the kubectl plugin nodes command (Rundeck resource model)
//
//go:embed kubectl_plugin/nodes_cmd.go.tmpl
var KubectlPluginNodesCmdTemplate string

// TargetAPIDeploymentTemplate is the template for the target API Deployment+Service
//
//go:embed target_api_deployment.yaml.tmpl
var TargetAPIDeploymentTemplate string

// DockerComposeTemplate is the template for docker-compose.yaml development environment
//
//go:embed docker_compose.yaml.tmpl
var DockerComposeTemplate string

// Rundeck Project Templates

// RundeckProjectPropertiesTemplate is the template for Rundeck project.properties
//
//go:embed rundeck/project_properties.tmpl
var RundeckProjectPropertiesTemplate string

// RundeckNodeSourceTemplate is the template for the node source discovery script (native execution)
//
//go:embed rundeck/node_source.sh.tmpl
var RundeckNodeSourceTemplate string

// RundeckResourceCreateJobTemplate is the template for per-resource create jobs
//
//go:embed rundeck/resource_create_job.yaml.tmpl
var RundeckResourceCreateJobTemplate string

// RundeckResourceGetJobTemplate is the template for per-resource list/get jobs
//
//go:embed rundeck/resource_get_job.yaml.tmpl
var RundeckResourceGetJobTemplate string

// RundeckResourceDescribeJobTemplate is the template for per-resource describe jobs
//
//go:embed rundeck/resource_describe_job.yaml.tmpl
var RundeckResourceDescribeJobTemplate string

// RundeckQueryJobTemplate is the template for per-query jobs
//
//go:embed rundeck/query_job.yaml.tmpl
var RundeckQueryJobTemplate string

// RundeckActionJobTemplate is the template for per-action jobs
//
//go:embed rundeck/action_job.yaml.tmpl
var RundeckActionJobTemplate string

// RundeckStatusJobTemplate is the template for the operator status job
//
//go:embed rundeck/status_job.yaml.tmpl
var RundeckStatusJobTemplate string

// RundeckDriftJobTemplate is the template for the drift detection job
//
//go:embed rundeck/drift_job.yaml.tmpl
var RundeckDriftJobTemplate string

// RundeckCleanupJobTemplate is the template for the cleanup job
//
//go:embed rundeck/cleanup_job.yaml.tmpl
var RundeckCleanupJobTemplate string

// Rundeck Docker Project Templates (docker run execution)

// RundeckDockerProjectPropertiesTemplate is the template for Docker Rundeck project.properties
//
//go:embed rundeck_docker/project_properties.tmpl
var RundeckDockerProjectPropertiesTemplate string

// RundeckDockerNodeSourceTemplate is the template for the node source discovery script (Docker execution)
//
//go:embed rundeck_docker/node_source.sh.tmpl
var RundeckDockerNodeSourceTemplate string

// RundeckDockerResourceCreateJobTemplate is the template for per-resource create jobs (Docker execution)
//
//go:embed rundeck_docker/resource_create_job.yaml.tmpl
var RundeckDockerResourceCreateJobTemplate string

// RundeckDockerResourceGetJobTemplate is the template for per-resource list/get jobs (Docker execution)
//
//go:embed rundeck_docker/resource_get_job.yaml.tmpl
var RundeckDockerResourceGetJobTemplate string

// RundeckDockerResourceDescribeJobTemplate is the template for per-resource describe jobs (Docker execution)
//
//go:embed rundeck_docker/resource_describe_job.yaml.tmpl
var RundeckDockerResourceDescribeJobTemplate string

// RundeckDockerQueryJobTemplate is the template for per-query jobs (Docker execution)
//
//go:embed rundeck_docker/query_job.yaml.tmpl
var RundeckDockerQueryJobTemplate string

// RundeckDockerActionJobTemplate is the template for per-action jobs (Docker execution)
//
//go:embed rundeck_docker/action_job.yaml.tmpl
var RundeckDockerActionJobTemplate string

// RundeckDockerStatusJobTemplate is the template for the operator status job (Docker execution)
//
//go:embed rundeck_docker/status_job.yaml.tmpl
var RundeckDockerStatusJobTemplate string

// RundeckDockerDriftJobTemplate is the template for the drift detection job (Docker execution)
//
//go:embed rundeck_docker/drift_job.yaml.tmpl
var RundeckDockerDriftJobTemplate string

// RundeckDockerCleanupJobTemplate is the template for the cleanup job (Docker execution)
//
//go:embed rundeck_docker/cleanup_job.yaml.tmpl
var RundeckDockerCleanupJobTemplate string

// Plugin RBAC Templates (for kubectl plugin ephemeral pod execution)

// PluginServiceAccountTemplate is the template for the plugin-runner ServiceAccount
//
//go:embed plugin_service_account.yaml.tmpl
var PluginServiceAccountTemplate string

// PluginRoleBindingTemplate is the template for the plugin-runner ClusterRoleBinding
//
//go:embed plugin_role_binding.yaml.tmpl
var PluginRoleBindingTemplate string

// PluginRunnerRoleTemplate is the template for the plugin-runner ClusterRole (pod management permissions)
//
//go:embed plugin_runner_role.yaml.tmpl
var PluginRunnerRoleTemplate string

// PluginRunnerRoleBindingTemplate is the template for the plugin-runner extra ClusterRoleBinding
//
//go:embed plugin_runner_role_binding.yaml.tmpl
var PluginRunnerRoleBindingTemplate string

// Rundeck Kubernetes Execution Project Templates (kubectl run execution)

// RundeckK8sProjectPropertiesTemplate is the template for K8s Rundeck project.properties
//
//go:embed rundeck_k8s/project_properties.tmpl
var RundeckK8sProjectPropertiesTemplate string

// RundeckK8sNodeSourceTemplate is the template for the node source discovery script (K8s pod execution)
//
//go:embed rundeck_k8s/node_source.sh.tmpl
var RundeckK8sNodeSourceTemplate string

// RundeckK8sResourceCreateJobTemplate is the template for per-resource create jobs (K8s pod execution)
//
//go:embed rundeck_k8s/resource_create_job.yaml.tmpl
var RundeckK8sResourceCreateJobTemplate string

// RundeckK8sResourceGetJobTemplate is the template for per-resource list/get jobs (K8s pod execution)
//
//go:embed rundeck_k8s/resource_get_job.yaml.tmpl
var RundeckK8sResourceGetJobTemplate string

// RundeckK8sResourceDescribeJobTemplate is the template for per-resource describe jobs (K8s pod execution)
//
//go:embed rundeck_k8s/resource_describe_job.yaml.tmpl
var RundeckK8sResourceDescribeJobTemplate string

// RundeckK8sQueryJobTemplate is the template for per-query jobs (K8s pod execution)
//
//go:embed rundeck_k8s/query_job.yaml.tmpl
var RundeckK8sQueryJobTemplate string

// RundeckK8sActionJobTemplate is the template for per-action jobs (K8s pod execution)
//
//go:embed rundeck_k8s/action_job.yaml.tmpl
var RundeckK8sActionJobTemplate string

// RundeckK8sStatusJobTemplate is the template for the operator status job (K8s pod execution)
//
//go:embed rundeck_k8s/status_job.yaml.tmpl
var RundeckK8sStatusJobTemplate string

// RundeckK8sDriftJobTemplate is the template for the drift detection job (K8s pod execution)
//
//go:embed rundeck_k8s/drift_job.yaml.tmpl
var RundeckK8sDriftJobTemplate string

// RundeckK8sCleanupJobTemplate is the template for the cleanup job (K8s pod execution)
//
//go:embed rundeck_k8s/cleanup_job.yaml.tmpl
var RundeckK8sCleanupJobTemplate string

// Rundeck Diagnostic/Operational Job Templates (native script execution)

// RundeckDiagnoseJobTemplate is the template for the diagnose job
//
//go:embed rundeck/diagnose_job.yaml.tmpl
var RundeckDiagnoseJobTemplate string

// RundeckCompareJobTemplate is the template for the compare job
//
//go:embed rundeck/compare_job.yaml.tmpl
var RundeckCompareJobTemplate string

// RundeckPauseJobTemplate is the template for the pause job
//
//go:embed rundeck/pause_job.yaml.tmpl
var RundeckPauseJobTemplate string

// RundeckUnpauseJobTemplate is the template for the unpause job
//
//go:embed rundeck/unpause_job.yaml.tmpl
var RundeckUnpauseJobTemplate string

// RundeckPatchJobTemplate is the template for the patch job
//
//go:embed rundeck/patch_job.yaml.tmpl
var RundeckPatchJobTemplate string

// Rundeck Diagnostic/Operational Job Templates (Docker execution)

// RundeckDockerDiagnoseJobTemplate is the template for the diagnose job (Docker execution)
//
//go:embed rundeck_docker/diagnose_job.yaml.tmpl
var RundeckDockerDiagnoseJobTemplate string

// RundeckDockerCompareJobTemplate is the template for the compare job (Docker execution)
//
//go:embed rundeck_docker/compare_job.yaml.tmpl
var RundeckDockerCompareJobTemplate string

// RundeckDockerPauseJobTemplate is the template for the pause job (Docker execution)
//
//go:embed rundeck_docker/pause_job.yaml.tmpl
var RundeckDockerPauseJobTemplate string

// RundeckDockerUnpauseJobTemplate is the template for the unpause job (Docker execution)
//
//go:embed rundeck_docker/unpause_job.yaml.tmpl
var RundeckDockerUnpauseJobTemplate string

// RundeckDockerPatchJobTemplate is the template for the patch job (Docker execution)
//
//go:embed rundeck_docker/patch_job.yaml.tmpl
var RundeckDockerPatchJobTemplate string

// Rundeck Diagnostic/Operational Job Templates (K8s pod execution)

// RundeckK8sDiagnoseJobTemplate is the template for the diagnose job (K8s pod execution)
//
//go:embed rundeck_k8s/diagnose_job.yaml.tmpl
var RundeckK8sDiagnoseJobTemplate string

// RundeckK8sCompareJobTemplate is the template for the compare job (K8s pod execution)
//
//go:embed rundeck_k8s/compare_job.yaml.tmpl
var RundeckK8sCompareJobTemplate string

// RundeckK8sPauseJobTemplate is the template for the pause job (K8s pod execution)
//
//go:embed rundeck_k8s/pause_job.yaml.tmpl
var RundeckK8sPauseJobTemplate string

// RundeckK8sUnpauseJobTemplate is the template for the unpause job (K8s pod execution)
//
//go:embed rundeck_k8s/unpause_job.yaml.tmpl
var RundeckK8sUnpauseJobTemplate string

// RundeckK8sPatchJobTemplate is the template for the patch job (K8s pod execution)
//
//go:embed rundeck_k8s/patch_job.yaml.tmpl
var RundeckK8sPatchJobTemplate string

// Rundeck Managed CR Lifecycle Job Templates (native script execution)

// RundeckManagedApplyJobTemplate is the template for managed CR apply jobs
//
//go:embed rundeck/managed_apply_job.yaml.tmpl
var RundeckManagedApplyJobTemplate string

// RundeckManagedGetJobTemplate is the template for managed CR get jobs
//
//go:embed rundeck/managed_get_job.yaml.tmpl
var RundeckManagedGetJobTemplate string

// RundeckManagedPatchJobTemplate is the template for managed CR patch jobs
//
//go:embed rundeck/managed_patch_job.yaml.tmpl
var RundeckManagedPatchJobTemplate string

// RundeckManagedDeleteJobTemplate is the template for managed CR delete jobs
//
//go:embed rundeck/managed_delete_job.yaml.tmpl
var RundeckManagedDeleteJobTemplate string

// RundeckManagedStatusJobTemplate is the template for managed CR status jobs
//
//go:embed rundeck/managed_status_job.yaml.tmpl
var RundeckManagedStatusJobTemplate string

// Rundeck Managed CR Lifecycle Job Templates (Docker execution)

// RundeckDockerManagedApplyJobTemplate is the template for managed CR apply jobs (Docker execution)
//
//go:embed rundeck_docker/managed_apply_job.yaml.tmpl
var RundeckDockerManagedApplyJobTemplate string

// RundeckDockerManagedGetJobTemplate is the template for managed CR get jobs (Docker execution)
//
//go:embed rundeck_docker/managed_get_job.yaml.tmpl
var RundeckDockerManagedGetJobTemplate string

// RundeckDockerManagedPatchJobTemplate is the template for managed CR patch jobs (Docker execution)
//
//go:embed rundeck_docker/managed_patch_job.yaml.tmpl
var RundeckDockerManagedPatchJobTemplate string

// RundeckDockerManagedDeleteJobTemplate is the template for managed CR delete jobs (Docker execution)
//
//go:embed rundeck_docker/managed_delete_job.yaml.tmpl
var RundeckDockerManagedDeleteJobTemplate string

// RundeckDockerManagedStatusJobTemplate is the template for managed CR status jobs (Docker execution)
//
//go:embed rundeck_docker/managed_status_job.yaml.tmpl
var RundeckDockerManagedStatusJobTemplate string

// Rundeck Managed CR Lifecycle Job Templates (K8s pod execution)

// RundeckK8sManagedApplyJobTemplate is the template for managed CR apply jobs (K8s pod execution)
//
//go:embed rundeck_k8s/managed_apply_job.yaml.tmpl
var RundeckK8sManagedApplyJobTemplate string

// RundeckK8sManagedGetJobTemplate is the template for managed CR get jobs (K8s pod execution)
//
//go:embed rundeck_k8s/managed_get_job.yaml.tmpl
var RundeckK8sManagedGetJobTemplate string

// RundeckK8sManagedPatchJobTemplate is the template for managed CR patch jobs (K8s pod execution)
//
//go:embed rundeck_k8s/managed_patch_job.yaml.tmpl
var RundeckK8sManagedPatchJobTemplate string

// RundeckK8sManagedDeleteJobTemplate is the template for managed CR delete jobs (K8s pod execution)
//
//go:embed rundeck_k8s/managed_delete_job.yaml.tmpl
var RundeckK8sManagedDeleteJobTemplate string

// RundeckK8sManagedStatusJobTemplate is the template for managed CR status jobs (K8s pod execution)
//
//go:embed rundeck_k8s/managed_status_job.yaml.tmpl
var RundeckK8sManagedStatusJobTemplate string

// Rundeck Workflow Templates (shared across all execution modes)
// Workflow jobs use jobref to chain existing atomic jobs by name/group.
// Since jobrefs are execution-mode agnostic, one template serves all modes.

// RundeckHousekeepingWorkflowTemplate is the template for the housekeeping workflow job
//
//go:embed rundeck_workflows/housekeeping_workflow.yaml.tmpl
var RundeckHousekeepingWorkflowTemplate string

// RundeckDriftRemediationWorkflowTemplate is the template for the drift remediation workflow job
//
//go:embed rundeck_workflows/drift_remediation_workflow.yaml.tmpl
var RundeckDriftRemediationWorkflowTemplate string

// RundeckStartMaintenanceWorkflowTemplate is the template for the start maintenance workflow job
//
//go:embed rundeck_workflows/start_maintenance_workflow.yaml.tmpl
var RundeckStartMaintenanceWorkflowTemplate string

// RundeckEndMaintenanceWorkflowTemplate is the template for the end maintenance workflow job
//
//go:embed rundeck_workflows/end_maintenance_workflow.yaml.tmpl
var RundeckEndMaintenanceWorkflowTemplate string

// RundeckCreateAndVerifyWorkflowTemplate is the template for the per-resource create-and-verify workflow job
//
//go:embed rundeck_workflows/create_and_verify_workflow.yaml.tmpl
var RundeckCreateAndVerifyWorkflowTemplate string

// RundeckManagedDeployWorkflowTemplate is the template for the managed CR deploy-and-verify workflow job
//
//go:embed rundeck_workflows/managed_deploy_workflow.yaml.tmpl
var RundeckManagedDeployWorkflowTemplate string

// Kubectl Plugin Dockerfile Template

// KubectlPluginDockerfileTemplate is the template for the kubectl plugin Docker image
//
//go:embed kubectl_plugin/dockerfile.tmpl
var KubectlPluginDockerfileTemplate string

// Rundeck Node Source Plugin Templates

// RundeckPluginYAMLTemplate is the template for the Rundeck ResourceModelSource plugin descriptor
//
//go:embed rundeck_plugin/plugin.yaml.tmpl
var RundeckPluginYAMLTemplate string

// RundeckPluginNodesScriptTemplate is the template for the Rundeck ResourceModelSource plugin script
//
//go:embed rundeck_plugin/nodes.sh.tmpl
var RundeckPluginNodesScriptTemplate string
