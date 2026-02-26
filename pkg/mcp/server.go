package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/bluecontainer/openapi-operator-gen/internal/config"
	"github.com/bluecontainer/openapi-operator-gen/pkg/generator"
	"github.com/bluecontainer/openapi-operator-gen/pkg/mapper"
	"github.com/bluecontainer/openapi-operator-gen/pkg/parser"
)

// NewServer creates an MCP server with validate, preview, and generate tools.
func NewServer(version, commit, date string) *server.MCPServer {
	s := server.NewMCPServer(
		"openapi-operator-gen",
		version,
	)

	h := &handlers{
		version: version,
		commit:  commit,
		date:    date,
	}

	s.AddTool(validateTool, h.handleValidate)
	s.AddTool(previewTool, h.handlePreview)
	s.AddTool(generateTool, h.handleGenerate)
	s.AddTool(describeTool, h.handleDescribe)
	s.AddTool(regenerateTool, h.handleRegenerate)
	s.AddTool(diffTool, h.handleDiff)

	s.AddPrompt(generateOperatorPrompt, h.handleGenerateOperatorPrompt)
	s.AddPrompt(previewAPIPrompt, h.handlePreviewAPIPrompt)
	s.AddPrompt(evolveSpecPrompt, h.handleEvolveSpecPrompt)

	return s
}

// Tool definitions

