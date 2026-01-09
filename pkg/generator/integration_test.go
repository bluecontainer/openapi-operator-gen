package generator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluecontainer/openapi-operator-gen/internal/config"
	"github.com/bluecontainer/openapi-operator-gen/pkg/mapper"
	"github.com/bluecontainer/openapi-operator-gen/pkg/parser"
)

// =============================================================================
// Option 2: Compilation Testing
// =============================================================================

// TestGeneratedCodeCompiles generates a complete operator and verifies it compiles
func TestGeneratedCodeCompiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compilation test in short mode")
	}

	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "petstore.example.com",
		APIVersion: "v1alpha1",
		ModuleName: "github.com/example/petstore-operator",
	}

	// Create CRDs for all three types: Resource, Query, and Action
	crds := []*mapper.CRDDefinition{
		// Standard Resource CRD
		{
			APIGroup:   cfg.APIGroup,
			APIVersion: cfg.APIVersion,
			Kind:       "Pet",
			Plural:     "pets",
			ShortNames: []string{"pe"},
			Scope:      "Namespaced",
			BasePath:   "/pet",
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{Name: "Name", JSONName: "name", GoType: "string", Required: true},
					{Name: "Status", JSONName: "status", GoType: "string"},
					{Name: "PhotoUrls", JSONName: "photoUrls", GoType: "[]string"},
				},
			},
		},
		// Query CRD
		{
			APIGroup:        cfg.APIGroup,
			APIVersion:      cfg.APIVersion,
			Kind:            "PetFindByStatusQuery",
			Plural:          "petfindbystatusqueries",
			ShortNames:      []string{"pfbs"},
			Scope:           "Namespaced",
			IsQuery:         true,
			QueryPath:       "/pet/findByStatus",
			ResponseIsArray: true,
			ResultItemType:  "Pet",
			UsesSharedType:  true,
			ResponseType:    "[]Pet",
			QueryParams: []mapper.QueryParamField{
				{Name: "Status", JSONName: "status", GoType: "string", Required: true},
			},
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{Name: "Status", JSONName: "status", GoType: "string", Required: true},
				},
			},
		},
		// Action CRD (with parent ID)
		{
			APIGroup:       cfg.APIGroup,
			APIVersion:     cfg.APIVersion,
			Kind:           "PetUploadImageAction",
			Plural:         "petuploadimageactions",
			ShortNames:     []string{"pui"},
			Scope:          "Namespaced",
			IsAction:       true,
			ActionPath:     "/pet/{petId}/uploadImage",
			ActionMethod:   "POST",
			ParentResource: "Pet",
			ParentIDParam:  "petId",
			ActionName:     "uploadImage",
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{Name: "PetId", JSONName: "petId", GoType: "string", Required: true},
					{Name: "AdditionalMetadata", JSONName: "additionalMetadata", GoType: "string"},
					{
						Name:        "ReExecuteInterval",
						JSONName:    "reExecuteInterval",
						GoType:      "*metav1.Duration",
						Description: "Interval at which to re-execute the action (e.g., 30s, 5m, 1h). If not set, action is one-shot.",
						Required:    false,
					},
				},
			},
		},
		// Action CRD (without parent ID)
		{
			APIGroup:     cfg.APIGroup,
			APIVersion:   cfg.APIVersion,
			Kind:         "UserLoginAction",
			Plural:       "userloginactions",
			ShortNames:   []string{"ul"},
			Scope:        "Namespaced",
			IsAction:     true,
			ActionPath:   "/user/login",
			ActionMethod: "POST",
			ActionName:   "login",
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{Name: "Username", JSONName: "username", GoType: "string", Required: true},
					{Name: "Password", JSONName: "password", GoType: "string", Required: true},
					{
						Name:        "ReExecuteInterval",
						JSONName:    "reExecuteInterval",
						GoType:      "*metav1.Duration",
						Description: "Interval at which to re-execute the action (e.g., 30s, 5m, 1h). If not set, action is one-shot.",
						Required:    false,
					},
				},
			},
		},
	}

	// Generate all code
	typesGen := NewTypesGenerator(cfg)
	if err := typesGen.Generate(crds); err != nil {
		t.Fatalf("TypesGenerator.Generate failed: %v", err)
	}

	controllerGen := NewControllerGenerator(cfg)
	if err := controllerGen.Generate(crds); err != nil {
		t.Fatalf("ControllerGenerator.Generate failed: %v", err)
	}

	// Run compilation steps
	if err := runCompilationSteps(t, tmpDir); err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	t.Log("Generated code compiles successfully")
}

