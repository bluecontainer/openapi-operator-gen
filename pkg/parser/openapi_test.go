package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewParser(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Fatal("expected non-nil parser")
	}
}

// =============================================================================
// toPascalCase Tests
// =============================================================================

func TestToPascalCase(t *testing.T) {
	p := &Parser{}

	tests := []struct {
		input    string
		expected string
	}{
		{"users", "Users"},
		{"user-profiles", "UserProfiles"},
		{"user_profiles", "UserProfiles"},
		{"user-profile-settings", "UserProfileSettings"},
		{"API", "Api"},
		{"api", "Api"},
		{"", ""},
		{"a", "A"},
		{"hello world", "HelloWorld"},
		{"HELLO_WORLD", "HelloWorld"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := p.toPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("toPascalCase(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// singularize Tests
// =============================================================================

func TestSingularize(t *testing.T) {
	p := &Parser{}

	tests := []struct {
		input    string
		expected string
	}{
		{"users", "user"},
		{"pets", "pet"},
		{"categories", "category"},
		{"entries", "entry"},
		{"boxes", "box"},
		{"buses", "bus"},
		{"class", "class"}, // ends in 'ss', should not singularize
		{"address", "address"},
		{"user", "user"}, // already singular
		{"", ""},
		{"a", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := p.singularize(tt.input)
			if result != tt.expected {
				t.Errorf("singularize(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// pluralize Tests
// =============================================================================

func TestPluralize(t *testing.T) {
	p := &Parser{}

	tests := []struct {
		input    string
		expected string
	}{
		{"user", "users"},
		{"pet", "pets"},
		{"category", "categories"},
		{"entry", "entries"},
		{"box", "boxes"},
		{"bus", "buses"},
		{"class", "classes"},
		{"match", "matches"},
		{"", "s"},
		{"a", "as"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := p.pluralize(tt.input)
			if result != tt.expected {
				t.Errorf("pluralize(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// extractResourceName Tests
// =============================================================================

func TestExtractResourceName(t *testing.T) {
	p := &Parser{}

	tests := []struct {
		path     string
		expected string
	}{
		{"/users", "User"},
		{"/users/{id}", "User"},
		{"/users/{userId}/posts", "User"},   // nested sub-resource: posts belong to users
		{"/api/v1/users", "User"},           // namespaced resource: users under api/v1
		{"/store/order", "Order"},           // namespaced resource: order under store
		{"/store/order/{orderId}", "Order"}, // namespaced resource with ID param
		{"/pets", "Pet"},
		{"/categories", "Category"},
		{"/user-profiles", "UserProfile"}, // singularize removes "s" from "profiles"
		{"/user_settings", "UserSetting"},
		{"/{id}", ""},       // only parameter
		{"/", ""},           // empty path
		{"", ""},            // empty string
		{"/items/", "Item"}, // trailing slash
		// Deeply nested paths with plural segments and singular param names
		{"/sharedmem/classes/{className}/instances/{instanceName}/variables/{variableName}", "Variable"},
		{"/classes/{className}", "Class"},
		{"/variables/{variableName}", "Variable"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := p.extractResourceName(tt.path)
			if result != tt.expected {
				t.Errorf("extractResourceName(%q) = %q, expected %q", tt.path, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// getBasePath Tests
// =============================================================================

func TestGetBasePath(t *testing.T) {
	p := &Parser{}

	tests := []struct {
		path     string
		expected string
	}{
		{"/users", "/users"},
		{"/users/{id}", "/users"},
		{"/users/{userId}/posts", "/users"},        // stops at {userId} since it's the resource ID
		{"/api/v1/users", "/api/v1/users"},         // includes full namespace path
		{"/store/order", "/store/order"},           // includes full namespace path
		{"/store/order/{orderId}", "/store/order"}, // stops at resource before ID param
		{"/", "/"},
		{"", "/"},
		{"/pets/", "/pets"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := p.getBasePath(tt.path)
			if result != tt.expected {
				t.Errorf("getBasePath(%q) = %q, expected %q", tt.path, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// extractRefName Tests
// =============================================================================

func TestExtractRefName(t *testing.T) {
	p := &Parser{}

	tests := []struct {
		ref      string
		expected string
	}{
		{"#/components/schemas/Pet", "Pet"},
		{"#/components/schemas/User", "User"},
		{"#/components/schemas/ApiResponse", "ApiResponse"},
		{"#/definitions/Pet", "Pet"},
		{"Pet", ""}, // no fragment
		{"", ""},    // empty
		{"#/", ""},  // empty fragment parts
		{"#/components", "components"},
		{"invalid://url/%", ""}, // invalid URL
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			result := p.extractRefName(tt.ref)
			if result != tt.expected {
				t.Errorf("extractRefName(%q) = %q, expected %q", tt.ref, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// parseStatusCode Tests
// =============================================================================

func TestParseStatusCode(t *testing.T) {
	p := &Parser{}

	tests := []struct {
		code     string
		expected int
	}{
		{"200", 200},
		{"201", 201},
		{"404", 200}, // unknown defaults to 200
		{"500", 200},
		{"", 200},
		{"abc", 200},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := p.parseStatusCode(tt.code)
			if result != tt.expected {
				t.Errorf("parseStatusCode(%q) = %d, expected %d", tt.code, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Integration Tests with OpenAPI Spec Files
// =============================================================================

func TestParse_SimpleSpec(t *testing.T) {
	// Create a temporary OpenAPI spec file
	specContent := `
openapi: "3.0.0"
info:
  title: "Test API"
  version: "1.0.0"
  description: "A test API"
servers:
  - url: "https://api.example.com/v1"
paths:
  /users:
    get:
      operationId: getUsers
      summary: Get all users
      responses:
        "200":
          description: Success
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/User'
    post:
      operationId: createUser
      summary: Create a user
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/User'
      responses:
        "201":
          description: Created
  /users/{id}:
    get:
      operationId: getUserById
      summary: Get user by ID
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: Success
components:
  schemas:
    User:
      type: object
      required:
        - name
        - email
      properties:
        id:
          type: integer
          format: int64
        name:
          type: string
          minLength: 1
          maxLength: 100
        email:
          type: string
          format: email
        age:
          type: integer
          minimum: 0
          maximum: 150
        status:
          type: string
          enum:
            - active
            - inactive
            - pending
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify basic info
	if spec.Title != "Test API" {
		t.Errorf("expected Title 'Test API', got %q", spec.Title)
	}
	if spec.Version != "1.0.0" {
		t.Errorf("expected Version '1.0.0', got %q", spec.Version)
	}
	if spec.Description != "A test API" {
		t.Errorf("expected Description 'A test API', got %q", spec.Description)
	}
	if spec.BaseURL != "https://api.example.com/v1" {
		t.Errorf("expected BaseURL 'https://api.example.com/v1', got %q", spec.BaseURL)
	}

	// Verify schemas
	if len(spec.Schemas) != 1 {
		t.Errorf("expected 1 schema, got %d", len(spec.Schemas))
	}
	userSchema, ok := spec.Schemas["User"]
	if !ok {
		t.Fatal("User schema not found")
	}
	if userSchema.Type != "object" {
		t.Errorf("expected User schema type 'object', got %q", userSchema.Type)
	}
	if len(userSchema.Properties) != 5 {
		t.Errorf("expected 5 properties, got %d", len(userSchema.Properties))
	}
	if len(userSchema.Required) != 2 {
		t.Errorf("expected 2 required fields, got %d", len(userSchema.Required))
	}

	// Verify resources
	if len(spec.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(spec.Resources))
	}
	userResource := spec.Resources[0]
	if userResource.Name != "User" {
		t.Errorf("expected resource name 'User', got %q", userResource.Name)
	}
	if userResource.PluralName != "Users" {
		t.Errorf("expected plural name 'Users', got %q", userResource.PluralName)
	}
	if userResource.Path != "/users" {
		t.Errorf("expected path '/users', got %q", userResource.Path)
	}

	// Verify operations - only GET and POST on /users are part of the resource
	// GET /users/{id} is now a QueryEndpoint (GET-only)
	if len(userResource.Operations) != 2 {
		t.Errorf("expected 2 operations, got %d", len(userResource.Operations))
	}

	// Verify that GET /users/{id} is now a query endpoint
	if len(spec.QueryEndpoints) != 1 {
		t.Errorf("expected 1 query endpoint, got %d", len(spec.QueryEndpoints))
	}

	// Check that schema was extracted
	if userResource.Schema == nil {
		t.Error("expected resource schema to be set")
	}
}

func TestParse_MultipleResources(t *testing.T) {
	specContent := `
openapi: "3.0.0"
info:
  title: "Multi Resource API"
  version: "1.0.0"
paths:
  /users:
    get:
      operationId: getUsers
      responses:
        "200":
          description: Success
  /pets:
    get:
      operationId: getPets
      responses:
        "200":
          description: Success
    post:
      operationId: createPet
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
      responses:
        "201":
          description: Created
  /orders:
    get:
      operationId: getOrders
      responses:
        "200":
          description: Success
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Only /pets is a Resource (GET+POST), /users and /orders are QueryEndpoints (GET-only)
	if len(spec.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(spec.Resources))
	}

	if spec.Resources[0].Name != "Pet" {
		t.Errorf("expected resource name 'Pet', got %q", spec.Resources[0].Name)
	}

	// /users and /orders are GET-only, so they're QueryEndpoints
	if len(spec.QueryEndpoints) != 2 {
		t.Errorf("expected 2 query endpoints, got %d", len(spec.QueryEndpoints))
	}
}

func TestParse_WithParameters(t *testing.T) {
	specContent := `
openapi: "3.0.0"
info:
  title: "API with Parameters"
  version: "1.0.0"
paths:
  /items/{id}:
    get:
      operationId: getItem
      parameters:
        - name: id
          in: path
          required: true
          description: Item ID
          schema:
            type: string
        - name: include
          in: query
          required: false
          description: Fields to include
          schema:
            type: string
        - name: limit
          in: query
          required: false
          schema:
            type: integer
      responses:
        "200":
          description: Success
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// /items/{id} with GET-only is now a QueryEndpoint, not a Resource
	if len(spec.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(spec.Resources))
	}

	if len(spec.QueryEndpoints) != 1 {
		t.Fatalf("expected 1 query endpoint, got %d", len(spec.QueryEndpoints))
	}

	qe := spec.QueryEndpoints[0]
	if qe.Path != "/items/{id}" {
		t.Errorf("expected path '/items/{id}', got %q", qe.Path)
	}

	// Query params should be captured
	if len(qe.QueryParams) != 2 {
		t.Errorf("expected 2 query params, got %d", len(qe.QueryParams))
	}
}

func TestParse_SchemaValidation(t *testing.T) {
	specContent := `
openapi: "3.0.0"
info:
  title: "Validation API"
  version: "1.0.0"
paths:
  /items:
    post:
      operationId: createItem
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Item'
      responses:
        "201":
          description: Created
components:
  schemas:
    Item:
      type: object
      properties:
        name:
          type: string
          minLength: 1
          maxLength: 255
          pattern: "^[a-zA-Z0-9]+$"
        price:
          type: number
          minimum: 0
          maximum: 10000
        tags:
          type: array
          minItems: 1
          maxItems: 10
          items:
            type: string
        status:
          type: string
          enum:
            - available
            - sold
            - pending
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	itemSchema, ok := spec.Schemas["Item"]
	if !ok {
		t.Fatal("Item schema not found")
	}

	// Check name field validation
	nameSchema := itemSchema.Properties["name"]
	if nameSchema == nil {
		t.Fatal("name property not found")
	}
	if nameSchema.MinLength == nil || *nameSchema.MinLength != 1 {
		t.Error("expected MinLength 1")
	}
	if nameSchema.MaxLength == nil || *nameSchema.MaxLength != 255 {
		t.Error("expected MaxLength 255")
	}
	if nameSchema.Pattern != "^[a-zA-Z0-9]+$" {
		t.Errorf("expected pattern, got %q", nameSchema.Pattern)
	}

	// Check price field validation
	priceSchema := itemSchema.Properties["price"]
	if priceSchema == nil {
		t.Fatal("price property not found")
	}
	if priceSchema.Minimum == nil || *priceSchema.Minimum != 0 {
		t.Error("expected Minimum 0")
	}
	if priceSchema.Maximum == nil || *priceSchema.Maximum != 10000 {
		t.Error("expected Maximum 10000")
	}

	// Check tags field validation
	tagsSchema := itemSchema.Properties["tags"]
	if tagsSchema == nil {
		t.Fatal("tags property not found")
	}
	if tagsSchema.MinItems == nil || *tagsSchema.MinItems != 1 {
		t.Error("expected MinItems 1")
	}
	if tagsSchema.MaxItems == nil || *tagsSchema.MaxItems != 10 {
		t.Error("expected MaxItems 10")
	}
	if tagsSchema.Items == nil {
		t.Error("expected Items to be set")
	}

	// Check enum
	statusSchema := itemSchema.Properties["status"]
	if statusSchema == nil {
		t.Fatal("status property not found")
	}
	if len(statusSchema.Enum) != 3 {
		t.Errorf("expected 3 enum values, got %d", len(statusSchema.Enum))
	}
}

func TestParse_NestedObjects(t *testing.T) {
	specContent := `
openapi: "3.0.0"
info:
  title: "Nested Objects API"
  version: "1.0.0"
paths:
  /users:
    get:
      responses:
        "200":
          description: Success
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
        address:
          type: object
          properties:
            street:
              type: string
            city:
              type: string
            location:
              type: object
              properties:
                lat:
                  type: number
                lng:
                  type: number
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	userSchema := spec.Schemas["User"]
	if userSchema == nil {
		t.Fatal("User schema not found")
	}

	addressSchema := userSchema.Properties["address"]
	if addressSchema == nil {
		t.Fatal("address property not found")
	}
	if addressSchema.Type != "object" {
		t.Errorf("expected address type 'object', got %q", addressSchema.Type)
	}

	locationSchema := addressSchema.Properties["location"]
	if locationSchema == nil {
		t.Fatal("location property not found")
	}
	if len(locationSchema.Properties) != 2 {
		t.Errorf("expected 2 properties in location, got %d", len(locationSchema.Properties))
	}

	latSchema := locationSchema.Properties["lat"]
	if latSchema == nil || latSchema.Type != "number" {
		t.Error("expected lat property with type number")
	}
}

func TestParse_ArrayTypes(t *testing.T) {
	specContent := `
openapi: "3.0.0"
info:
  title: "Array Types API"
  version: "1.0.0"
paths:
  /data:
    get:
      responses:
        "200":
          description: Success
components:
  schemas:
    Data:
      type: object
      properties:
        stringArray:
          type: array
          items:
            type: string
        intArray:
          type: array
          items:
            type: integer
            format: int64
        objectArray:
          type: array
          items:
            type: object
            properties:
              id:
                type: integer
              name:
                type: string
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	dataSchema := spec.Schemas["Data"]
	if dataSchema == nil {
		t.Fatal("Data schema not found")
	}

	// String array
	stringArraySchema := dataSchema.Properties["stringArray"]
	if stringArraySchema == nil {
		t.Fatal("stringArray not found")
	}
	if stringArraySchema.Type != "array" {
		t.Errorf("expected type 'array', got %q", stringArraySchema.Type)
	}
	if stringArraySchema.Items == nil || stringArraySchema.Items.Type != "string" {
		t.Error("expected Items with type string")
	}

	// Int array
	intArraySchema := dataSchema.Properties["intArray"]
	if intArraySchema == nil {
		t.Fatal("intArray not found")
	}
	if intArraySchema.Items == nil || intArraySchema.Items.Type != "integer" {
		t.Error("expected Items with type integer")
	}
	if intArraySchema.Items.Format != "int64" {
		t.Errorf("expected format 'int64', got %q", intArraySchema.Items.Format)
	}

	// Object array
	objectArraySchema := dataSchema.Properties["objectArray"]
	if objectArraySchema == nil {
		t.Fatal("objectArray not found")
	}
	if objectArraySchema.Items == nil || objectArraySchema.Items.Type != "object" {
		t.Error("expected Items with type object")
	}
	if len(objectArraySchema.Items.Properties) != 2 {
		t.Errorf("expected 2 properties in item, got %d", len(objectArraySchema.Items.Properties))
	}
}

func TestParse_NoServers(t *testing.T) {
	specContent := `
openapi: "3.0.0"
info:
  title: "No Servers API"
  version: "1.0.0"
paths:
  /test:
    get:
      responses:
        "200":
          description: Success
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if spec.BaseURL != "" {
		t.Errorf("expected empty BaseURL, got %q", spec.BaseURL)
	}
}

func TestParse_NoDescription(t *testing.T) {
	specContent := `
openapi: "3.0.0"
info:
  title: "No Description API"
  version: "1.0.0"
paths:
  /test:
    get:
      responses:
        "200":
          description: Success
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if spec.Description != "" {
		t.Errorf("expected empty Description, got %q", spec.Description)
	}
}

func TestParse_InvalidFile(t *testing.T) {
	p := NewParser()
	_, err := p.Parse("/nonexistent/path/openapi.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(specPath, []byte("not: valid: yaml: content:::"), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	_, err := p.Parse(specPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParse_InvalidOpenAPISpec(t *testing.T) {
	specContent := `
openapi: "3.0.0"
info:
  title: "Invalid API"
  # Missing required 'version' field
paths: {}
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	_, err := p.Parse(specPath)
	if err == nil {
		t.Error("expected error for invalid OpenAPI spec")
	}
}

func TestParse_EmptyPaths(t *testing.T) {
	specContent := `
openapi: "3.0.0"
info:
  title: "Empty Paths API"
  version: "1.0.0"
paths: {}
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(spec.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(spec.Resources))
	}
}

func TestParse_RequestResponseBodies(t *testing.T) {
	specContent := `
openapi: "3.0.0"
info:
  title: "Request/Response Bodies API"
  version: "1.0.0"
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: Success
    post:
      operationId: createItem
      summary: Create an item
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: integer
                  name:
                    type: string
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(spec.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(spec.Resources))
	}

	resource := spec.Resources[0]
	if len(resource.Operations) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(resource.Operations))
	}

	// Find the POST operation (createItem)
	var postOp *Operation
	for i := range resource.Operations {
		if resource.Operations[i].Method == "POST" {
			postOp = &resource.Operations[i]
			break
		}
	}
	if postOp == nil {
		t.Fatal("expected POST operation")
	}

	// Check request body
	if postOp.RequestBody == nil {
		t.Error("expected RequestBody to be set")
	} else {
		if len(postOp.RequestBody.Properties) != 1 {
			t.Errorf("expected 1 property in request body, got %d", len(postOp.RequestBody.Properties))
		}
	}

	// Check response body
	if postOp.ResponseBody == nil {
		t.Error("expected ResponseBody to be set")
	} else {
		if len(postOp.ResponseBody.Properties) != 2 {
			t.Errorf("expected 2 properties in response body, got %d", len(postOp.ResponseBody.Properties))
		}
	}
}

func TestParse_AllHTTPMethods(t *testing.T) {
	specContent := `
openapi: "3.0.0"
info:
  title: "All Methods API"
  version: "1.0.0"
paths:
  /resources/{id}:
    parameters:
      - name: id
        in: path
        required: true
        schema:
          type: string
    get:
      operationId: getResource
      responses:
        "200":
          description: Success
    post:
      operationId: createResource
      responses:
        "201":
          description: Created
    put:
      operationId: updateResource
      responses:
        "200":
          description: Success
    patch:
      operationId: patchResource
      responses:
        "200":
          description: Success
    delete:
      operationId: deleteResource
      responses:
        "204":
          description: Deleted
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(spec.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(spec.Resources))
	}

	resource := spec.Resources[0]
	if len(resource.Operations) != 5 {
		t.Errorf("expected 5 operations, got %d", len(resource.Operations))
	}

	methods := make(map[string]bool)
	for _, op := range resource.Operations {
		methods[op.Method] = true
	}

	expectedMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	for _, m := range expectedMethods {
		if !methods[m] {
			t.Errorf("expected method %s to be present", m)
		}
	}
}

func TestParse_TypeInference(t *testing.T) {
	specContent := `
openapi: "3.0.0"
info:
  title: "Type Inference API"
  version: "1.0.0"
paths:
  /test:
    get:
      responses:
        "200":
          description: Success
components:
  schemas:
    InferredObject:
      properties:
        name:
          type: string
    InferredArray:
      items:
        type: string
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Object type should be inferred from properties
	objSchema := spec.Schemas["InferredObject"]
	if objSchema == nil {
		t.Fatal("InferredObject schema not found")
	}
	if objSchema.Type != "object" {
		t.Errorf("expected inferred type 'object', got %q", objSchema.Type)
	}

	// Array type should be inferred from items
	arrSchema := spec.Schemas["InferredArray"]
	if arrSchema == nil {
		t.Fatal("InferredArray schema not found")
	}
	if arrSchema.Type != "array" {
		t.Errorf("expected inferred type 'array', got %q", arrSchema.Type)
	}
}

func TestParse_NullableField(t *testing.T) {
	specContent := `
openapi: "3.0.0"
info:
  title: "Nullable API"
  version: "1.0.0"
paths:
  /test:
    get:
      responses:
        "200":
          description: Success
components:
  schemas:
    NullableTest:
      type: object
      properties:
        normalField:
          type: string
        nullableField:
          type: string
          nullable: true
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	schema := spec.Schemas["NullableTest"]
	if schema == nil {
		t.Fatal("NullableTest schema not found")
	}

	normalField := schema.Properties["normalField"]
	if normalField == nil {
		t.Fatal("normalField not found")
	}
	if normalField.Nullable {
		t.Error("expected normalField to not be nullable")
	}

	nullableField := schema.Properties["nullableField"]
	if nullableField == nil {
		t.Fatal("nullableField not found")
	}
	if !nullableField.Nullable {
		t.Error("expected nullableField to be nullable")
	}
}

func TestParse_DefaultValues(t *testing.T) {
	specContent := `
openapi: "3.0.0"
info:
  title: "Default Values API"
  version: "1.0.0"
paths:
  /test:
    get:
      responses:
        "200":
          description: Success
components:
  schemas:
    DefaultsTest:
      type: object
      properties:
        count:
          type: integer
          default: 10
        enabled:
          type: boolean
          default: true
        name:
          type: string
          default: "default_name"
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	schema := spec.Schemas["DefaultsTest"]
	if schema == nil {
		t.Fatal("DefaultsTest schema not found")
	}

	countField := schema.Properties["count"]
	if countField == nil {
		t.Fatal("count field not found")
	}
	if countField.Default == nil {
		t.Error("expected count to have default value")
	}

	enabledField := schema.Properties["enabled"]
	if enabledField == nil {
		t.Fatal("enabled field not found")
	}
	if enabledField.Default == nil {
		t.Error("expected enabled to have default value")
	}

	nameField := schema.Properties["name"]
	if nameField == nil {
		t.Fatal("name field not found")
	}
	if nameField.Default == nil {
		t.Error("expected name to have default value")
	}
}

// =============================================================================
// isResourceIDPath Tests
// =============================================================================

func TestIsResourceIDPath(t *testing.T) {
	p := &Parser{}

	tests := []struct {
		path     string
		expected bool
	}{
		{"/pet/{petId}", true},
		{"/store/order/{orderId}", true},
		{"/users/{id}", true},
		{"/users/{userId}", true},
		{"/api/v1/users/{userId}", true},
		{"/pet", false},                     // No ID parameter
		{"/store/order", false},             // No ID parameter
		{"/{id}", false},                    // Only 1 segment
		{"/pet/{petId}/uploadImage", false}, // ID param not at end
		{"/pet/findByStatus", false},        // No ID parameter
		{"/user/{userId}/posts", false},     // Extra segment after ID
		{"/", false},                        // Root path
		{"", false},                         // Empty path
		{"/pet/{randomId}", false},          // ID param doesn't match resource name
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := p.isResourceIDPath(tt.path)
			if result != tt.expected {
				t.Errorf("isResourceIDPath(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// getBasePathForIDPath Tests
// =============================================================================

func TestGetBasePathForIDPath(t *testing.T) {
	p := &Parser{}

	tests := []struct {
		path     string
		expected string
	}{
		{"/pet/{petId}", "/pet"},
		{"/store/order/{orderId}", "/store/order"},
		{"/users/{id}", "/users"},
		{"/api/v1/users/{userId}", "/api/v1/users"},
		{"/pet", ""},                     // Not an ID path
		{"/store/order", ""},             // Not an ID path
		{"/pet/{petId}/uploadImage", ""}, // Not an ID path (ID not at end)
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := p.getBasePathForIDPath(tt.path)
			if result != tt.expected {
				t.Errorf("getBasePathForIDPath(%q) = %q, expected %q", tt.path, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Combined Path Tests
// =============================================================================

func TestParse_CombinedPaths(t *testing.T) {
	// Test that /pet (POST) and /pet/{petId} (GET/PUT/DELETE) are combined into one resource
	specContent := `
openapi: "3.0.0"
info:
  title: "Combined Paths API"
  version: "1.0.0"
paths:
  /pet:
    post:
      operationId: createPet
      summary: Create a pet
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
      responses:
        "201":
          description: Created
  /pet/{petId}:
    get:
      operationId: getPet
      parameters:
        - name: petId
          in: path
          required: true
          schema:
            type: integer
      responses:
        "200":
          description: Success
    put:
      operationId: updatePet
      parameters:
        - name: petId
          in: path
          required: true
          schema:
            type: integer
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
      responses:
        "200":
          description: Updated
    delete:
      operationId: deletePet
      parameters:
        - name: petId
          in: path
          required: true
          schema:
            type: integer
      responses:
        "204":
          description: Deleted
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should have exactly 1 resource (Pet), not 2
	if len(spec.Resources) != 1 {
		t.Fatalf("expected 1 resource (combined), got %d", len(spec.Resources))
	}

	resource := spec.Resources[0]
	if resource.Name != "Pet" {
		t.Errorf("expected resource name 'Pet', got %q", resource.Name)
	}

	// Should have all 4 operations (POST from /pet, GET/PUT/DELETE from /pet/{petId})
	if len(resource.Operations) != 4 {
		t.Errorf("expected 4 operations, got %d", len(resource.Operations))
	}

	methods := make(map[string]bool)
	for _, op := range resource.Operations {
		methods[op.Method] = true
	}

	expectedMethods := []string{"POST", "GET", "PUT", "DELETE"}
	for _, m := range expectedMethods {
		if !methods[m] {
			t.Errorf("expected method %s to be present", m)
		}
	}

	// Should NOT be classified as an ActionEndpoint
	if len(spec.ActionEndpoints) != 0 {
		t.Errorf("expected 0 action endpoints, got %d", len(spec.ActionEndpoints))
	}
}

func TestParse_CombinedPathsWithQueryEndpoints(t *testing.T) {
	// Test that combined paths work alongside query endpoints
	specContent := `
openapi: "3.0.0"
info:
  title: "Combined with Query API"
  version: "1.0.0"
paths:
  /pet:
    post:
      operationId: createPet
      responses:
        "201":
          description: Created
  /pet/{petId}:
    get:
      operationId: getPet
      parameters:
        - name: petId
          in: path
          required: true
          schema:
            type: integer
      responses:
        "200":
          description: Success
    delete:
      operationId: deletePet
      parameters:
        - name: petId
          in: path
          required: true
          schema:
            type: integer
      responses:
        "204":
          description: Deleted
  /pet/findByStatus:
    get:
      operationId: findPetsByStatus
      parameters:
        - name: status
          in: query
          schema:
            type: string
      responses:
        "200":
          description: Success
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should have 1 resource (Pet)
	if len(spec.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(spec.Resources))
	}

	resource := spec.Resources[0]
	if resource.Name != "Pet" {
		t.Errorf("expected resource name 'Pet', got %q", resource.Name)
	}

	// Should have 3 operations (POST, GET, DELETE)
	if len(resource.Operations) != 3 {
		t.Errorf("expected 3 operations, got %d", len(resource.Operations))
	}

	// Should have 1 query endpoint (findByStatus)
	if len(spec.QueryEndpoints) != 1 {
		t.Errorf("expected 1 query endpoint, got %d", len(spec.QueryEndpoints))
	}
}

func TestParse_PostOnlyWithoutIDPath_IsAction(t *testing.T) {
	// Test that POST-only endpoints WITHOUT a corresponding ID path are still actions
	specContent := `
openapi: "3.0.0"
info:
  title: "Action Only API"
  version: "1.0.0"
paths:
  /store:
    post:
      operationId: createStore
      responses:
        "201":
          description: Created
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should have 0 resources (no corresponding ID path)
	if len(spec.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(spec.Resources))
	}

	// Should have 1 action endpoint
	if len(spec.ActionEndpoints) != 1 {
		t.Errorf("expected 1 action endpoint, got %d", len(spec.ActionEndpoints))
	}
}

// =============================================================================
// isURL Tests
// =============================================================================

func TestIsURL(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"https://example.com/api/openapi.yaml", true},
		{"http://localhost:8080/api.json", true},
		{"https://raw.githubusercontent.com/user/repo/main/openapi.yaml", true},
		{"./openapi.yaml", false},
		{"/home/user/openapi.yaml", false},
		{"openapi.yaml", false},
		{"file:///path/to/openapi.yaml", false},
		{"ftp://example.com/api.yaml", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isURL(tt.path)
			if result != tt.expected {
				t.Errorf("isURL(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// URL Loading Integration Tests
// =============================================================================

func TestParse_FromURL(t *testing.T) {
	// Skip this test if running in short mode (no network)
	if testing.Short() {
		t.Skip("skipping URL test in short mode")
	}

	// Use a well-known public OpenAPI spec (Petstore)
	specURL := "https://petstore3.swagger.io/api/v3/openapi.json"

	p := NewParser()
	spec, err := p.Parse(specURL)
	if err != nil {
		// If the URL is unreachable, skip the test rather than fail
		t.Skipf("could not load spec from URL (network issue?): %v", err)
	}

	// Verify we got some data
	if spec.Title == "" {
		t.Error("expected Title to be set")
	}
	if spec.Version == "" {
		t.Error("expected Version to be set")
	}

	// Petstore should have resources
	if len(spec.Resources) == 0 && len(spec.QueryEndpoints) == 0 && len(spec.ActionEndpoints) == 0 {
		t.Error("expected at least some resources, queries, or actions")
	}
}

func TestParse_InvalidURL(t *testing.T) {
	p := NewParser()

	// Test with an invalid URL scheme
	_, err := p.Parse("https://invalid-domain-that-does-not-exist-12345.example/api.yaml")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestParse_MalformedURL(t *testing.T) {
	p := NewParser()

	// Test with a malformed URL (still has http:// prefix but invalid format)
	// Note: url.Parse is very permissive, so we test the actual fetch failure
	_, err := p.Parse("http://[::1]:namedport/api.yaml")
	if err == nil {
		t.Error("expected error for malformed URL")
	}
}

// =============================================================================
// Swagger 2.0 Support Tests
// =============================================================================

func TestDetectSpecVersion_Swagger2(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "Swagger 2.0 JSON",
			content:  `{"swagger": "2.0", "info": {"title": "Test", "version": "1.0"}}`,
			expected: "2.0",
		},
		{
			name:     "Swagger 2.0 YAML",
			content:  "swagger: \"2.0\"\ninfo:\n  title: Test\n  version: \"1.0\"",
			expected: "2.0",
		},
		{
			name:     "OpenAPI 3.0 JSON",
			content:  `{"openapi": "3.0.0", "info": {"title": "Test", "version": "1.0"}}`,
			expected: "3.x",
		},
		{
			name:     "OpenAPI 3.0 YAML",
			content:  "openapi: \"3.0.0\"\ninfo:\n  title: Test\n  version: \"1.0\"",
			expected: "3.x",
		},
		{
			name:     "OpenAPI 3.1 YAML",
			content:  "openapi: \"3.1.0\"\ninfo:\n  title: Test\n  version: \"1.0\"",
			expected: "3.x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectSpecVersion([]byte(tt.content))
			if result != tt.expected {
				t.Errorf("detectSpecVersion() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestParse_Swagger2Spec(t *testing.T) {
	// Create a temporary Swagger 2.0 spec file
	specContent := `
swagger: "2.0"
info:
  title: "Pet Store API"
  version: "1.0.0"
  description: "A sample Swagger 2.0 API"
host: "api.example.com"
basePath: "/v1"
schemes:
  - https
paths:
  /pets:
    get:
      summary: "List all pets"
      operationId: "listPets"
      produces:
        - application/json
      responses:
        200:
          description: "A list of pets"
          schema:
            type: array
            items:
              $ref: "#/definitions/Pet"
    post:
      summary: "Create a pet"
      operationId: "createPet"
      consumes:
        - application/json
      produces:
        - application/json
      parameters:
        - in: body
          name: pet
          schema:
            $ref: "#/definitions/Pet"
      responses:
        201:
          description: "Pet created"
          schema:
            $ref: "#/definitions/Pet"
  /pets/{petId}:
    get:
      summary: "Get a pet by ID"
      operationId: "getPet"
      produces:
        - application/json
      parameters:
        - name: petId
          in: path
          required: true
          type: integer
          format: int64
      responses:
        200:
          description: "A pet"
          schema:
            $ref: "#/definitions/Pet"
    put:
      summary: "Update a pet"
      operationId: "updatePet"
      consumes:
        - application/json
      produces:
        - application/json
      parameters:
        - name: petId
          in: path
          required: true
          type: integer
          format: int64
        - in: body
          name: pet
          schema:
            $ref: "#/definitions/Pet"
      responses:
        200:
          description: "Pet updated"
          schema:
            $ref: "#/definitions/Pet"
    delete:
      summary: "Delete a pet"
      operationId: "deletePet"
      parameters:
        - name: petId
          in: path
          required: true
          type: integer
          format: int64
      responses:
        204:
          description: "Pet deleted"
definitions:
  Pet:
    type: object
    required:
      - name
    properties:
      id:
        type: integer
        format: int64
      name:
        type: string
      tag:
        type: string
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "swagger.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify basic info was parsed
	if spec.Title != "Pet Store API" {
		t.Errorf("expected Title 'Pet Store API', got %q", spec.Title)
	}
	if spec.Version != "1.0.0" {
		t.Errorf("expected Version '1.0.0', got %q", spec.Version)
	}

	// Verify resources were extracted
	if len(spec.Resources) == 0 {
		t.Error("expected at least one resource")
	}

	// Find Pet resource
	var petResource *Resource
	for _, r := range spec.Resources {
		if r.Name == "Pet" {
			petResource = r
			break
		}
	}

	if petResource == nil {
		t.Fatal("expected Pet resource")
	}

	// Verify Pet resource has operations
	if len(petResource.Operations) == 0 {
		t.Error("expected Pet resource to have operations")
	}

	// Verify schemas were parsed
	if len(spec.Schemas) == 0 {
		t.Error("expected at least one schema")
	}

	if _, ok := spec.Schemas["Pet"]; !ok {
		t.Error("expected Pet schema to be parsed")
	}
}

func TestParse_Swagger2JSON(t *testing.T) {
	// Create a temporary Swagger 2.0 JSON spec
	specContent := `{
  "swagger": "2.0",
  "info": {
    "title": "Simple API",
    "version": "1.0.0"
  },
  "paths": {
    "/users": {
      "get": {
        "summary": "List users",
        "operationId": "listUsers",
        "responses": {
          "200": {
            "description": "Success"
          }
        }
      },
      "post": {
        "summary": "Create user",
        "operationId": "createUser",
        "responses": {
          "201": {
            "description": "Created"
          }
        }
      }
    },
    "/users/{userId}": {
      "get": {
        "summary": "Get user",
        "operationId": "getUser",
        "parameters": [
          {
            "name": "userId",
            "in": "path",
            "required": true,
            "type": "string"
          }
        ],
        "responses": {
          "200": {
            "description": "Success"
          }
        }
      }
    }
  }
}`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "swagger.json")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if spec.Title != "Simple API" {
		t.Errorf("expected Title 'Simple API', got %q", spec.Title)
	}

	// Should have User resource
	found := false
	for _, r := range spec.Resources {
		if r.Name == "User" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected User resource to be extracted")
	}
}

func TestParse_Swagger2WithQueryParams(t *testing.T) {
	specContent := `
swagger: "2.0"
info:
  title: "API with Query Params"
  version: "1.0.0"
paths:
  /pets/findByStatus:
    get:
      summary: "Find pets by status"
      operationId: "findPetsByStatus"
      parameters:
        - name: status
          in: query
          required: true
          type: string
          enum:
            - available
            - pending
            - sold
      responses:
        200:
          description: "Success"
          schema:
            type: array
            items:
              type: object
              properties:
                id:
                  type: integer
                name:
                  type: string
`

	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "swagger.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	p := NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should have a query endpoint for findByStatus
	if len(spec.QueryEndpoints) == 0 {
		t.Error("expected at least one query endpoint")
	}

	// Find the query endpoint
	var queryEndpoint *QueryEndpoint
	for _, qe := range spec.QueryEndpoints {
		if qe.Path == "/pets/findByStatus" {
			queryEndpoint = qe
			break
		}
	}

	if queryEndpoint == nil {
		t.Fatal("expected query endpoint for /pets/findByStatus")
	}

	// Verify query params were extracted
	if len(queryEndpoint.QueryParams) == 0 {
		t.Error("expected query parameters to be extracted")
	}

	// Verify status parameter exists
	found := false
	for _, p := range queryEndpoint.QueryParams {
		if p.Name == "status" {
			found = true
			if !p.Required {
				t.Error("expected status parameter to be required")
			}
			break
		}
	}
	if !found {
		t.Error("expected status query parameter")
	}
}

func TestParse_Swagger2FromURL(t *testing.T) {
	// Skip this test if running in short mode (no network)
	if testing.Short() {
		t.Skip("skipping URL test in short mode")
	}

	// Use the classic Swagger 2.0 Petstore
	specURL := "https://petstore.swagger.io/v2/swagger.json"

	p := NewParser()
	spec, err := p.Parse(specURL)
	if err != nil {
		// If the URL is unreachable, skip the test rather than fail
		t.Skipf("could not load spec from URL (network issue?): %v", err)
	}

	// Verify we got some data
	if spec.Title == "" {
		t.Error("expected Title to be set")
	}
	if spec.Version == "" {
		t.Error("expected Version to be set")
	}

	// Petstore should have resources
	if len(spec.Resources) == 0 && len(spec.QueryEndpoints) == 0 && len(spec.ActionEndpoints) == 0 {
		t.Error("expected at least some resources, queries, or actions")
	}
}