var validateTool = mcp.NewTool("validate",
	mcp.WithDescription("Validate an OpenAPI specification and show a summary of its contents. Use this to check if a spec is parseable before generating an operator."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("spec",
		mcp.Required(),
		mcp.Description("Path or URL to the OpenAPI specification file"),
	),
)

var previewTool = mcp.NewTool("preview",
	mcp.WithDescription("Parse an OpenAPI spec and show what Kubernetes CRDs would be generated, without writing any files. Shows resource classification (CRUD Resources, GET-only Queries, POST/PUT-only Actions) and the Kind names, paths, and fields for each CRD."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("spec",
		mcp.Required(),
		mcp.Description("Path or URL to the OpenAPI specification file"),
	),
	mcp.WithString("group",
		mcp.Description("Kubernetes API group (e.g., myapp.example.com). Used for Kind name derivation."),
	),
	mcp.WithString("include_paths",
		mcp.Description("Only include paths matching these patterns (comma-separated, glob supported: /users,/pets/*)"),
	),
	mcp.WithString("exclude_paths",
		mcp.Description("Exclude paths matching these patterns (comma-separated, glob supported: /internal/*,/admin/*)"),
	),
	mcp.WithString("include_tags",
		mcp.Description("Only include endpoints with these OpenAPI tags (comma-separated: public,v2)"),
	),
	mcp.WithString("exclude_tags",
		mcp.Description("Exclude endpoints with these OpenAPI tags (comma-separated: deprecated,internal)"),
	),
	mcp.WithString("include_operations",
		mcp.Description("Only include operations with these operationIds (comma-separated, glob supported: getPet*,createPet)"),
	),
	mcp.WithString("exclude_operations",
		mcp.Description("Exclude operations with these operationIds (comma-separated, glob supported: *Deprecated,deletePet)"),
	),
)

var generateTool = mcp.NewTool("generate",
	mcp.WithDescription("Generate a complete Kubernetes operator from an OpenAPI specification. Creates Go types, CRD manifests, controllers, Dockerfile, Makefile, and optionally kubectl plugins, Rundeck projects, aggregate/bundle CRDs. After generation, run: cd <output> && go mod tidy && make generate && make build && make test"),
	// Required parameters
	mcp.WithString("spec",
		mcp.Required(),
		mcp.Description("Path or URL to the OpenAPI specification file"),
	),
	mcp.WithString("output",
		mcp.Required(),
		mcp.Description("Output directory for generated operator code"),
	),
	mcp.WithString("group",
		mcp.Required(),
		mcp.Description("Kubernetes API group (e.g., myapp.example.com)"),
	),
	mcp.WithString("module",
		mcp.Required(),
		mcp.Description("Go module name for the generated operator (e.g., github.com/myorg/myapp-operator)"),
	),
	// Optional parameters
	mcp.WithString("version",
		mcp.Description("Kubernetes API version (default: v1alpha1)"),
	),
	mcp.WithString("mapping",
		mcp.Description("Resource mapping mode: 'per-resource' (one CRD per REST resource, default) or 'single-crd' (one CRD for entire API)"),
	),
	mcp.WithBoolean("aggregate",
		mcp.Description("Generate a Status Aggregator CRD for observing multiple resource types"),
	),
	mcp.WithBoolean("bundle",
		mcp.Description("Generate an Inline Composition Bundle CRD for creating multiple resources as a unit"),
	),
	mcp.WithBoolean("kubectl_plugin",
		mcp.Description("Generate a kubectl plugin for managing and diagnosing operator resources"),
	),
	mcp.WithBoolean("rundeck_project",
		mcp.Description("Generate Rundeck projects with jobs for operating the API (requires kubectl_plugin=true)"),
	),
	mcp.WithBoolean("standalone_node_source",
		mcp.Description("Use standalone kubectl-rundeck-nodes plugin instead of generating a per-API node source plugin"),
	),
	mcp.WithBoolean("generate_crds",
		mcp.Description("Generate CRD YAML manifests directly (default: use controller-gen via 'make generate')"),
	),
	mcp.WithString("root_kind",
		mcp.Description("Kind name for root '/' endpoint (default: derived from spec filename)"),
	),
	mcp.WithString("include_paths",
		mcp.Description("Only include paths matching these patterns (comma-separated, glob supported)"),
	),
	mcp.WithString("exclude_paths",
		mcp.Description("Exclude paths matching these patterns (comma-separated, glob supported)"),
	),
	mcp.WithString("include_tags",
		mcp.Description("Only include endpoints with these OpenAPI tags (comma-separated)"),
	),
	mcp.WithString("exclude_tags",
		mcp.Description("Exclude endpoints with these OpenAPI tags (comma-separated)"),
	),
	mcp.WithString("include_operations",
		mcp.Description("Only include operations with these operationIds (comma-separated, glob supported)"),
	),
	mcp.WithString("exclude_operations",
		mcp.Description("Exclude operations with these operationIds (comma-separated, glob supported)"),
	),
	mcp.WithString("update_with_post",
		mcp.Description("Use POST for updates when PUT is not available. Value: '*' for all, or comma-separated paths"),
	),
	mcp.WithBoolean("no_id_merge",
		mcp.Description("Disable automatic merging of path ID parameters with body 'id' fields"),
	),
	mcp.WithString("id_field_map",
		mcp.Description("Explicit path param to body field mappings (comma-separated: orderId=id,petId=id)"),
	),
	mcp.WithString("target_api_image",
		mcp.Description("Container image for target REST API (generates a Deployment+Service manifest for local testing)"),
	),
	mcp.WithNumber("target_api_port",
		mcp.Description("Container port for target REST API (overrides port from spec URL, default: 8080)"),
	),
	mcp.WithString("managed_crs",
		mcp.Description("Directory containing CR YAML files for managed Rundeck lifecycle jobs"),
	),
)

var describeTool = mcp.NewTool("describe",
	mcp.WithDescription("Inspect a previously generated operator. Shows the CRDs, their fields and operations, configuration options used, and file ownership. Reads the saved .openapi-operator-gen.yaml config and re-parses the spec."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("directory",
		mcp.Required(),
		mcp.Description("Path to the generated operator directory (must contain .openapi-operator-gen.yaml)"),
	),
)

var regenerateTool = mcp.NewTool("regenerate",
	mcp.WithDescription("Re-run generation for an existing operator using its saved configuration. Reads .openapi-operator-gen.yaml from the directory and re-generates all files. Optional parameters override saved values. After regeneration, run: go mod tidy && make generate && make build && make test"),
	mcp.WithString("directory",
		mcp.Required(),
		mcp.Description("Path to the generated operator directory (must contain .openapi-operator-gen.yaml)"),
	),
	mcp.WithString("spec",
		mcp.Description("Override the OpenAPI spec path or URL"),
	),
	mcp.WithString("group",
		mcp.Description("Override Kubernetes API group"),
	),
	mcp.WithString("module",
		mcp.Description("Override Go module name"),
	),
	mcp.WithString("version",
		mcp.Description("Override Kubernetes API version"),
	),
	mcp.WithString("mapping",
		mcp.Description("Override resource mapping mode: 'per-resource' or 'single-crd'"),
	),
	mcp.WithBoolean("aggregate",
		mcp.Description("Override: generate Status Aggregator CRD"),
	),
	mcp.WithBoolean("bundle",
		mcp.Description("Override: generate Bundle CRD"),
	),
	mcp.WithBoolean("kubectl_plugin",
		mcp.Description("Override: generate kubectl plugin"),
	),
	mcp.WithBoolean("rundeck_project",
		mcp.Description("Override: generate Rundeck projects"),
	),
	mcp.WithString("include_paths",
		mcp.Description("Override: path include patterns (comma-separated)"),
	),
	mcp.WithString("exclude_paths",
		mcp.Description("Override: path exclude patterns (comma-separated)"),
	),
)

var diffTool = mcp.NewTool("diff",
	mcp.WithDescription("Compare the current OpenAPI spec against what was last generated from. Shows added, removed, and changed CRDs with field-level detail. Uses the spec hash for fast no-change detection, and git history or the embedded spec copy for detailed comparison."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("directory",
		mcp.Required(),
		mcp.Description("Path to the generated operator directory (must contain .openapi-operator-gen.yaml)"),
	),
	mcp.WithString("spec",
		mcp.Description("Override the new spec path to compare against (default: uses spec path from saved config)"),
	),
)

// Prompt definitions

var generateOperatorPrompt = mcp.NewPrompt("generate-operator",
	mcp.WithPromptDescription("Walk through generating a Kubernetes operator from an OpenAPI spec. Guides you through spec validation, resource preview, option selection, and generation."),
	mcp.WithArgument("spec",
		mcp.ArgumentDescription("Path or URL to the OpenAPI specification file"),
		mcp.RequiredArgument(),
	),
	mcp.WithArgument("output",
		mcp.ArgumentDescription("Output directory for generated operator code"),
	),
	mcp.WithArgument("group",
		mcp.ArgumentDescription("Kubernetes API group (e.g., myapp.example.com)"),
	),
	mcp.WithArgument("module",
		mcp.ArgumentDescription("Go module name (e.g., github.com/myorg/myapp-operator)"),
	),
	mcp.WithArgument("version",
		mcp.ArgumentDescription("Kubernetes API version (default: v1alpha1)"),
	),
	mcp.WithArgument("mapping",
		mcp.ArgumentDescription("Resource mapping mode: 'per-resource' (one CRD per REST resource) or 'single-crd' (one CRD for entire API)"),
	),
)

var previewAPIPrompt = mcp.NewPrompt("preview-api",
	mcp.WithPromptDescription("Explore an OpenAPI spec to see what Kubernetes resources would be generated, without writing any files."),
	mcp.WithArgument("spec",
		mcp.ArgumentDescription("Path or URL to the OpenAPI specification file"),
		mcp.RequiredArgument(),
	),
	mcp.WithArgument("group",
		mcp.ArgumentDescription("Kubernetes API group for Kind name derivation (e.g., myapp.example.com)"),
	),
)

var evolveSpecPrompt = mcp.NewPrompt("evolve-spec",
	mcp.WithPromptDescription("Walk through evolving an existing operator after spec or generator changes. Describes the current state, diffs changes, regenerates, builds, and reviews file-level changes in git."),
	mcp.WithArgument("directory",
		mcp.ArgumentDescription("Path to the generated operator directory"),
		mcp.RequiredArgument(),
	),
	mcp.WithArgument("spec",
		mcp.ArgumentDescription("Path to the updated OpenAPI spec (if different from the saved config)"),
	),
)

// handlers holds version info and implements the MCP tool handlers.
type handlers struct {
	version string
	commit  string
	date    string
}

// handleValidate parses an OpenAPI spec and returns a summary.
func (h *handlers) handleValidate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	specPath := mcp.ParseString(req, "spec", "")
	if specPath == "" {
		return mcp.NewToolResultError("'spec' parameter is required"), nil
	}

	p := parser.NewParser()
	spec, err := p.Parse(specPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse OpenAPI spec: %v", err)), nil
	}

	var b strings.Builder
	b.WriteString("OpenAPI Specification: Valid\n\n")
	if spec.Title != "" {
		fmt.Fprintf(&b, "  Title:       %s\n", spec.Title)
	}
	if spec.Version != "" {
		fmt.Fprintf(&b, "  Version:     %s\n", spec.Version)
	}
	if spec.Description != "" {
		fmt.Fprintf(&b, "  Description: %s\n", spec.Description)
	}
	if spec.BaseURL != "" {
		fmt.Fprintf(&b, "  Base URL:    %s\n", spec.BaseURL)
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "  Resources (CRUD):        %d\n", len(spec.Resources))
	fmt.Fprintf(&b, "  Query Endpoints (GET):   %d\n", len(spec.QueryEndpoints))
	fmt.Fprintf(&b, "  Action Endpoints (POST): %d\n", len(spec.ActionEndpoints))

	return mcp.NewToolResultText(b.String()), nil
}

// handlePreview parses and maps a spec to CRDs without generating files.
func (h *handlers) handlePreview(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	specPath := mcp.ParseString(req, "spec", "")
	if specPath == "" {
		return mcp.NewToolResultError("'spec' parameter is required"), nil
	}

	cfg := &config.Config{
		SpecPath:    specPath,
		APIGroup:    mcp.ParseString(req, "group", "example.com"),
		APIVersion:  "v1alpha1",
		MappingMode: config.PerResource,
	}
	cfg.IncludePaths = parseCommaSeparated(mcp.ParseString(req, "include_paths", ""))
	cfg.ExcludePaths = parseCommaSeparated(mcp.ParseString(req, "exclude_paths", ""))
	cfg.IncludeTags = parseCommaSeparated(mcp.ParseString(req, "include_tags", ""))
	cfg.ExcludeTags = parseCommaSeparated(mcp.ParseString(req, "exclude_tags", ""))
	cfg.IncludeOperations = parseCommaSeparated(mcp.ParseString(req, "include_operations", ""))
	cfg.ExcludeOperations = parseCommaSeparated(mcp.ParseString(req, "exclude_operations", ""))

	filter := config.NewPathFilter(cfg)
	p := parser.NewParserWithFilter(cfg.RootKind, filter)
	spec, err := p.Parse(specPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse OpenAPI spec: %v", err)), nil
	}

	m := mapper.NewMapper(cfg)
	crds, err := m.MapResources(spec)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to map resources to CRDs: %v", err)), nil
	}

	var b strings.Builder

	// Header with spec metadata
	if spec.Title != "" {
		fmt.Fprintf(&b, "%s", spec.Title)
		if spec.Version != "" {
			fmt.Fprintf(&b, " (v%s)", spec.Version)
		}
		b.WriteString("\n")
	}
	if spec.Description != "" {
		fmt.Fprintf(&b, "%s\n", spec.Description)
	}
	if spec.BaseURL != "" {
		fmt.Fprintf(&b, "Base URL: %s\n", spec.BaseURL)
	}
	b.WriteString("\n")

	formatCRDs(&b, crds)

	return mcp.NewToolResultText(b.String()), nil
}

// handleGenerate runs the full generation pipeline.
func (h *handlers) handleGenerate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := h.configFromRequest(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return h.runGeneration(cfg)
}

// runGeneration executes the full generation pipeline for a given config.
// Used by both handleGenerate and handleRegenerate.
func (h *handlers) runGeneration(cfg *config.Config) (*mcp.CallToolResult, error) {
	// Validate
	if err := cfg.Validate(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid configuration: %v", err)), nil
	}

	// Compute spec hash before generation
	if hash, err := config.HashSpecFile(cfg.SpecPath); err == nil {
		cfg.SpecHash = hash
	}

	// Parse spec
	filter := config.NewPathFilter(cfg)
	p := parser.NewParserWithFilter(cfg.RootKind, filter)
	spec, err := p.Parse(cfg.SpecPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse OpenAPI spec: %v", err)), nil
	}
	cfg.SpecBaseURL = spec.BaseURL

	// Map resources to CRDs
	m := mapper.NewMapper(cfg)
	crds, err := m.MapResources(spec)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to map resources: %v", err)), nil
	}

	var messages []string
	messages = append(messages, fmt.Sprintf("Parsed %d resources, %d queries, %d actions from spec",
		len(spec.Resources), len(spec.QueryEndpoints), len(spec.ActionEndpoints)))
	messages = append(messages, fmt.Sprintf("Mapped to %d CRD definitions", len(crds)))

	// Generate types
	typesGen := generator.NewTypesGenerator(cfg)
	if err := typesGen.Generate(crds); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to generate types: %v", err)), nil
	}
	messages = append(messages, "Generated api/<version>/types.go")

	// Generate CRD YAML (optional)
	if cfg.GenerateCRDs {
		crdGen := generator.NewCRDGenerator(cfg)
		if err := crdGen.Generate(crds); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to generate CRD YAML: %v", err)), nil
		}
		messages = append(messages, "Generated config/crd/bases/*.yaml")
	}

	// Aggregate CRD
	var aggregate *mapper.AggregateDefinition
	if cfg.GenerateAggregate {
		aggregate = m.CreateAggregateDefinition(crds)
		if err := typesGen.GenerateAggregateTypes(aggregate); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to generate aggregate types: %v", err)), nil
		}
		messages = append(messages, "Generated aggregate CRD types")
	}

	// Bundle CRD
	var bundle *mapper.BundleDefinition
	if cfg.GenerateBundle {
		bundle = m.CreateBundleDefinition(crds)
		if err := typesGen.GenerateBundleTypes(bundle); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to generate bundle types: %v", err)), nil
		}
		messages = append(messages, "Generated bundle CRD types")
	}

	// Samples
	samplesGen := generator.NewSamplesGenerator(cfg)
	if err := samplesGen.Generate(crds, aggregate, bundle); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to generate samples: %v", err)), nil
	}
	messages = append(messages, "Generated config/samples/*.yaml")

	// Controllers
	controllerGen := generator.NewControllerGenerator(cfg)
	if err := controllerGen.Generate(crds, aggregate, bundle); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to generate controllers: %v", err)), nil
	}
	messages = append(messages, "Generated controllers, main.go, Dockerfile, Makefile")

	if cfg.TargetAPIImage != "" {
		if err := controllerGen.GenerateTargetAPIDeployment(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to generate target API deployment: %v", err)), nil
		}
		messages = append(messages, "Generated config/target-api/deployment.yaml")
	}

	if err := controllerGen.GenerateDockerCompose(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to generate docker-compose.yaml: %v", err)), nil
	}

	// Aggregate controller
	if aggregate != nil {
		if err := controllerGen.GenerateAggregateController(aggregate); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to generate aggregate controller: %v", err)), nil
		}
		messages = append(messages, "Generated aggregate controller")
	}

	// Bundle controller
	if bundle != nil {
		if err := controllerGen.GenerateBundleController(bundle); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to generate bundle controller: %v", err)), nil
		}
		messages = append(messages, "Generated bundle controller")
	}

	// CEL tests
	if aggregate != nil || bundle != nil {
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
			return mcp.NewToolResultError(fmt.Sprintf("Failed to generate CEL tests: %v", err)), nil
		}
		if err := controllerGen.GenerateCELTestData(resourceKinds, queryKinds, actionKinds, allKinds, aggregateKind, bundleKind, crds); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to generate CEL test data: %v", err)), nil
		}
		messages = append(messages, "Generated CEL tests and test data")
	}

	// kubectl plugin
	if cfg.GenerateKubectlPlugin {
		kubectlPluginGen := generator.NewKubectlPluginGenerator(cfg)
		if err := kubectlPluginGen.Generate(crds, aggregate, bundle); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to generate kubectl plugin: %v", err)), nil
		}
		messages = append(messages, "Generated kubectl plugin")
	}

	// Rundeck project
	if cfg.GenerateRundeckProject {
		if !cfg.GenerateKubectlPlugin {
			return mcp.NewToolResultError("rundeck_project requires kubectl_plugin to be enabled"), nil
		}
		rundeckGen := generator.NewRundeckProjectGenerator(cfg)
		if err := rundeckGen.Generate(crds); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to generate Rundeck project: %v", err)), nil
		}
		if err := rundeckGen.GenerateDockerProject(crds); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to generate Rundeck Docker project: %v", err)), nil
		}
		if err := rundeckGen.GenerateK8sProject(crds); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to generate Rundeck K8s project: %v", err)), nil
		}
		if err := rundeckGen.GeneratePluginDockerfile(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to generate plugin Dockerfile: %v", err)), nil
		}
		if !cfg.StandaloneNodeSource {
			if err := rundeckGen.GenerateNodeSourcePlugin(); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to generate node source plugin: %v", err)), nil
			}
		}
		if cfg.ManagedCRsDir != "" {
			if err := rundeckGen.GenerateManagedJobs(cfg.ManagedCRsDir); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to generate managed CR jobs: %v", err)), nil
			}
		}
		messages = append(messages, "Generated Rundeck projects")
	}

	// Build result summary
	messages = append(messages, "Saved .openapi-operator-gen.yaml config")
	var b strings.Builder
	b.WriteString("Operator generated successfully!\n\n")
	for _, msg := range messages {
		fmt.Fprintf(&b, "- %s\n", msg)
	}
	b.WriteString("\nGenerated CRDs:\n")
	for _, crd := range crds {
		fmt.Fprintf(&b, "- %s (%s)\n", crd.Kind, crd.Plural)
	}
	b.WriteString("\nNext steps:\n")
	fmt.Fprintf(&b, "  1. cd %s\n", cfg.OutputDir)
	b.WriteString("  2. go mod tidy\n")
	b.WriteString("  3. make generate  # Generate deep copy methods and CRD manifests\n")
	b.WriteString("  4. make build     # Build the operator binary\n")
	b.WriteString("  5. make test      # Run tests\n")
	b.WriteString("  6. make install   # Install CRDs to cluster\n")
	b.WriteString("  7. make run       # Run the operator locally\n")
	if cfg.GenerateKubectlPlugin {
		b.WriteString("\nTo build the kubectl plugin:\n")
		fmt.Fprintf(&b, "  cd %s/kubectl-plugin && make install\n", cfg.OutputDir)
	}

	return mcp.NewToolResultText(b.String()), nil
}

