package config

import (
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantErr     bool
		errField    string
		wantVersion string
		wantMode    MappingMode
		wantModule  string
	}{
		{
			name:     "missing SpecPath",
			config:   Config{OutputDir: "/out", APIGroup: "test.example.com"},
			wantErr:  true,
			errField: "SpecPath",
		},
		{
			name:     "missing OutputDir",
			config:   Config{SpecPath: "/spec.yaml", APIGroup: "test.example.com"},
			wantErr:  true,
			errField: "OutputDir",
		},
		{
			name:     "missing APIGroup",
			config:   Config{SpecPath: "/spec.yaml", OutputDir: "/out"},
			wantErr:  true,
			errField: "APIGroup",
		},
		{
			name: "valid config with defaults",
			config: Config{
				SpecPath:  "/petstore.yaml",
				OutputDir: "/out",
				APIGroup:  "test.example.com",
			},
			wantErr:     false,
			wantVersion: "v1alpha1",
			wantMode:    PerResource,
			wantModule:  "github.com/bluecontainer/generated-operator",
		},
		{
			name: "valid config with explicit values",
			config: Config{
				SpecPath:    "/spec.yaml",
				OutputDir:   "/out",
				APIGroup:    "test.example.com",
				APIVersion:  "v1beta1",
				MappingMode: SingleCRD,
				ModuleName:  "github.com/myorg/myoperator",
			},
			wantErr:     false,
			wantVersion: "v1beta1",
			wantMode:    SingleCRD,
			wantModule:  "github.com/myorg/myoperator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
					return
				}
				valErr, ok := err.(*ValidationError)
				if !ok {
					t.Errorf("Validate() expected *ValidationError, got %T", err)
					return
				}
				if valErr.Field != tt.errField {
					t.Errorf("Validate() error field = %q, want %q", valErr.Field, tt.errField)
				}
				return
			}
			if err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
				return
			}
			if tt.config.APIVersion != tt.wantVersion {
				t.Errorf("APIVersion = %q, want %q", tt.config.APIVersion, tt.wantVersion)
			}
			if tt.config.MappingMode != tt.wantMode {
				t.Errorf("MappingMode = %q, want %q", tt.config.MappingMode, tt.wantMode)
			}
			if tt.config.ModuleName != tt.wantModule {
				t.Errorf("ModuleName = %q, want %q", tt.config.ModuleName, tt.wantModule)
			}
		})
	}
}

