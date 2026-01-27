package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bluecontainer/openapi-operator-gen/internal/config"
	"github.com/bluecontainer/openapi-operator-gen/pkg/generator"
	"github.com/bluecontainer/openapi-operator-gen/pkg/mapper"
	"github.com/bluecontainer/openapi-operator-gen/pkg/parser"
)

var (
	// version is set at build time via -ldflags
	version = "dev"
	// commit is the git commit hash, set at build time via -ldflags
	commit = "none"
	// date is the build date, set at build time via -ldflags
	date = "unknown"

	cfg = &config.Config{}

	// Config file path flag
	configFile string

	// Filter flags (comma-separated strings, parsed into slices)
	includePaths      string
	excludePaths      string
	includeTags       string
	excludeTags       string
	includeOperations string
	excludeOperations string
	updateWithPost    string
	idFieldMap        string
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

Examples:
  # Generate from local file
  openapi-operator-gen generate --spec api.yaml --output ./generated \
    --group myapp.example.com --version v1alpha1

  # Generate from URL
  openapi-operator-gen generate --spec https://example.com/api/openapi.yaml \
    --output ./generated --group myapp.example.com`,
}

func init() {
	rootCmd.Version = version
	rootCmd.SetVersionTemplate(fmt.Sprintf("openapi-operator-gen version %s\n  commit: %s\n  built:  %s\n", version, commit, date))
}

var initConfigFile string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create an example configuration file",
	Long: `Create an example openapi-operator-gen configuration file.

This command creates a YAML configuration file with all available options
documented with comments. You can then edit this file and use it with:

  openapi-operator-gen generate --config myconfig.yaml

Or place it in the current directory as .openapi-operator-gen.yaml for
automatic discovery.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVarP(&initConfigFile, "output", "o", ".openapi-operator-gen.yaml", "Output path for config file")
}

func runInit(cmd *cobra.Command, args []string) error {
	if err := config.WriteExampleConfig(initConfigFile); err != nil {
		return err
	}
	fmt.Printf("Created example config file: %s\n", initConfigFile)
	fmt.Println()
	fmt.Println("Edit the file to configure your generator options, then run:")
	fmt.Println("  openapi-operator-gen generate")
	fmt.Println()
	fmt.Println("Or specify the config file explicitly:")
	fmt.Printf("  openapi-operator-gen generate --config %s\n", initConfigFile)
	return nil
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

	// Config file flag
	generateCmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to config file (default: searches for .openapi-operator-gen.yaml)")

	// Generate command flags
	generateCmd.Flags().StringVarP(&cfg.SpecPath, "spec", "s", "", "Path or URL to OpenAPI specification file")
	generateCmd.Flags().StringVarP(&cfg.OutputDir, "output", "o", "./generated", "Output directory for generated code")
	generateCmd.Flags().StringVarP(&cfg.APIGroup, "group", "g", "", "Kubernetes API group (e.g., myapp.example.com)")
	generateCmd.Flags().StringVarP(&cfg.APIVersion, "version", "v", "v1alpha1", "Kubernetes API version")
	generateCmd.Flags().StringVarP((*string)(&cfg.MappingMode), "mapping", "m", "per-resource", "Resource mapping mode: per-resource or single-crd")
	generateCmd.Flags().StringVar(&cfg.ModuleName, "module", "github.com/bluecontainer/generated-operator", "Go module name for generated code")
	generateCmd.Flags().BoolVar(&cfg.GenerateCRDs, "generate-crds", false, "Generate CRD YAML manifests directly (default: use controller-gen)")
	generateCmd.Flags().StringVar(&cfg.RootKind, "root-kind", "", "Kind name for root '/' endpoint (default: derived from spec filename)")
	generateCmd.Flags().BoolVar(&cfg.GenerateAggregate, "aggregate", false, "Generate a Status Aggregator CRD for observing multiple resource types")
	generateCmd.Flags().BoolVar(&cfg.GenerateBundle, "bundle", false, "Generate an Inline Composition Bundle CRD for creating multiple resources")
	generateCmd.Flags().StringVar(&updateWithPost, "update-with-post", "", "Use POST for updates when PUT is not available. Value: '*' for all, or comma-separated paths (e.g., /store/order,/users/*)")

	// Resource filtering flags
	generateCmd.Flags().StringVar(&includePaths, "include-paths", "", "Only include paths matching these patterns (comma-separated, glob supported: /users,/pets/*)")
	generateCmd.Flags().StringVar(&excludePaths, "exclude-paths", "", "Exclude paths matching these patterns (comma-separated, glob supported: /internal/*,/admin/*)")
	generateCmd.Flags().StringVar(&includeTags, "include-tags", "", "Only include endpoints with these OpenAPI tags (comma-separated: public,v2)")
	generateCmd.Flags().StringVar(&excludeTags, "exclude-tags", "", "Exclude endpoints with these OpenAPI tags (comma-separated: deprecated,internal)")
	generateCmd.Flags().StringVar(&includeOperations, "include-operations", "", "Only include operations with these operationIds (comma-separated, glob supported: getPet*,createPet)")
	generateCmd.Flags().StringVar(&excludeOperations, "exclude-operations", "", "Exclude operations with these operationIds (comma-separated, glob supported: *Deprecated,deletePet)")

	// ID field merging flags
	generateCmd.Flags().BoolVar(&cfg.NoIDMerge, "no-id-merge", false, "Disable automatic merging of path ID parameters with body 'id' fields")
	generateCmd.Flags().StringVar(&idFieldMap, "id-field-map", "", "Explicit path param to body field mappings (comma-separated: orderId=id,petId=id)")

	// Note: spec and group are no longer marked as required since they can come from config file
}