// handleGenerateOperatorPrompt returns instructions that guide the assistant
// through the full generate workflow: validate → preview → discuss options → generate → build.
func (h *handlers) handleGenerateOperatorPrompt(_ context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	spec := req.Params.Arguments["spec"]
	output := req.Params.Arguments["output"]
	group := req.Params.Arguments["group"]
	module := req.Params.Arguments["module"]
	version := req.Params.Arguments["version"]
	mapping := req.Params.Arguments["mapping"]

	var b strings.Builder
	b.WriteString("I want to generate a Kubernetes operator from an OpenAPI specification.\n\n")
	fmt.Fprintf(&b, "The OpenAPI spec is at: %s\n", spec)
	if output != "" {
		fmt.Fprintf(&b, "Output directory: %s\n", output)
	}
	if group != "" {
		fmt.Fprintf(&b, "API group: %s\n", group)
	}
	if module != "" {
		fmt.Fprintf(&b, "Go module: %s\n", module)
	}
	if version != "" {
		fmt.Fprintf(&b, "API version: %s\n", version)
	}
	if mapping != "" {
		fmt.Fprintf(&b, "Mapping mode: %s\n", mapping)
	}

	instructions := `Follow these steps to generate the operator:

1. **Validate** the spec using the validate tool to confirm it is parseable.

2. **Preview** the resources using the preview tool` + func() string {
		if group != "" {
			return fmt.Sprintf(" with group=%q", group)
		}
		return ""
	}() + `. Show me what CRDs would be generated — the Resources (CRUD), Query Endpoints (GET-only), and Action Endpoints (POST/PUT-only).

3. **Discuss options** before generating. Ask me about:
   - Which output directory and Go module name to use (if not already provided)
   - Which API group and version to use (if not already provided)
   - **Mapping mode** (if not already provided): "per-resource" creates one CRD per REST resource (default), "single-crd" creates a single CRD for the entire API
   - Whether to enable optional features:
     - **aggregate**: A Status Aggregator CRD that monitors health across all resources
     - **bundle**: A Bundle CRD for creating multiple resources as a unit
     - **generate_crds**: Generate CRD YAML manifests directly (default: use controller-gen via 'make generate')
     - **kubectl_plugin**: A kubectl plugin for managing the operator
     - **rundeck_project**: Rundeck job definitions for web-based management (requires kubectl_plugin)
     - **standalone_node_source**: Use the generic kubectl-rundeck-nodes plugin instead of generating a per-API node source (only with rundeck_project)
   - Whether any paths, tags, or operations should be filtered (include or exclude patterns)
   - **update_with_post**: Whether any resources should use POST for updates because the API lacks PUT endpoints (can be "*" for all, or specific paths)
   - **ID field handling**: Whether to disable automatic merging of path ID parameters with body 'id' fields (no_id_merge), or provide explicit mappings (id_field_map)
   - **Target API deployment**: Whether to include a container image and port for the target REST API (generates a Deployment+Service manifest for local testing)
   - **managed_crs**: A directory of CR YAML files to generate managed Rundeck lifecycle jobs (only with rundeck_project)

4. **Generate** the operator using the generate tool with the confirmed options.

5. **Build** the generated operator by running these commands in the output directory:
   - go mod tidy
   - make generate
   - make build
   - make test
   Report the results of each step.`

	return mcp.NewGetPromptResult(
		"Generate a Kubernetes operator from an OpenAPI spec",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleUser,
				mcp.NewTextContent(b.String()+"\n"+instructions),
			),
		},
	), nil
}

