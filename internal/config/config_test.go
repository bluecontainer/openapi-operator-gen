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