// parseCommaSeparated splits a comma-separated string into a slice, trimming whitespace
func parseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseIDFieldMap parses a comma-separated list of "key=value" pairs into a map.
// Example: "orderId=id,petId=id" -> {"orderId": "id", "petId": "id"}
func parseIDFieldMap(s string) map[string]string {
	if s == "" {
		return nil
	}
	result := make(map[string]string)
	parts := strings.Split(s, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			if key != "" && value != "" {
				result[key] = value
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func runGenerate(cmd *cobra.Command, args []string) error {
	// Load config file if specified or found
	var cfgFilePath string
	if configFile != "" {
		cfgFilePath = configFile
	} else {
		cfgFilePath = config.FindConfigFile()
	}

	if cfgFilePath != "" {
		fileCfg, err := config.LoadConfigFile(cfgFilePath)
		if err != nil {
			return fmt.Errorf("failed to load config file %s: %w", cfgFilePath, err)
		}
		if fileCfg != nil {
			fmt.Printf("Using config file: %s\n", cfgFilePath)
			config.MergeConfigFile(cfg, fileCfg)
		}
	}

	// Set the generator version and commit info for embedding in generated go.mod
	cfg.GeneratorVersion = version
	cfg.CommitHash = commit
	cfg.CommitTimestamp = date

	// Parse filter flags into config slices (CLI flags override config file)
	if includePaths != "" {
		cfg.IncludePaths = parseCommaSeparated(includePaths)
	}
	if excludePaths != "" {
		cfg.ExcludePaths = parseCommaSeparated(excludePaths)
	}
	if includeTags != "" {
		cfg.IncludeTags = parseCommaSeparated(includeTags)
	}
	if excludeTags != "" {
		cfg.ExcludeTags = parseCommaSeparated(excludeTags)
	}
	if includeOperations != "" {
		cfg.IncludeOperations = parseCommaSeparated(includeOperations)
	}
	if excludeOperations != "" {
		cfg.ExcludeOperations = parseCommaSeparated(excludeOperations)
	}
	if updateWithPost != "" {
		cfg.UpdateWithPost = parseCommaSeparated(updateWithPost)
	}
	if idFieldMap != "" {
		cfg.IDFieldMap = parseIDFieldMap(idFieldMap)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	fmt.Printf("Generating operator code from OpenAPI spec: %s\n", cfg.SpecPath)
	fmt.Printf("Output directory: %s\n", cfg.OutputDir)
	fmt.Printf("API Group: %s\n", cfg.APIGroup)
	fmt.Printf("API Version: %s\n", cfg.APIVersion)
	fmt.Printf("Mapping mode: %s\n", cfg.MappingMode)
	if len(cfg.IncludePaths) > 0 {
		fmt.Printf("Include paths: %s\n", strings.Join(cfg.IncludePaths, ", "))
	}
	if len(cfg.ExcludePaths) > 0 {
		fmt.Printf("Exclude paths: %s\n", strings.Join(cfg.ExcludePaths, ", "))
	}
	if len(cfg.IncludeTags) > 0 {
		fmt.Printf("Include tags: %s\n", strings.Join(cfg.IncludeTags, ", "))
	}
	if len(cfg.ExcludeTags) > 0 {
		fmt.Printf("Exclude tags: %s\n", strings.Join(cfg.ExcludeTags, ", "))
	}
	if len(cfg.IncludeOperations) > 0 {
		fmt.Printf("Include operations: %s\n", strings.Join(cfg.IncludeOperations, ", "))
	}
	if len(cfg.ExcludeOperations) > 0 {
		fmt.Printf("Exclude operations: %s\n", strings.Join(cfg.ExcludeOperations, ", "))
	}
	if len(cfg.UpdateWithPost) > 0 {
		fmt.Printf("Update with POST: %s\n", strings.Join(cfg.UpdateWithPost, ", "))
	}
	if cfg.NoIDMerge {
		fmt.Println("ID field merging: disabled")
	} else if len(cfg.IDFieldMap) > 0 {
		mappings := make([]string, 0, len(cfg.IDFieldMap))
		for k, v := range cfg.IDFieldMap {
			mappings = append(mappings, k+"="+v)
		}
		fmt.Printf("ID field map: %s\n", strings.Join(mappings, ", "))
	}
	fmt.Println()

	// Parse OpenAPI spec
	fmt.Println("Parsing OpenAPI specification...")
	filter := config.NewPathFilter(cfg)
	p := parser.NewParserWithFilter(cfg.RootKind, filter)
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

	// Generate Status Aggregator CRD (optional) - do this before samples so we can include aggregate sample
	var aggregate *mapper.AggregateDefinition
	if cfg.GenerateAggregate {
		fmt.Println("Generating Status Aggregator CRD...")
		aggregate = m.CreateAggregateDefinition(crds)
		if err := typesGen.GenerateAggregateTypes(aggregate); err != nil {
			return fmt.Errorf("failed to generate aggregate types: %w", err)
		}
		fmt.Println("  Generated api/<version>/aggregate_types.go")
		fmt.Println()
	}

	// Generate Bundle CRD (optional) - do this before samples so we can include bundle sample
	var bundle *mapper.BundleDefinition
	if cfg.GenerateBundle {
		fmt.Println("Generating Inline Composition Bundle CRD...")
		bundle = m.CreateBundleDefinition(crds)
		if err := typesGen.GenerateBundleTypes(bundle); err != nil {
			return fmt.Errorf("failed to generate bundle types: %w", err)
		}
		fmt.Println("  Generated api/<version>/bundle_types.go")
		fmt.Println()
	}

	// Generate example CR samples (always generated, includes aggregate/bundle samples if enabled)
	fmt.Println("Generating example CR samples...")
	samplesGen := generator.NewSamplesGenerator(cfg)
	if err := samplesGen.Generate(crds, aggregate, bundle); err != nil {
		return fmt.Errorf("failed to generate example CRs: %w", err)
	}
	fmt.Println("  Generated config/samples/*.yaml")
	fmt.Println()

	// Generate controllers (pass aggregate and bundle to include in main.go registration)
	fmt.Println("Generating controller reconciliation logic...")
	controllerGen := generator.NewControllerGenerator(cfg)
	if err := controllerGen.Generate(crds, aggregate, bundle); err != nil {
		return fmt.Errorf("failed to generate controllers: %w", err)
	}
	fmt.Println("  Generated internal/controller/*_controller.go")
	fmt.Println("  Generated cmd/manager/main.go")
	fmt.Println("  Generated go.mod")
	fmt.Println("  Generated Dockerfile")
	fmt.Println("  Generated Makefile")
	fmt.Println("  Copied OpenAPI spec file")
	fmt.Println()

	// Generate aggregate controller if enabled
	if aggregate != nil {
		if err := controllerGen.GenerateAggregateController(aggregate); err != nil {
			return fmt.Errorf("failed to generate aggregate controller: %w", err)
		}
		fmt.Println("  Generated internal/controller/statusaggregate_controller.go")
		fmt.Println()
	}

	// Generate bundle controller if enabled
	if bundle != nil {
		if err := controllerGen.GenerateBundleController(bundle); err != nil {
			return fmt.Errorf("failed to generate bundle controller: %w", err)
		}
		fmt.Printf("  Generated internal/controller/%s_controller.go\n", strings.ToLower(bundle.Kind))
		fmt.Println()
	}

	// Generate CEL test file and test data if aggregate or bundle is enabled (they use CEL expressions)
	if aggregate != nil || bundle != nil {
		// Collect kinds for CEL templates
		var resourceKinds, queryKinds, actionKinds, allKinds []string
		var aggregateKind, bundleKind string

		if aggregate != nil {
			resourceKinds = aggregate.ResourceKinds
			queryKinds = aggregate.QueryKinds
			actionKinds = aggregate.ActionKinds
			allKinds = aggregate.AllKinds
			aggregateKind = aggregate.Kind
		}
		if bundle != nil {
			if len(resourceKinds) == 0 {
				resourceKinds = bundle.ResourceKinds
				queryKinds = bundle.QueryKinds
				actionKinds = bundle.ActionKinds
				allKinds = bundle.AllKinds
			}
			bundleKind = bundle.Kind
		}

		if err := controllerGen.GenerateCELTest(allKinds); err != nil {
			return fmt.Errorf("failed to generate CEL tests: %w", err)
		}
		fmt.Println("  Generated internal/controller/cel_test.go")

		if err := controllerGen.GenerateCELTestData(resourceKinds, queryKinds, actionKinds, allKinds, aggregateKind, bundleKind, crds); err != nil {
			return fmt.Errorf("failed to generate CEL test data: %w", err)
		}
		fmt.Println("  Generated testdata/cel-test-data.json")
		fmt.Println("  Generated testdata/README.md")
		if aggregateKind != "" || bundleKind != "" {
			fmt.Println("  Generated testdata/resources.yaml")
		}
		fmt.Println()
	}

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