// handlePreviewAPIPrompt returns instructions for exploring an API spec.
func (h *handlers) handlePreviewAPIPrompt(_ context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	spec := req.Params.Arguments["spec"]
	group := req.Params.Arguments["group"]

	var previewArgs string
	if group != "" {
		previewArgs = fmt.Sprintf(" with group=%q", group)
	}

	text := fmt.Sprintf(`I want to explore what Kubernetes resources would be generated from an OpenAPI spec before committing to generation.

The OpenAPI spec is at: %s

Follow these steps:

1. **Validate** the spec using the validate tool. Show me the title, version, and endpoint counts.

2. **Preview** the resources using the preview tool%s. Show me the full breakdown:
   - Resources (CRUD) — what Kind names, paths, HTTP methods, and spec fields each would have
   - Query Endpoints (GET-only) — read-only data fetchers
   - Action Endpoints (POST/PUT-only) — one-shot operations

3. **Summarize** the findings: how many CRDs total, any notable patterns (e.g., resources without DELETE, actions tied to parent resources), and what optional features (aggregate, bundle, kubectl plugin) might be useful for this API.

4. If the preview looks noisy, suggest filtering options: include/exclude paths, tags, or operationIds to narrow the scope.`, spec, previewArgs)

	return mcp.NewGetPromptResult(
		"Preview what CRDs an OpenAPI spec would produce",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleUser,
				mcp.NewTextContent(text),
			),
		},
	), nil
}