// TestGeneratedCodeWithE2ETests generates code with E2E tests and runs them
func TestGeneratedCodeWithE2ETests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "petstore.example.com",
		APIVersion: "v1alpha1",
		ModuleName: "github.com/example/petstore-operator",
	}

	// Create a simple Resource CRD for testing
	crds := []*mapper.CRDDefinition{
		{
			APIGroup:   cfg.APIGroup,
			APIVersion: cfg.APIVersion,
			Kind:       "Pet",
			Plural:     "pets",
			ShortNames: []string{"pe"},
			Scope:      "Namespaced",
			BasePath:   "/pet",
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{Name: "Name", JSONName: "name", GoType: "string", Required: true},
					{Name: "Status", JSONName: "status", GoType: "string"},
				},
			},
		},
	}

	// Generate types and controllers
	typesGen := NewTypesGenerator(cfg)
	if err := typesGen.Generate(crds); err != nil {
		t.Fatalf("TypesGenerator.Generate failed: %v", err)
	}

	controllerGen := NewControllerGenerator(cfg)
	if err := controllerGen.Generate(crds); err != nil {
		t.Fatalf("ControllerGenerator.Generate failed: %v", err)
	}

	// Generate E2E test file
	testContent := generateControllerTestTemplate(cfg, crds[0])
	testPath := filepath.Join(tmpDir, "internal", "controller", "pet_controller_test.go")
	if err := os.WriteFile(testPath, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Run compilation and tests
	if err := runCompilationAndTestSteps(t, tmpDir); err != nil {
		t.Fatalf("Compilation/tests failed: %v", err)
	}

	t.Log("Generated code with E2E tests passed successfully")
}

// TestGeneratedCodeFromPetstoreSpec generates code from actual petstore.yaml and verifies compilation
func TestGeneratedCodeFromPetstoreSpec(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compilation test in short mode")
	}

	// Find the petstore spec file
	specPath := filepath.Join("..", "..", "examples", "petstore.1.0.27.yaml")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		t.Skipf("petstore.yaml not found at %s", specPath)
	}

	tmpDir := t.TempDir()
	cfg := &config.Config{
		SpecPath:    specPath,
		OutputDir:   tmpDir,
		APIGroup:    "petstore.example.com",
		APIVersion:  "v1alpha1",
		ModuleName:  "github.com/example/petstore-operator",
		MappingMode: config.PerResource,
	}

	// Parse spec
	p := parser.NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("failed to parse spec: %v", err)
	}

	// Map to CRDs
	m := mapper.NewMapper(cfg)
	crds, err := m.MapResources(spec)
	if err != nil {
		t.Fatalf("failed to map resources: %v", err)
	}

	// Generate all code
	typesGen := NewTypesGenerator(cfg)
	if err := typesGen.Generate(crds); err != nil {
		t.Fatalf("TypesGenerator.Generate failed: %v", err)
	}

	controllerGen := NewControllerGenerator(cfg)
	if err := controllerGen.Generate(crds); err != nil {
		t.Fatalf("ControllerGenerator.Generate failed: %v", err)
	}

	// Run compilation steps
	if err := runCompilationSteps(t, tmpDir); err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	t.Logf("Generated code from petstore.yaml compiles successfully (%d CRDs)", len(crds))
}

