package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bluecontainer/openapi-operator-gen/internal/config"
	"github.com/bluecontainer/openapi-operator-gen/pkg/generator"
	"github.com/bluecontainer/openapi-operator-gen/pkg/mapper"
	"github.com/bluecontainer/openapi-operator-gen/pkg/parser"
)

var (
	version = "dev"
	cfg     = &config.Config{}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "openapi-operator-gen",
	Short: "Generate Kubernetes operator code from OpenAPI specifications",
	Long: `openapi-operator-gen is a CLI tool that generates Kubernetes Custom Resource
Definitions (CRDs) and operator reconciliation logic from OpenAPI specifications.

It takes a REST API specification and generates:
  - Go type definitions for CRDs
  - CRD YAML manifests
  - Controller reconciliation logic with full CRUD sync

Example:
  openapi-operator-gen generate --spec api.yaml --output ./generated \
    --group myapp.example.com --version v1alpha1`,
	Version: version,
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate operator code from OpenAPI spec",
	Long: `Generate Kubernetes operator code from an OpenAPI specification.

This command parses the OpenAPI spec and generates:
  - api/<version>/types.go - CRD Go types
  - api/<version>/groupversion_info.go - API group info
  - config/crd/bases/*.yaml - CRD manifests
  - internal/controller/*_controller.go - Reconcilers
  - cmd/manager/main.go - Operator entrypoint
  - Dockerfile - Container image build
  - Makefile - Build automation`,
	RunE: runGenerate,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	// Generate command flags
	generateCmd.Flags().StringVarP(&cfg.SpecPath, "spec", "s", "", "Path to OpenAPI specification file (required)")
	generateCmd.Flags().StringVarP(&cfg.OutputDir, "output", "o", "./generated", "Output directory for generated code")
	generateCmd.Flags().StringVarP(&cfg.APIGroup, "group", "g", "", "Kubernetes API group (e.g., myapp.example.com) (required)")
	generateCmd.Flags().StringVarP(&cfg.APIVersion, "version", "v", "v1alpha1", "Kubernetes API version")
	generateCmd.Flags().StringVarP((*string)(&cfg.MappingMode), "mapping", "m", "per-resource", "Resource mapping mode: per-resource or single-crd")
	generateCmd.Flags().StringVar(&cfg.ModuleName, "module", "github.com/bluecontainer/generated-operator", "Go module name for generated code")
	generateCmd.Flags().BoolVar(&cfg.GenerateCRDs, "generate-crds", false, "Generate CRD YAML manifests directly (default: use controller-gen)")

	generateCmd.MarkFlagRequired("spec")
	generateCmd.MarkFlagRequired("group")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	fmt.Printf("Generating operator code from OpenAPI spec: %s\n", cfg.SpecPath)
	fmt.Printf("Output directory: %s\n", cfg.OutputDir)
	fmt.Printf("API Group: %s\n", cfg.APIGroup)
	fmt.Printf("API Version: %s\n", cfg.APIVersion)
	fmt.Printf("Mapping mode: %s\n", cfg.MappingMode)
	fmt.Println()

	// Parse OpenAPI spec
	fmt.Println("Parsing OpenAPI specification...")
	p := parser.NewParser()
	spec, err := p.Parse(cfg.SpecPath)
	if err != nil {
		return fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}
	fmt.Printf("  Found %d resources\n", len(spec.Resources))
	for _, r := range spec.Resources {
		fmt.Printf("    - %s (%s)\n", r.Name, r.Path)
	}
	fmt.Println()

	// Map resources to CRDs
	fmt.Println("Mapping resources to CRD definitions...")
	m := mapper.NewMapper(cfg)
	crds, err := m.MapResources(spec)
	if err != nil {
		return fmt.Errorf("failed to map resources: %w", err)
	}
	fmt.Printf("  Generated %d CRD definitions\n", len(crds))
	for _, crd := range crds {
		fmt.Printf("    - %s (%s)\n", crd.Kind, crd.Plural)
	}
	fmt.Println()

	// Generate types
	fmt.Println("Generating Go type definitions...")
	typesGen := generator.NewTypesGenerator(cfg)
	if err := typesGen.Generate(crds); err != nil {
		return fmt.Errorf("failed to generate types: %w", err)
	}
	fmt.Println("  Generated api/<version>/types.go")
	fmt.Println("  Generated api/<version>/groupversion_info.go")
	fmt.Println()

	// Generate CRD YAML (optional - controller-gen is recommended)
	if cfg.GenerateCRDs {
		fmt.Println("Generating CRD YAML manifests...")
		crdGen := generator.NewCRDGenerator(cfg)
		if err := crdGen.Generate(crds); err != nil {
			return fmt.Errorf("failed to generate CRD YAML: %w", err)
		}
		fmt.Println("  Generated config/crd/bases/*.yaml")
		fmt.Println()
	} else {
		fmt.Println("Skipping CRD YAML generation (use 'make generate' to generate with controller-gen)")
		fmt.Println()
	}

	// Generate controllers
	fmt.Println("Generating controller reconciliation logic...")
	controllerGen := generator.NewControllerGenerator(cfg)
	if err := controllerGen.Generate(crds); err != nil {
		return fmt.Errorf("failed to generate controllers: %w", err)
	}
	fmt.Println("  Generated internal/controller/*_controller.go")
	fmt.Println("  Generated cmd/manager/main.go")
	fmt.Println("  Generated go.mod")
	fmt.Println("  Generated Dockerfile")
	fmt.Println("  Generated Makefile")
	fmt.Println()

	fmt.Println("Code generation complete!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. cd %s\n", cfg.OutputDir)
	fmt.Println("  2. go mod tidy")
	fmt.Println("  3. make generate  # Generate deep copy methods")
	fmt.Println("  4. make build     # Build the operator")
	fmt.Println("  5. make install   # Install CRDs to cluster")
	fmt.Println("  6. make run       # Run the operator locally")

	return nil
}