// handleDescribe inspects a previously generated operator.
func (h *handlers) handleDescribe(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	directory := mcp.ParseString(req, "directory", "")
	if directory == "" {
		return mcp.NewToolResultError("'directory' parameter is required"), nil
	}

	// Load saved config
	configPath := filepath.Join(directory, ".openapi-operator-gen.yaml")
	file, err := config.LoadConfigFile(configPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load config: %v", err)), nil
	}
	if file == nil {
		return mcp.NewToolResultError(fmt.Sprintf("No .openapi-operator-gen.yaml found in %s", directory)), nil
	}

	cfg := config.ConfigFromFile(file)
	cfg.OutputDir = directory

	// Parse spec and map to CRDs
	filter := config.NewPathFilter(cfg)
	p := parser.NewParserWithFilter(cfg.RootKind, filter)
	spec, err := p.Parse(cfg.SpecPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse OpenAPI spec at %s: %v", cfg.SpecPath, err)), nil
	}

	m := mapper.NewMapper(cfg)
	crds, err := m.MapResources(spec)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to map resources: %v", err)), nil
	}

	var b strings.Builder

	// Header with spec metadata
	if spec.Title != "" {
		fmt.Fprintf(&b, "%s", spec.Title)
		if spec.Version != "" {
			fmt.Fprintf(&b, " (v%s)", spec.Version)
		}
		b.WriteString("\n")
	} else {
		appName := strings.Split(cfg.APIGroup, ".")[0]
		titleAppName := appName
		if len(appName) > 0 {
			titleAppName = strings.ToUpper(appName[:1]) + appName[1:]
		}
		fmt.Fprintf(&b, "%s Operator\n", titleAppName)
	}
	if spec.Description != "" {
		fmt.Fprintf(&b, "%s\n", spec.Description)
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "  API Group:   %s\n", cfg.APIGroup)
	fmt.Fprintf(&b, "  API Version: %s\n", cfg.APIVersion)
	fmt.Fprintf(&b, "  Module:      %s\n", cfg.ModuleName)
	if spec.BaseURL != "" {
		fmt.Fprintf(&b, "  Base URL:    %s\n", spec.BaseURL)
	}

	// Spec status with hash comparison
	fmt.Fprintf(&b, "  Spec:        %s", cfg.SpecPath)
	if cfg.SpecHash != "" {
		currentHash, hashErr := config.HashSpecFile(cfg.SpecPath)
		if hashErr == nil {
			if currentHash == cfg.SpecHash {
				b.WriteString(" (unchanged since last generation)")
			} else {
				b.WriteString(" (MODIFIED since last generation — run diff to see changes)")
			}
		}
	}
	b.WriteString("\n")

	// Generator version status
	if cfg.GeneratorVersion != "" {
		if cfg.GeneratorVersion == h.version {
			fmt.Fprintf(&b, "  Generator:   %s (current)\n", cfg.GeneratorVersion)
		} else {
			fmt.Fprintf(&b, "  Generator:   %s → %s available (run regenerate to upgrade)\n", cfg.GeneratorVersion, h.version)
		}
	} else {
		fmt.Fprintf(&b, "  Generator:   unknown (generated before version tracking was added)\n")
	}
	b.WriteString("\n")

	// Configuration options
	b.WriteString("CONFIGURATION:\n")
	fmt.Fprintf(&b, "  Mapping mode:       %s\n", cfg.MappingMode)
	if cfg.GenerateAggregate {
		b.WriteString("  Aggregate CRD:      enabled\n")
	}
	if cfg.GenerateBundle {
		b.WriteString("  Bundle CRD:         enabled\n")
	}
	if cfg.GenerateKubectlPlugin {
		b.WriteString("  kubectl plugin:     enabled\n")
	}
	if cfg.GenerateRundeckProject {
		b.WriteString("  Rundeck project:    enabled\n")
	}
	if cfg.GenerateCRDs {
		b.WriteString("  CRD YAML gen:       enabled\n")
	}
	if len(cfg.UpdateWithPost) > 0 {
		fmt.Fprintf(&b, "  Update with POST:   %s\n", strings.Join(cfg.UpdateWithPost, ", "))
	}
	if cfg.NoIDMerge {
		b.WriteString("  ID merge:           disabled\n")
	}
	if len(cfg.IDFieldMap) > 0 {
		pairs := make([]string, 0, len(cfg.IDFieldMap))
		for k, v := range cfg.IDFieldMap {
			pairs = append(pairs, k+"="+v)
		}
		fmt.Fprintf(&b, "  ID field map:       %s\n", strings.Join(pairs, ", "))
	}
	if cfg.TargetAPIImage != "" {
		fmt.Fprintf(&b, "  Target API image:   %s\n", cfg.TargetAPIImage)
	}
	if len(cfg.IncludePaths) > 0 {
		fmt.Fprintf(&b, "  Include paths:      %s\n", strings.Join(cfg.IncludePaths, ", "))
	}
	if len(cfg.ExcludePaths) > 0 {
		fmt.Fprintf(&b, "  Exclude paths:      %s\n", strings.Join(cfg.ExcludePaths, ", "))
	}
	if len(cfg.IncludeTags) > 0 {
		fmt.Fprintf(&b, "  Include tags:       %s\n", strings.Join(cfg.IncludeTags, ", "))
	}
	if len(cfg.ExcludeTags) > 0 {
		fmt.Fprintf(&b, "  Exclude tags:       %s\n", strings.Join(cfg.ExcludeTags, ", "))
	}
	b.WriteString("\n")

	// CRDs (reuse shared formatter)
	formatCRDs(&b, crds)
	b.WriteString("\n")

	// File ownership
	b.WriteString("FILE OWNERSHIP:\n\n")
	b.WriteString("  Regenerated (overwritten on re-generation — do not hand-edit):\n")
	fmt.Fprintf(&b, "    api/%s/              CRD Go types with kubebuilder markers\n", cfg.APIVersion)
	b.WriteString("    internal/controller/   Reconciliation logic for each CRD\n")
	b.WriteString("    config/crd/            CRD YAML manifests\n")
	b.WriteString("    main.go                Controller manager entrypoint\n")
	b.WriteString("    Dockerfile, Makefile, go.mod\n\n")
	b.WriteString("  Safe to customize:\n")
	b.WriteString("    config/manager/        Deployment resource limits, replicas, env vars\n")
	b.WriteString("    config/rbac/           Additional RBAC rules\n")
	b.WriteString("    config/samples/        Example CR YAML files\n")
	b.WriteString("    config/default/        Kustomize overlays\n")

	return mcp.NewToolResultText(b.String()), nil
}

// handleRegenerate re-runs generation using saved config with optional overrides.
func (h *handlers) handleRegenerate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	directory := mcp.ParseString(req, "directory", "")
	if directory == "" {
		return mcp.NewToolResultError("'directory' parameter is required"), nil
	}

	// Load saved config
	configPath := filepath.Join(directory, ".openapi-operator-gen.yaml")
	file, err := config.LoadConfigFile(configPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load config: %v", err)), nil
	}
	if file == nil {
		return mcp.NewToolResultError(fmt.Sprintf("No .openapi-operator-gen.yaml found in %s", directory)), nil
	}

	cfg := config.ConfigFromFile(file)
	cfg.OutputDir = directory
	cfg.GeneratorVersion = h.version
	cfg.CommitHash = h.commit
	cfg.CommitTimestamp = h.date

	// Apply overrides from request
	if v := mcp.ParseString(req, "spec", ""); v != "" {
		cfg.SpecPath = v
	}
	if v := mcp.ParseString(req, "group", ""); v != "" {
		cfg.APIGroup = v
	}
	if v := mcp.ParseString(req, "module", ""); v != "" {
		cfg.ModuleName = v
	}
	if v := mcp.ParseString(req, "version", ""); v != "" {
		cfg.APIVersion = v
	}
	if v := mcp.ParseString(req, "mapping", ""); v != "" {
		cfg.MappingMode = config.MappingMode(v)
	}
	if mcp.ParseBoolean(req, "aggregate", false) {
		cfg.GenerateAggregate = true
	}
	if mcp.ParseBoolean(req, "bundle", false) {
		cfg.GenerateBundle = true
	}
	if mcp.ParseBoolean(req, "kubectl_plugin", false) {
		cfg.GenerateKubectlPlugin = true
	}
	if mcp.ParseBoolean(req, "rundeck_project", false) {
		cfg.GenerateRundeckProject = true
	}
	if v := parseCommaSeparated(mcp.ParseString(req, "include_paths", "")); len(v) > 0 {
		cfg.IncludePaths = v
	}
	if v := parseCommaSeparated(mcp.ParseString(req, "exclude_paths", "")); len(v) > 0 {
		cfg.ExcludePaths = v
	}

	return h.runGeneration(cfg)
}