func TestConfig_deriveRootKindFromSpecPath(t *testing.T) {
	tests := []struct {
		specPath string
		want     string
	}{
		{"petstore.yaml", "Petstore"},
		{"petstore.json", "Petstore"},
		{"/path/to/petstore.yaml", "Petstore"},
		{"my-api.yaml", "MyApi"},
		{"my_api.yaml", "MyApi"},
		{"my.api.yaml", "MyApi"},
		{"petstore.1.0.27.yaml", "Petstore"},
		{"api-v1.yaml", "ApiV1"},              // hyphen-separated versions are not stripped
		{"api-v2.json", "ApiV2"},              // hyphen-separated versions are not stripped
		{"my-service-v1.yaml", "MyServiceV1"}, // hyphen-separated versions are not stripped
		{"API.yaml", "Api"},
		{"PETSTORE.yaml", "Petstore"},
		{"PetStore.yaml", "Petstore"},
		{"pet-store-api.yaml", "PetStoreApi"},
		{"", ""},
		// URL-based spec paths
		{"https://example.com/api/petstore.yaml", "Petstore"},
		{"https://example.com/api/petstore.json", "Petstore"},
		{"http://localhost:8080/my-api.yaml", "MyApi"},
		{"https://raw.githubusercontent.com/user/repo/main/openapi.yaml", "Openapi"},
		{"https://petstore3.swagger.io/api/v3/openapi.json", "Openapi"},
		{"https://example.com/specs/my-service-api.1.0.0.yaml", "MyServiceApi"},
		{"https://example.com/", ""}, // URL with no filename
		{"https://example.com", ""},  // URL with no path
		{"http://api.example.com/spec.yaml", "Spec"},
	}

	for _, tt := range tests {
		t.Run(tt.specPath, func(t *testing.T) {
			c := &Config{SpecPath: tt.specPath}
			got := c.deriveRootKindFromSpecPath()
			if got != tt.want {
				t.Errorf("deriveRootKindFromSpecPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsVersionLike(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"1", true},
		{"1.0", true},
		{"123", true},
		{"v1", true},
		{"v2", true},
		{"V1", true},
		{"V2", true},
		{"", false},
		{"api", false},
		{"abc", false},
		{"va", false},
		{"version", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isVersionLike(tt.input)
			if got != tt.want {
				t.Errorf("isVersionLike(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"petstore", "Petstore"},
		{"pet-store", "PetStore"},
		{"pet_store", "PetStore"},
		{"pet.store", "PetStore"},
		{"pet-store-api", "PetStoreApi"},
		{"PET", "Pet"},
		{"pet store", "PetStore"},
		{"  pet   store  ", "PetStore"},
		{"", ""},
		{"a", "A"},
		{"API", "Api"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toPascalCase(tt.input)
			if got != tt.want {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "TestField",
		Message: "test message",
	}
	want := "TestField: test message"
	if got := err.Error(); got != want {
		t.Errorf("ValidationError.Error() = %q, want %q", got, want)
	}
}

func TestMappingModeConstants(t *testing.T) {
	if PerResource != "per-resource" {
		t.Errorf("PerResource = %q, want %q", PerResource, "per-resource")
	}
	if SingleCRD != "single-crd" {
		t.Errorf("SingleCRD = %q, want %q", SingleCRD, "single-crd")
	}
}

func TestConfig_Validate_SetsRootKind(t *testing.T) {
	tests := []struct {
		name         string
		specPath     string
		explicitKind string
		wantKind     string
	}{
		{
			name:     "derives from spec path",
			specPath: "/path/to/petstore.yaml",
			wantKind: "Petstore",
		},
		{
			name:         "uses explicit RootKind",
			specPath:     "/path/to/petstore.yaml",
			explicitKind: "MyCustomKind",
			wantKind:     "MyCustomKind",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				SpecPath:  tt.specPath,
				OutputDir: "/out",
				APIGroup:  "test.example.com",
				RootKind:  tt.explicitKind,
			}
			err := c.Validate()
			if err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}
			if c.RootKind != tt.wantKind {
				t.Errorf("RootKind = %q, want %q", c.RootKind, tt.wantKind)
			}
		})
	}
}

func TestPathFilter_ShouldIncludePath(t *testing.T) {
	tests := []struct {
		name         string
		includePaths []string
		excludePaths []string
		path         string
		want         bool
	}{
		{
			name: "no filters - include all",
			path: "/users",
			want: true,
		},
		{
			name:         "exact include match",
			includePaths: []string{"/users"},
			path:         "/users",
			want:         true,
		},
		{
			name:         "exact include no match",
			includePaths: []string{"/users"},
			path:         "/pets",
			want:         false,
		},
		{
			name:         "wildcard include match",
			includePaths: []string{"/users/*"},
			path:         "/users/123",
			want:         true,
		},
		{
			name:         "wildcard include nested match",
			includePaths: []string{"/users/*"},
			path:         "/users/123/profile",
			want:         true,
		},
		{
			name:         "wildcard include base path",
			includePaths: []string{"/users/*"},
			path:         "/users",
			want:         true,
		},
		{
			name:         "exact exclude match",
			excludePaths: []string{"/internal"},
			path:         "/internal",
			want:         false,
		},
		{
			name:         "wildcard exclude match",
			excludePaths: []string{"/internal/*"},
			path:         "/internal/metrics",
			want:         false,
		},
		{
			name:         "exclude takes precedence over include",
			includePaths: []string{"/api/*"},
			excludePaths: []string{"/api/internal/*"},
			path:         "/api/internal/secret",
			want:         false,
		},
		{
			name:         "include with exclude - passes include, not excluded",
			includePaths: []string{"/api/*"},
			excludePaths: []string{"/api/internal/*"},
			path:         "/api/users",
			want:         true,
		},
		{
			name:         "multiple include patterns",
			includePaths: []string{"/users", "/pets"},
			path:         "/pets",
			want:         true,
		},
		{
			name:         "multiple exclude patterns",
			excludePaths: []string{"/internal/*", "/admin/*"},
			path:         "/admin/users",
			want:         false,
		},
		{
			name:         "single segment wildcard",
			includePaths: []string{"/users/?"},
			path:         "/users/123",
			want:         true,
		},
		{
			name:         "single segment wildcard - no nested",
			includePaths: []string{"/users/?"},
			path:         "/users/123/profile",
			want:         false,
		},
		{
			name:         "path with params matches exact pattern",
			includePaths: []string{"/pet/{petId}"},
			path:         "/pet/{petId}",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				IncludePaths: tt.includePaths,
				ExcludePaths: tt.excludePaths,
			}
			f := NewPathFilter(cfg)
			got := f.ShouldIncludePath(tt.path)
			if got != tt.want {
				t.Errorf("ShouldIncludePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestPathFilter_ShouldIncludeTags(t *testing.T) {
	tests := []struct {
		name        string
		includeTags []string
		excludeTags []string
		tags        []string
		want        bool
	}{
		{
			name: "no filters - include all",
			tags: []string{"pet", "store"},
			want: true,
		},
		{
			name:        "exact include match",
			includeTags: []string{"pet"},
			tags:        []string{"pet", "store"},
			want:        true,
		},
		{
			name:        "include no match",
			includeTags: []string{"user"},
			tags:        []string{"pet", "store"},
			want:        false,
		},
		{
			name:        "exact exclude match",
			excludeTags: []string{"deprecated"},
			tags:        []string{"pet", "deprecated"},
			want:        false,
		},
		{
			name:        "exclude takes precedence",
			includeTags: []string{"pet"},
			excludeTags: []string{"deprecated"},
			tags:        []string{"pet", "deprecated"},
			want:        false,
		},
		{
			name:        "case insensitive include",
			includeTags: []string{"PET"},
			tags:        []string{"pet"},
			want:        true,
		},
		{
			name:        "case insensitive exclude",
			excludeTags: []string{"DEPRECATED"},
			tags:        []string{"pet", "deprecated"},
			want:        false,
		},
		{
			name:        "empty tags with include filter",
			includeTags: []string{"pet"},
			tags:        []string{},
			want:        false,
		},
		{
			name:        "empty tags with exclude filter only",
			excludeTags: []string{"deprecated"},
			tags:        []string{},
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				IncludeTags: tt.includeTags,
				ExcludeTags: tt.excludeTags,
			}
			f := NewPathFilter(cfg)
			got := f.ShouldIncludeTags(tt.tags)
			if got != tt.want {
				t.Errorf("ShouldIncludeTags(%v) = %v, want %v", tt.tags, got, tt.want)
			}
		})
	}
}

func TestPathFilter_ShouldInclude(t *testing.T) {
	tests := []struct {
		name         string
		includePaths []string
		excludePaths []string
		includeTags  []string
		excludeTags  []string
		path         string
		tags         []string
		want         bool
	}{
		{
			name: "no filters",
			path: "/users",
			tags: []string{"user"},
			want: true,
		},
		{
			name:         "path included, tags included",
			includePaths: []string{"/users"},
			includeTags:  []string{"user"},
			path:         "/users",
			tags:         []string{"user"},
			want:         true,
		},
		{
			name:         "path included, tags excluded",
			includePaths: []string{"/users"},
			excludeTags:  []string{"deprecated"},
			path:         "/users",
			tags:         []string{"user", "deprecated"},
			want:         false,
		},
		{
			name:         "path excluded, tags included",
			excludePaths: []string{"/internal/*"},
			includeTags:  []string{"user"},
			path:         "/internal/users",
			tags:         []string{"user"},
			want:         false,
		},
		{
			name:         "path not matching include",
			includePaths: []string{"/users"},
			path:         "/pets",
			tags:         []string{"pet"},
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				IncludePaths: tt.includePaths,
				ExcludePaths: tt.excludePaths,
				IncludeTags:  tt.includeTags,
				ExcludeTags:  tt.excludeTags,
			}
			f := NewPathFilter(cfg)
			got := f.ShouldInclude(tt.path, tt.tags)
			if got != tt.want {
				t.Errorf("ShouldInclude(%q, %v) = %v, want %v", tt.path, tt.tags, got, tt.want)
			}
		})
	}
}

func TestPathFilter_HasFilters(t *testing.T) {
	tests := []struct {
		name         string
		includePaths []string
		excludePaths []string
		includeTags  []string
		excludeTags  []string
		want         bool
	}{
		{
			name: "no filters",
			want: false,
		},
		{
			name:         "has include paths",
			includePaths: []string{"/users"},
			want:         true,
		},
		{
			name:         "has exclude paths",
			excludePaths: []string{"/internal/*"},
			want:         true,
		},
		{
			name:        "has include tags",
			includeTags: []string{"public"},
			want:        true,
		},
		{
			name:        "has exclude tags",
			excludeTags: []string{"deprecated"},
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				IncludePaths: tt.includePaths,
				ExcludePaths: tt.excludePaths,
				IncludeTags:  tt.includeTags,
				ExcludeTags:  tt.excludeTags,
			}
			f := NewPathFilter(cfg)
			got := f.HasFilters()
			if got != tt.want {
				t.Errorf("HasFilters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchPath(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		// Exact matches
		{"/users", "/users", true},
		{"/users", "/pets", false},
		{"/users/", "/users", true}, // trailing slash normalized

		// Wildcard suffix /*
		{"/users/*", "/users", true},
		{"/users/*", "/users/123", true},
		{"/users/*", "/users/123/profile", true},
		{"/users/*", "/pets", false},

		// Single segment /?
		{"/users/?", "/users/123", true},
		{"/users/?", "/users/123/profile", false},
		{"/users/?", "/users", false},

		// Glob patterns
		{"/api/*/users", "/api/v1/users", true},
		{"/api/*/users", "/api/v2/users", true},
		{"/api/*/users", "/api/users", false},
		{"/*", "/anything", true},
		{"/*", "/", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			got := matchPath(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("matchPath(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestPathFilter_ShouldIncludeOperation(t *testing.T) {
	tests := []struct {
		name              string
		includeOperations []string
		excludeOperations []string
		operationID       string
		want              bool
	}{
		{
			name:        "no filters - include all",
			operationID: "getPetById",
			want:        true,
		},
		{
			name:              "exact include match",
			includeOperations: []string{"getPetById"},
			operationID:       "getPetById",
			want:              true,
		},
		{
			name:              "exact include no match",
			includeOperations: []string{"getPetById"},
			operationID:       "createPet",
			want:              false,
		},
		{
			name:              "prefix pattern match",
			includeOperations: []string{"get*"},
			operationID:       "getPetById",
			want:              true,
		},
		{
			name:              "prefix pattern no match",
			includeOperations: []string{"get*"},
			operationID:       "createPet",
			want:              false,
		},
		{
			name:              "suffix pattern match",
			includeOperations: []string{"*Pet"},
			operationID:       "createPet",
			want:              true,
		},
		{
			name:              "suffix pattern no match",
			includeOperations: []string{"*Pet"},
			operationID:       "getPetById",
			want:              false,
		},
		{
			name:              "contains pattern match",
			includeOperations: []string{"*Pet*"},
			operationID:       "getPetById",
			want:              true,
		},
		{
			name:              "contains pattern match 2",
			includeOperations: []string{"*Pet*"},
			operationID:       "updatePetStatus",
			want:              true,
		},
		{
			name:              "exact exclude match",
			excludeOperations: []string{"deletePet"},
			operationID:       "deletePet",
			want:              false,
		},
		{
			name:              "exclude takes precedence",
			includeOperations: []string{"*Pet*"},
			excludeOperations: []string{"deletePet"},
			operationID:       "deletePet",
			want:              false,
		},
		{
			name:              "complex glob pattern",
			includeOperations: []string{"get*ById"},
			operationID:       "getPetById",
			want:              true,
		},
		{
			name:              "complex glob pattern 2",
			includeOperations: []string{"get*ById"},
			operationID:       "getUserById",
			want:              true,
		},
		{
			name:              "complex glob pattern no match",
			includeOperations: []string{"get*ById"},
			operationID:       "getPets",
			want:              false,
		},
		{
			name:              "multiple include patterns",
			includeOperations: []string{"getPetById", "createPet"},
			operationID:       "createPet",
			want:              true,
		},
		{
			name:              "empty operationId with include filter",
			includeOperations: []string{"getPet*"},
			operationID:       "",
			want:              false,
		},
		{
			name:              "empty operationId with exclude filter only",
			excludeOperations: []string{"deletePet"},
			operationID:       "",
			want:              true, // empty operationId doesn't match exclude pattern, so it passes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				IncludeOperations: tt.includeOperations,
				ExcludeOperations: tt.excludeOperations,
			}
			f := NewPathFilter(cfg)
			got := f.ShouldIncludeOperation(tt.operationID)
			if got != tt.want {
				t.Errorf("ShouldIncludeOperation(%q) = %v, want %v", tt.operationID, got, tt.want)
			}
		})
	}
}

func TestPathFilter_ShouldIncludeWithOperations(t *testing.T) {
	tests := []struct {
		name              string
		includePaths      []string
		excludePaths      []string
		includeTags       []string
		excludeTags       []string
		includeOperations []string
		excludeOperations []string
		path              string
		tags              []string
		operationIDs      []string
		want              bool
	}{
		{
			name:         "no filters",
			path:         "/pets",
			tags:         []string{"pet"},
			operationIDs: []string{"getPetById"},
			want:         true,
		},
		{
			name:              "path and operation included",
			includePaths:      []string{"/pets/*"},
			includeOperations: []string{"get*"},
			path:              "/pets/{petId}",
			tags:              []string{"pet"},
			operationIDs:      []string{"getPetById", "updatePet", "deletePet"},
			want:              true,
		},
		{
			name:              "path included but operation excluded",
			includePaths:      []string{"/pets/*"},
			excludeOperations: []string{"deletePet"},
			path:              "/pets/{petId}",
			tags:              []string{"pet"},
			operationIDs:      []string{"deletePet"}, // only has deletePet
			want:              false,
		},
		{
			name:              "path included, some operations excluded but one passes",
			includePaths:      []string{"/pets/*"},
			excludeOperations: []string{"deletePet"},
			path:              "/pets/{petId}",
			tags:              []string{"pet"},
			operationIDs:      []string{"getPetById", "deletePet"}, // getPetById passes
			want:              true,
		},
		{
			name:              "operation filter with no operationIds",
			includeOperations: []string{"get*"},
			path:              "/pets",
			tags:              []string{"pet"},
			operationIDs:      []string{},
			want:              false, // include filter requires match
		},
		{
			name:              "exclude operation filter with no operationIds",
			excludeOperations: []string{"deletePet"},
			path:              "/pets",
			tags:              []string{"pet"},
			operationIDs:      []string{},
			want:              true, // no operationIds, nothing to exclude
		},
		{
			name:         "path excluded overrides everything",
			excludePaths: []string{"/internal/*"},
			path:         "/internal/pets",
			tags:         []string{"pet"},
			operationIDs: []string{"getPetById"},
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				IncludePaths:      tt.includePaths,
				ExcludePaths:      tt.excludePaths,
				IncludeTags:       tt.includeTags,
				ExcludeTags:       tt.excludeTags,
				IncludeOperations: tt.includeOperations,
				ExcludeOperations: tt.excludeOperations,
			}
			f := NewPathFilter(cfg)
			got := f.ShouldIncludeWithOperations(tt.path, tt.tags, tt.operationIDs)
			if got != tt.want {
				t.Errorf("ShouldIncludeWithOperations(%q, %v, %v) = %v, want %v",
					tt.path, tt.tags, tt.operationIDs, got, tt.want)
			}
		})
	}
}

func TestMatchOperationID(t *testing.T) {
	tests := []struct {
		pattern     string
		operationID string
		want        bool
	}{
		// Exact matches
		{"getPetById", "getPetById", true},
		{"getPetById", "createPet", false},

		// Prefix patterns
		{"get*", "getPetById", true},
		{"get*", "getUserById", true},
		{"get*", "createPet", false},

		// Suffix patterns
		{"*Pet", "createPet", true},
		{"*Pet", "updatePet", true},
		{"*Pet", "getPetById", false},

		// Contains patterns
		{"*Pet*", "getPetById", true},
		{"*Pet*", "updatePetStatus", true},
		{"*Pet*", "getUser", false},

		// Complex glob patterns
		{"get*ById", "getPetById", true},
		{"get*ById", "getUserById", true},
		{"get*ById", "getPets", false},
		// Note: *Pet*Status - the first * can match "update", Pet matches, second * matches empty, Status matches
		// But our implementation handles this as contains pattern, so Pet*Status should be tested differently
		{"*PetStatus", "updatePetStatus", true}, // suffix pattern
		{"update*Status", "updatePetStatus", true},
		{"*Pet*", "getPetById", true}, // contains pattern

		// Empty cases
		{"", "", true},
		{"", "getPet", false},
		{"getPet", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.operationID, func(t *testing.T) {
			got := matchOperationID(tt.pattern, tt.operationID)
			if got != tt.want {
				t.Errorf("matchOperationID(%q, %q) = %v, want %v", tt.pattern, tt.operationID, got, tt.want)
			}
		})
	}
}

func TestConfig_ShouldUpdateWithPost(t *testing.T) {
	tests := []struct {
		name           string
		updateWithPost []string
		resourcePath   string
		want           bool
	}{
		{
			name:         "empty config - disabled",
			resourcePath: "/store/order",
			want:         false,
		},
		{
			name:           "wildcard enables all",
			updateWithPost: []string{"*"},
			resourcePath:   "/store/order",
			want:           true,
		},
		{
			name:           "exact path match",
			updateWithPost: []string{"/store/order"},
			resourcePath:   "/store/order",
			want:           true,
		},
		{
			name:           "exact path no match",
			updateWithPost: []string{"/store/order"},
			resourcePath:   "/users",
			want:           false,
		},
		{
			name:           "wildcard path match",
			updateWithPost: []string{"/store/*"},
			resourcePath:   "/store/order",
			want:           true,
		},
		{
			name:           "multiple patterns - first matches",
			updateWithPost: []string{"/store/order", "/users"},
			resourcePath:   "/store/order",
			want:           true,
		},
		{
			name:           "multiple patterns - second matches",
			updateWithPost: []string{"/store/order", "/users"},
			resourcePath:   "/users",
			want:           true,
		},
		{
			name:           "multiple patterns - none match",
			updateWithPost: []string{"/store/order", "/users"},
			resourcePath:   "/pets",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				UpdateWithPost: tt.updateWithPost,
			}
			got := cfg.ShouldUpdateWithPost(tt.resourcePath)
			if got != tt.want {
				t.Errorf("ShouldUpdateWithPost(%q) = %v, want %v", tt.resourcePath, got, tt.want)
			}
		})
	}
}

func TestConfig_GetIDFieldMapping(t *testing.T) {
	tests := []struct {
		name             string
		noIDMerge        bool
		idFieldMap       map[string]string
		pathParam        string
		kindName         string
		extensionMapping string
		want             string
	}{
		{
			name:      "auto-detect orderId -> id for Order kind",
			pathParam: "orderId",
			kindName:  "Order",
			want:      "id",
		},
		{
			name:      "auto-detect petId -> id for Pet kind",
			pathParam: "petId",
			kindName:  "Pet",
			want:      "id",
		},
		{
			name:      "auto-detect userId -> id for User kind",
			pathParam: "userId",
			kindName:  "User",
			want:      "id",
		},
		{
			name:      "no auto-detect for non-matching param",
			pathParam: "categoryId",
			kindName:  "Order",
			want:      "",
		},
		{
			name:      "no auto-detect for generic id param",
			pathParam: "id",
			kindName:  "Order",
			want:      "",
		},
		{
			name:      "no ID merge disabled",
			noIDMerge: true,
			pathParam: "orderId",
			kindName:  "Order",
			want:      "",
		},
		{
			name:       "explicit IDFieldMap takes precedence",
			idFieldMap: map[string]string{"orderId": "orderNumber"},
			pathParam:  "orderId",
			kindName:   "Order",
			want:       "orderNumber",
		},
		{
			name:             "extension takes precedence over auto-detect",
			pathParam:        "orderId",
			kindName:         "Order",
			extensionMapping: "externalId",
			want:             "externalId",
		},
		{
			name:             "IDFieldMap takes precedence over extension",
			idFieldMap:       map[string]string{"orderId": "customId"},
			pathParam:        "orderId",
			kindName:         "Order",
			extensionMapping: "externalId",
			want:             "customId",
		},
		{
			name:      "case insensitive kind matching",
			pathParam: "ORDERID",
			kindName:  "Order",
			want:      "id",
		},
		{
			name:      "case insensitive kind matching 2",
			pathParam: "orderID",
			kindName:  "ORDER",
			want:      "id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				NoIDMerge:  tt.noIDMerge,
				IDFieldMap: tt.idFieldMap,
			}
			got := cfg.GetIDFieldMapping(tt.pathParam, tt.kindName, tt.extensionMapping)
			if got != tt.want {
				t.Errorf("GetIDFieldMapping(%q, %q, %q) = %q, want %q",
					tt.pathParam, tt.kindName, tt.extensionMapping, got, tt.want)
			}
		})
	}
}
