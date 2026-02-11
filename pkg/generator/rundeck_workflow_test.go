package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluecontainer/openapi-operator-gen/internal/config"
	"github.com/bluecontainer/openapi-operator-gen/pkg/mapper"
)

// =============================================================================
// Rundeck Workflow Generation Tests
// =============================================================================

func testCRDs(cfg *config.Config) []*mapper.CRDDefinition {
	return []*mapper.CRDDefinition{
		{
			APIGroup:   cfg.APIGroup,
			APIVersion: cfg.APIVersion,
			Kind:       "Pet",
			Plural:     "pets",
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{Name: "Name", JSONName: "name", GoType: "string", Required: true},
					{Name: "Status", JSONName: "status", GoType: "string", Enum: []string{"available", "pending", "sold"}},
				},
			},
		},
		{
			APIGroup:   cfg.APIGroup,
			APIVersion: cfg.APIVersion,
			Kind:       "Order",
			Plural:     "orders",
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{Name: "Quantity", JSONName: "quantity", GoType: "int32", Required: true},
				},
			},
		},
		{
			APIGroup:   cfg.APIGroup,
			APIVersion: cfg.APIVersion,
			Kind:       "PetFindByStatusQuery",
			Plural:     "petfindbystatusqueries",
			IsQuery:    true,
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{Name: "Status", JSONName: "status", GoType: "string", Required: true},
				},
			},
		},
		{
			APIGroup:   cfg.APIGroup,
			APIVersion: cfg.APIVersion,
			Kind:       "PetUploadImageAction",
			Plural:     "petuploadimageactions",
			IsAction:   true,
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{Name: "PetId", JSONName: "petId", GoType: "int64", Required: true},
				},
			},
		},
	}
}

func TestRundeckWorkflowGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:        tmpDir,
		APIGroup:         "petstore.example.com",
		APIVersion:       "v1alpha1",
		GeneratorVersion: "test",
	}
	g := NewRundeckProjectGenerator(cfg)
	crds := testCRDs(cfg)

	if err := g.Generate(crds); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	workflowDir := filepath.Join(tmpDir, "rundeck-project", "jobs", "workflows")

	// Verify workflows directory exists
	if info, err := os.Stat(workflowDir); err != nil {
		t.Fatalf("expected workflows directory to exist: %v", err)
	} else if !info.IsDir() {
		t.Fatal("expected workflows to be a directory")
	}

	// Verify managed subdirectory exists
	managedDir := filepath.Join(workflowDir, "managed")
	if info, err := os.Stat(managedDir); err != nil {
		t.Fatalf("expected workflows/managed directory to exist: %v", err)
	} else if !info.IsDir() {
		t.Fatal("expected workflows/managed to be a directory")
	}

	// Verify all 4 operations workflows exist
	operationsWorkflows := []string{
		"housekeeping.yaml",
		"drift-remediation.yaml",
		"start-maintenance.yaml",
		"end-maintenance.yaml",
	}
	for _, f := range operationsWorkflows {
		path := filepath.Join(workflowDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", f)
		}
	}

	// Verify per-resource create-and-verify workflows exist for CRUD resources only
	crudWorkflows := []string{
		"create-and-verify-pet.yaml",
		"create-and-verify-order.yaml",
	}
	for _, f := range crudWorkflows {
		path := filepath.Join(workflowDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", f)
		}
	}

	// Verify NO workflows generated for query/action CRDs
	nonCrudWorkflows := []string{
		"create-and-verify-petfindbystatusquery.yaml",
		"create-and-verify-petuploadimageaction.yaml",
	}
	for _, f := range nonCrudWorkflows {
		path := filepath.Join(workflowDir, f)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("should NOT generate %s for query/action endpoints", f)
		}
	}
}