// handleDiff compares the current spec against what was last generated from.
func (h *handlers) handleDiff(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	directory := mcp.ParseString(req, "directory", "")
	if directory == "" {
		return mcp.NewToolResultError("'directory' parameter is required"), nil
	}

	// Load saved config
	configPath := filepath.Join(directory, ".openapi-operator-gen.yaml")
	file, err := config.LoadConfigFile(configPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load config: %v", err)), nil
	}
	if file == nil {
		return mcp.NewToolResultError(fmt.Sprintf("No .openapi-operator-gen.yaml found in %s", directory)), nil
	}

	cfg := config.ConfigFromFile(file)
	cfg.OutputDir = directory

	// Determine new spec path
	newSpecPath := mcp.ParseString(req, "spec", "")
	if newSpecPath == "" {
		newSpecPath = cfg.SpecPath
	}

	// Fast path: check spec hash
	if cfg.SpecHash != "" {
		currentHash, hashErr := config.HashSpecFile(newSpecPath)
		if hashErr == nil && currentHash == cfg.SpecHash {
			msg := fmt.Sprintf(
				"No changes detected. The spec is identical to what was last generated from.\n\nSpec: %s\nHash: %s",
				newSpecPath, cfg.SpecHash)
			if cfg.GeneratorVersion != "" && cfg.GeneratorVersion != h.version {
				msg += fmt.Sprintf("\n\nNote: Generator version has changed (%s → %s). Run regenerate to update generated code.",
					cfg.GeneratorVersion, h.version)
			}
			return mcp.NewToolResultText(msg), nil
		}
	}

	// Slow path: get old spec for detailed comparison
	specBasename := filepath.Base(cfg.SpecPath)
	embeddedSpecPath := filepath.Join(directory, specBasename)

	// Try git first to get the committed version
	var oldSpecPath string
	gitRef := fmt.Sprintf("HEAD:%s", embeddedSpecPath)
	gitCmd := exec.Command("git", "show", gitRef)
	gitOutput, gitErr := gitCmd.Output()
	if gitErr == nil && len(gitOutput) > 0 {
		// Write git content to a temp file for parsing
		tmpFile := filepath.Join(directory, ".openapi-operator-gen-diff-old-spec.tmp")
		if writeErr := writeFile(tmpFile, gitOutput); writeErr == nil {
			oldSpecPath = tmpFile
			defer removeFile(tmpFile)
		}
	}

	// Fall back to the embedded spec copy on disk
	if oldSpecPath == "" {
		if fileExists(embeddedSpecPath) {
			oldSpecPath = embeddedSpecPath
		}
	}

	if oldSpecPath == "" {
		if cfg.SpecHash != "" {
			return mcp.NewToolResultText(fmt.Sprintf(
				"Spec has changed (hash mismatch) but no previous spec copy found for detailed comparison.\n\n"+
					"Saved hash: %s\nSpec: %s\n\nRun regenerate to update the operator.",
				cfg.SpecHash, newSpecPath)), nil
		}
		return mcp.NewToolResultError("No previous spec found to compare against. Generate the operator first."), nil
	}

	// Parse old spec
	oldFilter := config.NewPathFilter(cfg)
	oldParser := parser.NewParserWithFilter(cfg.RootKind, oldFilter)
	oldSpec, err := oldParser.Parse(oldSpecPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse old spec: %v", err)), nil
	}
	oldMapper := mapper.NewMapper(cfg)
	oldCRDs, err := oldMapper.MapResources(oldSpec)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to map old resources: %v", err)), nil
	}

	// Parse new spec
	newFilter := config.NewPathFilter(cfg)
	newParser := parser.NewParserWithFilter(cfg.RootKind, newFilter)
	newSpec, err := newParser.Parse(newSpecPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse new spec: %v", err)), nil
	}
	newMapper := mapper.NewMapper(cfg)
	newCRDs, err := newMapper.MapResources(newSpec)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to map new resources: %v", err)), nil
	}

	// Build maps by Kind
	oldByKind := make(map[string]*mapper.CRDDefinition)
	for _, crd := range oldCRDs {
		oldByKind[crd.Kind] = crd
	}
	newByKind := make(map[string]*mapper.CRDDefinition)
	for _, crd := range newCRDs {
		newByKind[crd.Kind] = crd
	}

	// Compute diff
	var added, removed, changed, unchanged []string
	for kind := range newByKind {
		if _, ok := oldByKind[kind]; !ok {
			added = append(added, kind)
		}
	}
	for kind := range oldByKind {
		if _, ok := newByKind[kind]; !ok {
			removed = append(removed, kind)
		}
	}
	for kind, newCRD := range newByKind {
		oldCRD, ok := oldByKind[kind]
		if !ok {
			continue
		}
		changes := compareCRDs(oldCRD, newCRD)
		if len(changes) > 0 {
			changed = append(changed, kind)
		} else {
			unchanged = append(unchanged, kind)
		}
	}

	// Format output
	var b strings.Builder
	fmt.Fprintf(&b, "Spec Diff: %s\n", filepath.Base(newSpecPath))
	fmt.Fprintf(&b, "Summary: %d added, %d removed, %d changed, %d unchanged\n",
		len(added), len(removed), len(changed), len(unchanged))

	if cfg.GeneratorVersion != "" && cfg.GeneratorVersion != h.version {
		fmt.Fprintf(&b, "\nNote: Generator version has also changed (%s → %s). Regeneration will update both spec changes and generated code templates.\n",
			cfg.GeneratorVersion, h.version)
	}
	b.WriteString("\n")

	if len(added) == 0 && len(removed) == 0 && len(changed) == 0 {
		b.WriteString("No changes detected. The spec matches the last generation.\n")
		return mcp.NewToolResultText(b.String()), nil
	}

	if len(added) > 0 {
		b.WriteString("ADDED CRDs:\n")
		for _, kind := range added {
			crd := newByKind[kind]
			crdType := "Resource"
			if crd.IsQuery {
				crdType = "QueryEndpoint"
			} else if crd.IsAction {
				crdType = "ActionEndpoint"
			}
			fmt.Fprintf(&b, "  + %s (%s)\n", kind, crdType)
		}
		b.WriteString("\n")
	}

	if len(removed) > 0 {
		b.WriteString("REMOVED CRDs:\n")
		for _, kind := range removed {
			fmt.Fprintf(&b, "  - %s\n", kind)
		}
		b.WriteString("\n")
	}

	if len(changed) > 0 {
		b.WriteString("CHANGED CRDs:\n")
		for _, kind := range changed {
			changes := compareCRDs(oldByKind[kind], newByKind[kind])
			fmt.Fprintf(&b, "\n  %s:\n", kind)
			for _, change := range changes {
				fmt.Fprintf(&b, "    ~ %s\n", change)
			}
		}
		b.WriteString("\n")
	}

	return mcp.NewToolResultText(b.String()), nil
}