// runCompilationSteps runs go mod tidy, controller-gen, and go build
func runCompilationSteps(t *testing.T, dir string) error {
	t.Helper()

	env := append(os.Environ(), "GO111MODULE=on")

	// Step 0: Add replace directive for local openapi-operator-gen
	// Get the project root (relative to this test file: pkg/generator -> ../../)
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return fmt.Errorf("failed to get project root: %v", err)
	}
	t.Logf("Adding replace directive for local module: %s", projectRoot)
	replaceCmd := exec.Command("go", "mod", "edit", "-replace",
		fmt.Sprintf("github.com/bluecontainer/openapi-operator-gen=%s", projectRoot))
	replaceCmd.Dir = dir
	replaceCmd.Env = env
	if output, err := replaceCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod edit -replace failed: %v\nOutput: %s", err, output)
	}

	// Step 1: Run go mod tidy
	t.Log("Running go mod tidy...")
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = dir
	tidyCmd.Env = env
	if output, err := tidyCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod tidy failed: %v\nOutput: %s", err, output)
	}

	// Step 2: Install controller-gen if not present
	t.Log("Installing controller-gen...")
	localBin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(localBin, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %v", err)
	}

	controllerGenPath := filepath.Join(localBin, "controller-gen")
	installCmd := exec.Command("go", "install", "sigs.k8s.io/controller-tools/cmd/controller-gen@v0.17.0")
	installCmd.Dir = dir
	installCmd.Env = append(env, "GOBIN="+localBin)
	if output, err := installCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install controller-gen: %v\nOutput: %s", err, output)
	}

	// Step 3: Run controller-gen to generate DeepCopy methods
	t.Log("Running controller-gen to generate DeepCopy methods...")
	genCmd := exec.Command(controllerGenPath,
		"object:headerFile=hack/boilerplate.go.txt",
		"paths=./...",
	)
	genCmd.Dir = dir
	genCmd.Env = env
	if output, err := genCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("controller-gen failed: %v\nOutput: %s", err, output)
	}

	// Step 4: Run go build
	t.Log("Running go build...")
	buildCmd := exec.Command("go", "build", "-buildvcs=false", "./...")
	buildCmd.Dir = dir
	buildCmd.Env = env
	if output, err := buildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build failed: %v\nOutput: %s", err, output)
	}

	return nil
}

// runCompilationAndTestSteps runs compilation steps plus go test
func runCompilationAndTestSteps(t *testing.T, dir string) error {
	t.Helper()

	// First run compilation steps
	if err := runCompilationSteps(t, dir); err != nil {
		return err
	}

	env := append(os.Environ(), "GO111MODULE=on")

	// Step 5: Run go test
	t.Log("Running go test...")
	testCmd := exec.Command("go", "test", "-v", "./...")
	testCmd.Dir = dir
	testCmd.Env = env
	output, err := testCmd.CombinedOutput()
	t.Logf("Test output:\n%s", output)
	if err != nil {
		return fmt.Errorf("go test failed: %v\nOutput: %s", err, output)
	}

	return nil
}

// =============================================================================
// Option 4 & 5: E2E and HTTP Mock Test Templates
// =============================================================================

// TestGeneratedCodeIncludesTestHelpers verifies that generated code includes
// necessary components for E2E and HTTP mock testing
func TestGeneratedCodeIncludesTestHelpers(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "test.example.com",
		APIVersion: "v1alpha1",
		ModuleName: "github.com/example/test-operator",
	}

	crds := []*mapper.CRDDefinition{
		{
			APIGroup:   cfg.APIGroup,
			APIVersion: cfg.APIVersion,
			Kind:       "Widget",
			Plural:     "widgets",
			Scope:      "Namespaced",
			BasePath:   "/widgets",
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{Name: "Name", JSONName: "name", GoType: "string", Required: true},
				},
			},
		},
	}

	// Generate code
	typesGen := NewTypesGenerator(cfg)
	if err := typesGen.Generate(crds); err != nil {
		t.Fatalf("TypesGenerator.Generate failed: %v", err)
	}

	controllerGen := NewControllerGenerator(cfg)
	if err := controllerGen.Generate(crds); err != nil {
		t.Fatalf("ControllerGenerator.Generate failed: %v", err)
	}

	// Verify controller has necessary components for testing
	controllerPath := filepath.Join(tmpDir, "internal", "controller", "widget_controller.go")
	content, err := os.ReadFile(controllerPath)
	if err != nil {
		t.Fatalf("failed to read controller: %v", err)
	}
	contentStr := string(content)

	// Check for HTTPClient field (needed for HTTP mocking)
	if !strings.Contains(contentStr, "HTTPClient") {
		t.Error("expected HTTPClient field in controller for HTTP mock testing")
	}

	// Check for client.Client interface (needed for fake client testing)
	if !strings.Contains(contentStr, "client.Client") {
		t.Error("expected client.Client interface in controller for E2E testing")
	}

	// Check for Scheme field (needed for fake client setup)
	if !strings.Contains(contentStr, "Scheme") {
		t.Error("expected Scheme field in controller")
	}
}