func TestRundeckWorkflowContent(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:        tmpDir,
		APIGroup:         "petstore.example.com",
		APIVersion:       "v1alpha1",
		GeneratorVersion: "test",
	}
	g := NewRundeckProjectGenerator(cfg)
	crds := testCRDs(cfg)

	if err := g.Generate(crds); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	workflowDir := filepath.Join(tmpDir, "rundeck-project", "jobs", "workflows")

	t.Run("housekeeping uses jobref", func(t *testing.T) {
		content := readFile(t, filepath.Join(workflowDir, "housekeeping.yaml"))
		assertContains(t, content, "jobref:")
		assertContains(t, content, "name: cleanup")
		assertContains(t, content, "group: operations")
		assertContains(t, content, "name: status")
		assertContains(t, content, "-expired true -force true")
		assertContains(t, content, "keepgoing: false")
		assertNotContains(t, content, "scriptInterpreter")
		assertNotContains(t, content, "k8s_token")
	})

	t.Run("drift-remediation uses jobref", func(t *testing.T) {
		content := readFile(t, filepath.Join(workflowDir, "drift-remediation.yaml"))
		assertContains(t, content, "jobref:")
		assertContains(t, content, "name: drift")
		assertContains(t, content, "name: status")
		assertContains(t, content, "-show_diff true")
		assertNotContains(t, content, "k8s_token")
	})

	t.Run("start-maintenance chains pause then status", func(t *testing.T) {
		content := readFile(t, filepath.Join(workflowDir, "start-maintenance.yaml"))
		assertContains(t, content, "jobref:")
		assertContains(t, content, "name: pause")
		assertContains(t, content, "name: status")
		assertContains(t, content, "-all true")
		assertContains(t, content, "resource_kind")
		assertContains(t, content, "required: true")
		assertNotContains(t, content, "k8s_token")
	})

	t.Run("end-maintenance chains unpause then drift then status", func(t *testing.T) {
		content := readFile(t, filepath.Join(workflowDir, "end-maintenance.yaml"))
		assertContains(t, content, "name: unpause")
		assertContains(t, content, "name: drift")
		assertContains(t, content, "name: status")
		// Verify 3 jobref steps
		if strings.Count(content, "jobref:") != 3 {
			t.Errorf("expected 3 jobref steps, got %d", strings.Count(content, "jobref:"))
		}
		assertNotContains(t, content, "k8s_token")
	})

	t.Run("create-and-verify-pet has correct structure", func(t *testing.T) {
		content := readFile(t, filepath.Join(workflowDir, "create-and-verify-pet.yaml"))
		assertContains(t, content, "name: create-pet")
		assertContains(t, content, "name: describe-pet")
		assertContains(t, content, "group: resources/pet")
		// resource_name is optional (auto-generated if not specified)
		assertContains(t, content, "name: resource_name")
		assertNotContains(t, content, "\"Name for the CR (required")
		// Spec fields present
		assertContains(t, content, "name: name")
		assertContains(t, content, "name: status")
		// Enum values from Pet.status
		assertContains(t, content, "available")
		assertContains(t, content, "pending")
		assertContains(t, content, "sold")
		// Create step uses importOptions to forward parent options, with -output json override
		assertContains(t, content, "importOptions: true")
		assertContains(t, content, "-output json")
		// Describe step uses export.resource_name from child's export-var
		assertContains(t, content, "${export.resource_name}")
		// Output option renamed to verify_output to avoid importOptions collision
		assertContains(t, content, "name: verify_output")
		assertContains(t, content, "${option.verify_output}")
		// Pure jobref workflow - no inline scripts
		assertNotContains(t, content, "script: |-")
		assertNotContains(t, content, "key-value-data")
		assertNotContains(t, content, "RD_JOB_EXECID")
		// No credentials in workflow (child jobs handle their own)
		assertNotContains(t, content, "k8s_token")
		assertNotContains(t, content, "scriptInterpreter")
		// No invalid Rundeck attributes
		assertNotContains(t, content, "exportAll")
		// Group is workflows
		assertContains(t, content, "group: workflows")
	})

	t.Run("all workflows are pure jobref with no credentials", func(t *testing.T) {
		files := []string{
			"housekeeping.yaml",
			"drift-remediation.yaml",
			"start-maintenance.yaml",
			"end-maintenance.yaml",
			"create-and-verify-pet.yaml",
			"create-and-verify-order.yaml",
		}
		for _, f := range files {
			content := readFile(t, filepath.Join(workflowDir, f))
			assertContains(t, content, "jobref:")
			assertNotContains(t, content, "k8s_token")
			assertNotContains(t, content, "storagePath")
			assertNotContains(t, content, "scriptInterpreter")
			assertNotContains(t, content, "kubectl")
			assertNotContains(t, content, "script: |-")
		}
	})
}