// handleEvolveSpecPrompt returns instructions for evolving an operator after spec changes.
func (h *handlers) handleEvolveSpecPrompt(_ context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	directory := req.Params.Arguments["directory"]
	spec := req.Params.Arguments["spec"]

	var b strings.Builder
	b.WriteString("I want to update an existing Kubernetes operator after changes to the OpenAPI spec.\n\n")
	fmt.Fprintf(&b, "The operator directory is: %s\n", directory)
	if spec != "" {
		fmt.Fprintf(&b, "The updated spec is at: %s\n", spec)
	}

	instructions := `Follow these steps:

1. **Describe** the current operator using the describe tool to understand its current state (CRDs, configuration, spec status).

2. **Discuss** what changes were made to the spec. If I haven't explained, ask me what changed and why.

3. **Diff** the spec changes using the diff tool` + func() string {
		if spec != "" {
			return fmt.Sprintf(` with spec=%q`, spec)
		}
		return ""
	}() + `. Show me what CRDs would be added, removed, or changed.

4. **Review** the diff with me. Highlight any breaking changes (removed CRDs, changed field types) and ask if I want to proceed.

5. **Regenerate** the operator using the regenerate tool` + func() string {
		if spec != "" {
			return fmt.Sprintf(` with spec=%q`, spec)
		}
		return ""
	}() + `.

6. **Build** the regenerated operator:
   - go mod tidy
   - make generate
   - make build
   - make test
   Report the results of each step.

7. **Review** the file-level changes using git (if the directory is a git repo):
   - Run ` + "`git diff --stat`" + ` to show which files changed
   - Run ` + "`git diff`" + ` on key files if I want to see details
   - Highlight any files with unexpected changes`

	return mcp.NewGetPromptResult(
		"Update an operator after OpenAPI spec changes",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleUser,
				mcp.NewTextContent(b.String()+"\n"+instructions),
			),
		},
	), nil
}

// compareCRDs compares two CRD definitions and returns a list of human-readable changes.
func compareCRDs(old, new *mapper.CRDDefinition) []string {
	var changes []string

	// Compare operations
	oldOps := make(map[string]string)
	for _, op := range old.Operations {
		oldOps[op.CRDAction] = op.HTTPMethod + " " + op.Path
	}
	newOps := make(map[string]string)
	for _, op := range new.Operations {
		newOps[op.CRDAction] = op.HTTPMethod + " " + op.Path
	}
	for action, detail := range newOps {
		if _, ok := oldOps[action]; !ok {
			changes = append(changes, fmt.Sprintf("Added operation: %s (%s)", action, detail))
		}
	}
	for action := range oldOps {
		if _, ok := newOps[action]; !ok {
			changes = append(changes, fmt.Sprintf("Removed operation: %s", action))
		}
	}

	// Compare spec fields
	oldFields := make(map[string]*mapper.FieldDefinition)
	if old.Spec != nil {
		for _, f := range old.Spec.Fields {
			oldFields[f.JSONName] = f
		}
	}
	newFields := make(map[string]*mapper.FieldDefinition)
	if new.Spec != nil {
		for _, f := range new.Spec.Fields {
			newFields[f.JSONName] = f
		}
	}
	for name, newF := range newFields {
		oldF, ok := oldFields[name]
		if !ok {
			req := ""
			if newF.Required {
				req = " (required)"
			}
			changes = append(changes, fmt.Sprintf("Added field: %s (%s)%s", name, newF.GoType, req))
			continue
		}
		if oldF.GoType != newF.GoType {
			changes = append(changes, fmt.Sprintf("Changed field type: %s %s -> %s", name, oldF.GoType, newF.GoType))
		}
		if oldF.Required != newF.Required {
			if newF.Required {
				changes = append(changes, fmt.Sprintf("Field now required: %s", name))
			} else {
				changes = append(changes, fmt.Sprintf("Field now optional: %s", name))
			}
		}
	}
	for name := range oldFields {
		if _, ok := newFields[name]; !ok {
			changes = append(changes, fmt.Sprintf("Removed field: %s", name))
		}
	}

	// Compare query parameters (for query endpoints)
	if old.IsQuery && new.IsQuery {
		oldQP := make(map[string]string)
		for _, qp := range old.QueryParams {
			oldQP[qp.JSONName] = qp.GoType
		}
		newQP := make(map[string]string)
		for _, qp := range new.QueryParams {
			newQP[qp.JSONName] = qp.GoType
		}
		for name, newType := range newQP {
			oldType, ok := oldQP[name]
			if !ok {
				changes = append(changes, fmt.Sprintf("Added query param: %s (%s)", name, newType))
			} else if oldType != newType {
				changes = append(changes, fmt.Sprintf("Changed query param type: %s %s -> %s", name, oldType, newType))
			}
		}
		for name := range oldQP {
			if _, ok := newQP[name]; !ok {
				changes = append(changes, fmt.Sprintf("Removed query param: %s", name))
			}
		}
	}

	// Compare response type (for query endpoints)
	if old.IsQuery && new.IsQuery && old.ResponseType != new.ResponseType {
		changes = append(changes, fmt.Sprintf("Changed response type: %s -> %s", old.ResponseType, new.ResponseType))
	}

	return changes
}

// writeFile is a helper that writes data to a file.
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