// TestGenerateControllerTestFile generates a test file template for controllers
func TestGenerateControllerTestFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OutputDir:  tmpDir,
		APIGroup:   "test.example.com",
		APIVersion: "v1alpha1",
		ModuleName: "github.com/example/test-operator",
	}

	crds := []*mapper.CRDDefinition{
		{
			APIGroup:   cfg.APIGroup,
			APIVersion: cfg.APIVersion,
			Kind:       "Pet",
			Plural:     "pets",
			Scope:      "Namespaced",
			BasePath:   "/pet",
			Spec: &mapper.FieldDefinition{
				Fields: []*mapper.FieldDefinition{
					{Name: "Name", JSONName: "name", GoType: "string", Required: true},
				},
			},
		},
	}

	// Generate the controller
	controllerGen := NewControllerGenerator(cfg)
	if err := controllerGen.Generate(crds); err != nil {
		t.Fatalf("ControllerGenerator.Generate failed: %v", err)
	}

	// Generate test file for the controller
	testContent := generateControllerTestTemplate(cfg, crds[0])
	testPath := filepath.Join(tmpDir, "internal", "controller", "pet_controller_test.go")
	if err := os.WriteFile(testPath, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Verify test file was created
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Error("expected test file to exist")
	}

	// Verify test file content
	content, _ := os.ReadFile(testPath)
	contentStr := string(content)

	// Check for test components
	checks := []string{
		"package controller",
		"testing",
		"httptest",
		"fake.NewClientBuilder",
		"TestPetReconciler",
		"TestPetHTTPMock",
	}
	for _, check := range checks {
		if !strings.Contains(contentStr, check) {
			t.Errorf("expected %q in test file", check)
		}
	}
}