func TestRundeckWorkflowsIdenticalAcrossModes(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:        tmpDir,
		APIGroup:         "petstore.example.com",
		APIVersion:       "v1alpha1",
		GeneratorVersion: "test",
	}
	g := NewRundeckProjectGenerator(cfg)
	crds := testCRDs(cfg)

	// Generate all 3 modes
	if err := g.Generate(crds); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if err := g.GenerateDockerProject(crds); err != nil {
		t.Fatalf("GenerateDockerProject failed: %v", err)
	}
	if err := g.GenerateK8sProject(crds); err != nil {
		t.Fatalf("GenerateK8sProject failed: %v", err)
	}

	workflowFiles := []string{
		"housekeeping.yaml",
		"drift-remediation.yaml",
		"start-maintenance.yaml",
		"end-maintenance.yaml",
		"create-and-verify-pet.yaml",
		"create-and-verify-order.yaml",
	}

	modes := []string{"rundeck-project", "rundeck-docker-project", "rundeck-k8s-project"}

	for _, f := range workflowFiles {
		// Read the native version as reference
		nativePath := filepath.Join(tmpDir, modes[0], "jobs", "workflows", f)
		nativeContent := readFile(t, nativePath)

		for _, mode := range modes[1:] {
			modePath := filepath.Join(tmpDir, mode, "jobs", "workflows", f)
			modeContent := readFile(t, modePath)

			if nativeContent != modeContent {
				t.Errorf("workflow %s differs between %s and %s", f, modes[0], mode)
			}
		}
	}
}

func TestRundeckManagedDeployWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:        tmpDir,
		APIGroup:         "petstore.example.com",
		APIVersion:       "v1alpha1",
		GeneratorVersion: "test",
	}
	g := NewRundeckProjectGenerator(cfg)
	crds := testCRDs(cfg)

	// Generate base projects first (creates directory structure)
	if err := g.Generate(crds); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if err := g.GenerateDockerProject(crds); err != nil {
		t.Fatalf("GenerateDockerProject failed: %v", err)
	}
	if err := g.GenerateK8sProject(crds); err != nil {
		t.Fatalf("GenerateK8sProject failed: %v", err)
	}

	// Create a managed CR YAML file
	managedDir := filepath.Join(tmpDir, "managed-crs")
	if err := os.MkdirAll(managedDir, 0755); err != nil {
		t.Fatalf("failed to create managed CRs dir: %v", err)
	}
	crYaml := `apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: fluffy
spec:
  name: Fluffy
  status: available`
	if err := os.WriteFile(filepath.Join(managedDir, "pet.yaml"), []byte(crYaml), 0644); err != nil {
		t.Fatalf("failed to write managed CR: %v", err)
	}

	if err := g.GenerateManagedJobs(managedDir); err != nil {
		t.Fatalf("GenerateManagedJobs failed: %v", err)
	}

	// Verify deploy workflow exists in all 3 modes
	modes := []string{"rundeck-project", "rundeck-docker-project", "rundeck-k8s-project"}
	for _, mode := range modes {
		deployPath := filepath.Join(tmpDir, mode, "jobs", "workflows", "managed", "deploy-pet-fluffy.yaml")
		if _, err := os.Stat(deployPath); os.IsNotExist(err) {
			t.Errorf("expected %s to exist in %s", "deploy-pet-fluffy.yaml", mode)
			continue
		}

		content := readFile(t, deployPath)

		// Verify jobref structure
		assertContains(t, content, "jobref:")
		assertContains(t, content, "name: apply-pet-fluffy")
		assertContains(t, content, "name: status-pet-fluffy")
		assertContains(t, content, "group: managed/pet-fluffy")
		assertContains(t, content, "group: workflows/managed")
		assertNotContains(t, content, "k8s_token")
		assertNotContains(t, content, "scriptInterpreter")
	}

	// Verify deploy workflows are identical across modes
	nativeContent := readFile(t, filepath.Join(tmpDir, modes[0], "jobs", "workflows", "managed", "deploy-pet-fluffy.yaml"))
	for _, mode := range modes[1:] {
		modeContent := readFile(t, filepath.Join(tmpDir, mode, "jobs", "workflows", "managed", "deploy-pet-fluffy.yaml"))
		if nativeContent != modeContent {
			t.Errorf("managed deploy workflow differs between %s and %s", modes[0], mode)
		}
	}
}

// =============================================================================
// Rundeck Node Source Discovery Tests
// =============================================================================