// removeFile is a helper that removes a file, ignoring errors.
func removeFile(path string) {
	os.Remove(path)
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// configFromRequest builds a config.Config from MCP request parameters.
func (h *handlers) configFromRequest(req mcp.CallToolRequest) (*config.Config, error) {
	specPath := mcp.ParseString(req, "spec", "")
	if specPath == "" {
		return nil, fmt.Errorf("'spec' parameter is required")
	}
	outputDir := mcp.ParseString(req, "output", "")
	if outputDir == "" {
		return nil, fmt.Errorf("'output' parameter is required")
	}
	group := mcp.ParseString(req, "group", "")
	if group == "" {
		return nil, fmt.Errorf("'group' parameter is required")
	}
	module := mcp.ParseString(req, "module", "")
	if module == "" {
		return nil, fmt.Errorf("'module' parameter is required")
	}

	apiVersion := mcp.ParseString(req, "version", "v1alpha1")
	mappingMode := config.MappingMode(mcp.ParseString(req, "mapping", "per-resource"))

	cfg := &config.Config{
		SpecPath:               specPath,
		OutputDir:              outputDir,
		APIGroup:               group,
		APIVersion:             apiVersion,
		MappingMode:            mappingMode,
		ModuleName:             module,
		GeneratorVersion:       h.version,
		CommitHash:             h.commit,
		CommitTimestamp:        h.date,
		GenerateCRDs:           mcp.ParseBoolean(req, "generate_crds", false),
		RootKind:               mcp.ParseString(req, "root_kind", ""),
		GenerateAggregate:      mcp.ParseBoolean(req, "aggregate", false),
		GenerateBundle:         mcp.ParseBoolean(req, "bundle", false),
		GenerateKubectlPlugin:  mcp.ParseBoolean(req, "kubectl_plugin", false),
		GenerateRundeckProject: mcp.ParseBoolean(req, "rundeck_project", false),
		StandaloneNodeSource:   mcp.ParseBoolean(req, "standalone_node_source", false),
		NoIDMerge:              mcp.ParseBoolean(req, "no_id_merge", false),
		TargetAPIImage:         mcp.ParseString(req, "target_api_image", ""),
		TargetAPIPort:          mcp.ParseInt(req, "target_api_port", 0),
		ManagedCRsDir:          mcp.ParseString(req, "managed_crs", ""),
	}

	cfg.IncludePaths = parseCommaSeparated(mcp.ParseString(req, "include_paths", ""))
	cfg.ExcludePaths = parseCommaSeparated(mcp.ParseString(req, "exclude_paths", ""))
	cfg.IncludeTags = parseCommaSeparated(mcp.ParseString(req, "include_tags", ""))
	cfg.ExcludeTags = parseCommaSeparated(mcp.ParseString(req, "exclude_tags", ""))
	cfg.IncludeOperations = parseCommaSeparated(mcp.ParseString(req, "include_operations", ""))
	cfg.ExcludeOperations = parseCommaSeparated(mcp.ParseString(req, "exclude_operations", ""))
	cfg.UpdateWithPost = parseCommaSeparated(mcp.ParseString(req, "update_with_post", ""))
	cfg.IDFieldMap = parseIDFieldMap(mcp.ParseString(req, "id_field_map", ""))

	return cfg, nil
}

// formatCRDs writes rich markdown output for a list of CRD definitions.
// Used by handlePreview and handleDescribe.
func formatCRDs(b *strings.Builder, crds []*mapper.CRDDefinition) {
	// Classify CRDs
	var resources, queries, actions []*mapper.CRDDefinition
	for _, crd := range crds {
		switch {
		case crd.IsQuery:
			queries = append(queries, crd)
		case crd.IsAction:
			actions = append(actions, crd)
		default:
			resources = append(resources, crd)
		}
	}

	// Summary
	fmt.Fprintf(b, "CRDs: %d total (%d resources, %d queries, %d actions)\n\n",
		len(crds), len(resources), len(queries), len(actions))

	// Resources (CRUD)
	if len(resources) > 0 {
		fmt.Fprintf(b, "RESOURCES (CRUD) — %d:\n\n", len(resources))
		for _, crd := range resources {
			fmt.Fprintf(b, "  %s (%s)  scope=%s\n", crd.Kind, crd.Plural, crd.Scope)
			if crd.Description != "" {
				fmt.Fprintf(b, "    %s\n", crd.Description)
			}

			// Operations
			b.WriteString("    Operations:\n")
			for _, op := range crd.Operations {
				fmt.Fprintf(b, "      %-8s %s %s\n", op.CRDAction, op.HTTPMethod, op.Path)
			}
			if crd.UpdateWithPost {
				b.WriteString("      (uses POST for updates — PUT not available)\n")
			}

			// Spec fields
			if crd.Spec != nil && len(crd.Spec.Fields) > 0 {
				b.WriteString("    Spec fields:\n")
				for _, f := range crd.Spec.Fields {
					req := ""
					if f.Required {
						req = " (required)"
					}
					goType := f.GoType
					if len(f.Enum) > 0 {
						goType += " enum: " + strings.Join(f.Enum, ", ")
					}
					fmt.Fprintf(b, "      %-20s %s%s\n", f.JSONName, goType, req)
				}
			}

			// ID field mappings
			if len(crd.IDFieldMappings) > 0 {
				b.WriteString("    ID field mappings: ")
				mappings := make([]string, 0, len(crd.IDFieldMappings))
				for _, m := range crd.IDFieldMappings {
					mappings = append(mappings, fmt.Sprintf("{%s} -> %s", m.PathParam, m.BodyField))
				}
				b.WriteString(strings.Join(mappings, ", "))
				b.WriteString("\n")
			}

			if crd.NeedsExternalIDRef {
				b.WriteString("    Note: uses externalIDRef to reference existing resources (no path params for identification)\n")
			}
			b.WriteString("\n")
		}
	}

	// Query Endpoints
	if len(queries) > 0 {
		fmt.Fprintf(b, "QUERY ENDPOINTS (GET-only) — %d:\n\n", len(queries))
		for _, crd := range queries {
			fmt.Fprintf(b, "  %s (%s)  GET %s\n", crd.Kind, crd.Plural, crd.QueryPath)
			if crd.Description != "" {
				fmt.Fprintf(b, "    %s\n", crd.Description)
			}

			if crd.ResponseType != "" {
				fmt.Fprintf(b, "    Response type: %s", crd.ResponseType)
				if crd.ResponseIsArray {
					b.WriteString(" (array)")
				}
				b.WriteString("\n")
				if crd.UsesSharedType {
					fmt.Fprintf(b, "    Reuses type from: %s\n", crd.ResultItemType)
				}
			}

			// Query parameters
			if len(crd.QueryParams) > 0 {
				b.WriteString("    Query parameters:\n")
				for _, qp := range crd.QueryParams {
					req := ""
					if qp.Required {
						req = " (required)"
					}
					goType := qp.GoType
					if qp.IsArray {
						goType = "[]" + qp.ItemType
					}
					fmt.Fprintf(b, "      %-20s %s%s\n", qp.JSONName, goType, req)
				}
			}

			// Path parameters for query endpoints
			if len(crd.QueryPathParams) > 0 {
				b.WriteString("    Path parameters:\n")
				for _, pp := range crd.QueryPathParams {
					req := ""
					if pp.Required {
						req = " (required)"
					}
					fmt.Fprintf(b, "      %-20s %s%s\n", pp.JSONName, pp.GoType, req)
				}
			}

			// Result fields (when not using a shared type)
			if !crd.UsesSharedType && len(crd.ResultFields) > 0 {
				b.WriteString("    Result fields:\n")
				for _, f := range crd.ResultFields {
					fmt.Fprintf(b, "      %-20s %s\n", f.JSONName, f.GoType)
				}
			}
			b.WriteString("\n")
		}
	}

	// Action Endpoints
	if len(actions) > 0 {
		fmt.Fprintf(b, "ACTION ENDPOINTS (POST/PUT-only) — %d:\n\n", len(actions))
		for _, crd := range actions {
			fmt.Fprintf(b, "  %s (%s)  %s %s\n", crd.Kind, crd.Plural, crd.ActionMethod, crd.ActionPath)
			if crd.Description != "" {
				fmt.Fprintf(b, "    %s\n", crd.Description)
			}

			if crd.ParentResource != "" {
				fmt.Fprintf(b, "    Parent resource: %s (via %s)\n", crd.ParentResource, crd.ParentIDParam)
			}
			if crd.HasBinaryBody {
				fmt.Fprintf(b, "    Binary upload: %s\n", crd.BinaryContentType)
			}

			// Spec fields for action
			if crd.Spec != nil && len(crd.Spec.Fields) > 0 {
				b.WriteString("    Request fields:\n")
				for _, f := range crd.Spec.Fields {
					req := ""
					if f.Required {
						req = " (required)"
					}
					fmt.Fprintf(b, "      %-20s %s%s\n", f.JSONName, f.GoType, req)
				}
			}
			b.WriteString("\n")
		}
	}
}

// parseCommaSeparated splits a comma-separated string into a slice, trimming whitespace.
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

// parseIDFieldMap parses "key=value,key=value" into a map.
func parseIDFieldMap(s string) map[string]string {
	if s == "" {
		return nil
	}
	result := make(map[string]string)
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if kv := strings.SplitN(p, "=", 2); len(kv) == 2 {
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