// generateControllerTestTemplate generates a test file template for a controller
func generateControllerTestTemplate(cfg *config.Config, crd *mapper.CRDDefinition) string {
	return `/*
Copyright 2024 Generated by openapi-operator-gen.
*/

package controller

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	` + cfg.APIVersion + ` "` + cfg.ModuleName + `/api/` + cfg.APIVersion + `"
)

// =============================================================================
// Option 4: E2E Testing with Fake Client
// =============================================================================

func TestPetReconciler_E2E(t *testing.T) {
	// Setup scheme
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = ` + cfg.APIVersion + `.AddToScheme(scheme)

	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Return existing resource
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   123,
				"name": "TestPet",
			})
		case http.MethodPost:
			// Return created resource
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   123,
				"name": "TestPet",
			})
		case http.MethodPut:
			// Return updated resource
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   123,
				"name": "UpdatedPet",
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	// Create test CR
	pet := &` + cfg.APIVersion + `.Pet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pet",
			Namespace: "default",
		},
		Spec: ` + cfg.APIVersion + `.PetSpec{
			Name: "TestPet",
		},
	}

	// Create fake client with the pet
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pet).
		WithStatusSubresource(pet).
		Build()

	// Create reconciler
	reconciler := &PetReconciler{
		Client:     fakeClient,
		Scheme:     scheme,
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
	}

	// Run reconcile
	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-pet",
			Namespace: "default",
		},
	})

	// Verify no error
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	// Verify result (should requeue for periodic sync)
	if result.RequeueAfter == 0 {
		t.Log("Reconcile completed without requeue")
	}

	// Verify status was updated
	var updatedPet ` + cfg.APIVersion + `.Pet
	if err := fakeClient.Get(ctx, types.NamespacedName{Name: "test-pet", Namespace: "default"}, &updatedPet); err != nil {
		t.Fatalf("failed to get updated pet: %v", err)
	}

	if updatedPet.Status.State == "" {
		t.Error("expected status.state to be set")
	}
}

func TestPetReconciler_CreateResource(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = ` + cfg.APIVersion + `.AddToScheme(scheme)

	// Track HTTP requests
	var receivedMethod string
	var receivedPath string
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&receivedBody)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   456,
			"name": "NewPet",
		})
	}))
	defer server.Close()

	pet := &` + cfg.APIVersion + `.Pet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "new-pet",
			Namespace: "default",
		},
		Spec: ` + cfg.APIVersion + `.PetSpec{
			Name: "NewPet",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pet).
		WithStatusSubresource(pet).
		Build()

	reconciler := &PetReconciler{
		Client:     fakeClient,
		Scheme:     scheme,
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
	}

	ctx := context.Background()
	_, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "new-pet",
			Namespace: "default",
		},
	})

	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	// Verify HTTP request was made correctly
	if receivedMethod != http.MethodPost {
		t.Errorf("expected POST request, got %s", receivedMethod)
	}

	t.Logf("HTTP request: %s %s", receivedMethod, receivedPath)
}

// =============================================================================
// Option 5: HTTP Mock Testing
// =============================================================================

func TestPetHTTPMock_GetResource(t *testing.T) {
	expectedResponse := map[string]interface{}{
		"id":     float64(123),
		"name":   "Fluffy",
		"status": "available",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/pet/123" {
			t.Errorf("expected /pet/123, got %s", r.URL.Path)
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	// Make request using the same HTTP client pattern as the controller
	client := server.Client()
	resp, err := client.Get(server.URL + "/pet/123")
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result["name"] != expectedResponse["name"] {
		t.Errorf("expected name %v, got %v", expectedResponse["name"], result["name"])
	}
}

func TestPetHTTPMock_CreateResource(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Capture request body
		json.NewDecoder(r.Body).Decode(&capturedBody)

		// Return created resource with ID
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		response := capturedBody
		response["id"] = float64(789)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create request
	reqBody := map[string]interface{}{
		"name":   "NewPet",
		"status": "available",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	client := server.Client()
	resp, err := client.Post(server.URL+"/pet", "application/json",
		&mockReader{data: bodyBytes})
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	// Verify captured body
	if capturedBody["name"] != "NewPet" {
		t.Errorf("expected name NewPet, got %v", capturedBody["name"])
	}
}

func TestPetHTTPMock_UpdateResource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/pet/123" {
			t.Errorf("expected /pet/123, got %s", r.URL.Path)
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	}))
	defer server.Close()

	reqBody := map[string]interface{}{
		"id":     float64(123),
		"name":   "UpdatedPet",
		"status": "sold",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPut, server.URL+"/pet/123", &mockReader{data: bodyBytes})
	req.Header.Set("Content-Type", "application/json")

	client := server.Client()
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestPetHTTPMock_DeleteResource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/pet/123" {
			t.Errorf("expected /pet/123, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/pet/123", nil)

	client := server.Client()
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("DELETE request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", resp.StatusCode)
	}
}

func TestPetHTTPMock_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		expectedError  bool
	}{
		{"NotFound", http.StatusNotFound, true},
		{"ServerError", http.StatusInternalServerError, true},
		{"BadRequest", http.StatusBadRequest, true},
		{"Unauthorized", http.StatusUnauthorized, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "test error",
				})
			}))
			defer server.Close()

			client := server.Client()
			resp, err := client.Get(server.URL + "/pet/999")
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.statusCode {
				t.Errorf("expected status %d, got %d", tt.statusCode, resp.StatusCode)
			}
		})
	}
}

func TestPetHTTPMock_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client with short timeout
	client := &http.Client{
		Timeout: 10 * time.Millisecond,
	}

	_, err := client.Get(server.URL + "/pet/123")
	if err == nil {
		t.Error("expected timeout error")
	}
}

// mockReader implements io.Reader for request bodies
type mockReader struct {
	data []byte
	pos  int
}

func (r *mockReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
`
}
