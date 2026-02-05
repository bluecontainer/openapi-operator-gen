package generator

import (
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