func TestRundeckProjectPropertiesNodeSource(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:        tmpDir,
		APIGroup:         "petstore.example.com",
		APIVersion:       "v1alpha1",
		GeneratorVersion: "test",
	}
	g := NewRundeckProjectGenerator(cfg)
	crds := testCRDs(cfg)

	// Generate all 3 modes
	if err := g.Generate(crds); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if err := g.GenerateDockerProject(crds); err != nil {
		t.Fatalf("GenerateDockerProject failed: %v", err)
	}
	if err := g.GenerateK8sProject(crds); err != nil {
		t.Fatalf("GenerateK8sProject failed: %v", err)
	}

	t.Run("native project.properties uses custom node source plugin", func(t *testing.T) {
		content := readFile(t, filepath.Join(tmpDir, "rundeck-project", "project.properties"))
		assertContains(t, content, "resources.source.1.type=petstore-k8s-nodes")
		assertContains(t, content, "resources.source.1.config.cluster_token_suffix=project/petstore-operator/k8s-token")
		assertContains(t, content, "resources.source.1.config.k8s_token=keys/${resources.source.1.config.cluster_token_suffix}")
		assertContains(t, content, "resources.source.1.config.execution_mode=native")
		// Verify the node-source.sh reference script exists and contains expected content
		script := readFile(t, filepath.Join(tmpDir, "rundeck-project", "node-source.sh"))
		assertContains(t, script, "kubectl-petstore")
		assertContains(t, script, "nodes")
	})

	t.Run("docker project.properties uses custom node source plugin", func(t *testing.T) {
		content := readFile(t, filepath.Join(tmpDir, "rundeck-docker-project", "project.properties"))
		assertContains(t, content, "resources.source.1.type=petstore-k8s-nodes")
		assertContains(t, content, "resources.source.1.config.cluster_token_suffix=project/petstore-operator-docker/k8s-token")
		assertContains(t, content, "resources.source.1.config.k8s_token=keys/${resources.source.1.config.cluster_token_suffix}")
		assertContains(t, content, "resources.source.1.config.execution_mode=docker")
		script := readFile(t, filepath.Join(tmpDir, "rundeck-docker-project", "node-source.sh"))
		assertContains(t, script, "docker run")
		assertContains(t, script, "petstore")
		assertContains(t, script, "nodes")
	})

	t.Run("k8s project.properties uses custom node source plugin", func(t *testing.T) {
		content := readFile(t, filepath.Join(tmpDir, "rundeck-k8s-project", "project.properties"))
		assertContains(t, content, "resources.source.1.type=petstore-k8s-nodes")
		assertContains(t, content, "resources.source.1.config.cluster_token_suffix=project/petstore-operator-k8s/k8s-token")
		assertContains(t, content, "resources.source.1.config.k8s_token=keys/${resources.source.1.config.cluster_token_suffix}")
		assertContains(t, content, "resources.source.1.config.execution_mode=kubernetes")
		script := readFile(t, filepath.Join(tmpDir, "rundeck-k8s-project", "node-source.sh"))
		assertContains(t, script, "kubectl")
		assertContains(t, script, "plugin-runner")
		assertContains(t, script, "nodes")
	})
}

func TestRundeckProjectJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:        tmpDir,
		APIGroup:         "petstore.example.com",
		APIVersion:       "v1alpha1",
		GeneratorVersion: "test",
	}
	g := NewRundeckProjectGenerator(cfg)
	crds := testCRDs(cfg)

	// Generate all 3 modes
	if err := g.Generate(crds); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if err := g.GenerateDockerProject(crds); err != nil {
		t.Fatalf("GenerateDockerProject failed: %v", err)
	}
	if err := g.GenerateK8sProject(crds); err != nil {
		t.Fatalf("GenerateK8sProject failed: %v", err)
	}

	type projectJSON struct {
		Name   string            `json:"name"`
		Config map[string]string `json:"config"`
	}

	tests := []struct {
		name          string
		dir           string
		projectName   string
		executionMode string
	}{
		{"native", "rundeck-project", "petstore-operator", "native"},
		{"docker", "rundeck-docker-project", "petstore-operator-docker", "docker"},
		{"k8s", "rundeck-k8s-project", "petstore-operator-k8s", "kubernetes"},
	}

	for _, tt := range tests {
		t.Run(tt.name+" project.json is valid and mirrors properties", func(t *testing.T) {
			jsonPath := filepath.Join(tmpDir, tt.dir, "project.json")
			data, err := os.ReadFile(jsonPath)
			if err != nil {
				t.Fatalf("failed to read project.json: %v", err)
			}

			var proj projectJSON
			if err := json.Unmarshal(data, &proj); err != nil {
				t.Fatalf("project.json is not valid JSON: %v", err)
			}

			if proj.Name != tt.projectName {
				t.Errorf("expected name %q, got %q", tt.projectName, proj.Name)
			}

			// Verify config contains custom node source plugin
			if proj.Config["resources.source.1.type"] != "petstore-k8s-nodes" {
				t.Errorf("expected resources.source.1.type=petstore-k8s-nodes, got %q", proj.Config["resources.source.1.type"])
			}
			if proj.Config["resources.source.1.config.execution_mode"] != tt.executionMode {
				t.Errorf("expected resources.source.1.config.execution_mode=%s, got %q", tt.executionMode, proj.Config["resources.source.1.config.execution_mode"])
			}

			// Verify config mirrors project.properties keys
			if proj.Config["project.name"] != tt.projectName {
				t.Errorf("expected config project.name=%q, got %q", tt.projectName, proj.Config["project.name"])
			}
			if proj.Config["project.globals.namespace"] != "petstore-system" {
				t.Errorf("expected namespace=petstore-system, got %q", proj.Config["project.globals.namespace"])
			}
		})
	}
}

func TestRundeckJobTemplatesHybridTargeting(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:        tmpDir,
		APIGroup:         "petstore.example.com",
		APIVersion:       "v1alpha1",
		GeneratorVersion: "test",
	}
	g := NewRundeckProjectGenerator(cfg)
	crds := testCRDs(cfg)

	// Generate all 3 modes
	if err := g.Generate(crds); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if err := g.GenerateDockerProject(crds); err != nil {
		t.Fatalf("GenerateDockerProject failed: %v", err)
	}
	if err := g.GenerateK8sProject(crds); err != nil {
		t.Fatalf("GenerateK8sProject failed: %v", err)
	}

	// Job templates that should have hybrid targeting
	modes := []string{"rundeck-project", "rundeck-docker-project", "rundeck-k8s-project"}
	targetingJobs := map[string]string{
		"resources/pet/create-pet.yaml":     "jobs/resources/create-pet.yaml",
		"queries/petfindbystatusquery.yaml": "jobs/queries/petfindbystatusquery.yaml",
		"actions/petuploadimageaction.yaml": "jobs/actions/petuploadimageaction.yaml",
		"operations/patch.yaml":             "jobs/operations/patch.yaml",
	}

	for name, relPath := range targetingJobs {
		for _, mode := range modes {
			t.Run(mode+"/"+name+" has hybrid targeting", func(t *testing.T) {
				path := filepath.Join(tmpDir, mode, relPath)
				content := readFile(t, path)

				// Verify node dispatch enabled
				assertContains(t, content, "nodeFilterEditable: true")
				assertContains(t, content, "nodefilters:")
				assertContains(t, content, "nodeStep: true")

				// Verify hybrid targeting uses node attributes
				assertContains(t, content, `@node.targetType@`)
				assertContains(t, content, `@node.targetValue@`)
				assertContains(t, content, `@node.targetNamespace@`)

				// Verify explicit options still available as overrides
				assertContains(t, content, "target_statefulset")
				assertContains(t, content, "target_deployment")
				assertContains(t, content, "target_helm_release")

				// Verify case statement for node type dispatch
				assertContains(t, content, "helm-release)")
				assertContains(t, content, "statefulset)")
				assertContains(t, content, "deployment)")
			})
		}
	}
}

func TestRundeckNonTargetingJobsUnmodified(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:        tmpDir,
		APIGroup:         "petstore.example.com",
		APIVersion:       "v1alpha1",
		GeneratorVersion: "test",
	}
	g := NewRundeckProjectGenerator(cfg)
	crds := testCRDs(cfg)

	if err := g.Generate(crds); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Non-targeting jobs should NOT have node attributes
	nonTargetingJobs := []string{
		"jobs/resources/get-pets.yaml",
		"jobs/resources/describe-pet.yaml",
		"jobs/operations/status.yaml",
		"jobs/operations/drift.yaml",
		"jobs/operations/cleanup.yaml",
	}

	for _, relPath := range nonTargetingJobs {
		t.Run(relPath+" has no node targeting", func(t *testing.T) {
			path := filepath.Join(tmpDir, "rundeck-project", relPath)
			content := readFile(t, path)
			assertNotContains(t, content, "@node.targetType@")
			assertNotContains(t, content, "@node.targetValue@")
		})
	}
}

// =============================================================================
// Test Helpers
// =============================================================================

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(data)
}

func assertContains(t *testing.T, content, substr string) {
	t.Helper()
	if !strings.Contains(content, substr) {
		t.Errorf("expected content to contain %q", substr)
	}
}

func assertNotContains(t *testing.T, content, substr string) {
	t.Helper()
	if strings.Contains(content, substr) {
		t.Errorf("expected content NOT to contain %q", substr)
	}
}
