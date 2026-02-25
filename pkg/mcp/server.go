package mcp

import (
	"context"
	"fmt"
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

	s.AddPrompt(generateOperatorPrompt, h.handleGenerateOperatorPrompt)
	s.AddPrompt(previewAPIPrompt, h.handlePreviewAPIPrompt)

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
		fmt.Fprintf(&b, "Title: %s\n", spec.Title)
	}
	if spec.Version != "" {
		fmt.Fprintf(&b, "Version: %s\n", spec.Version)
	}
	if spec.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", spec.Description)
	}
	if spec.BaseURL != "" {
		fmt.Fprintf(&b, "Base URL: %s\n", spec.BaseURL)
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "Resources (CRUD):        %d\n", len(spec.Resources))
	fmt.Fprintf(&b, "Query Endpoints (GET):   %d\n", len(spec.QueryEndpoints))
	fmt.Fprintf(&b, "Action Endpoints (POST): %d\n", len(spec.ActionEndpoints))

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
		fmt.Fprintf(&b, "# %s", spec.Title)
		if spec.Version != "" {
			fmt.Fprintf(&b, " (v%s)", spec.Version)
		}
		b.WriteString("\n\n")
	}
	if spec.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", spec.Description)
	}
	if spec.BaseURL != "" {
		fmt.Fprintf(&b, "**Base URL:** `%s`\n\n", spec.BaseURL)
	}

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

	// Resources (CRUD)
	if len(resources) > 0 {
		fmt.Fprintf(&b, "## Resources (CRUD) — %d\n", len(resources))
		b.WriteString("Full lifecycle resources with GET, CREATE, UPDATE, DELETE operations.\n\n")
		for _, crd := range resources {
			fmt.Fprintf(&b, "### %s\n", crd.Kind)
			fmt.Fprintf(&b, "**Plural:** %s | **Scope:** %s\n", crd.Plural, crd.Scope)
			if crd.Description != "" {
				fmt.Fprintf(&b, "%s\n", crd.Description)
			}
			b.WriteString("\n")

			// Operations table
			b.WriteString("| Operation | Method | Path |\n")
			b.WriteString("|-----------|--------|------|\n")
			for _, op := range crd.Operations {
				fmt.Fprintf(&b, "| %s | %s | `%s` |\n", op.CRDAction, op.HTTPMethod, op.Path)
			}
			if crd.UpdateWithPost {
				b.WriteString("\n> Uses POST for updates (PUT not available)\n")
			}
			b.WriteString("\n")

			// Spec fields
			if crd.Spec != nil && len(crd.Spec.Fields) > 0 {
				b.WriteString("**Spec fields:**\n\n")
				b.WriteString("| Field | Type | Required |\n")
				b.WriteString("|-------|------|----------|\n")
				for _, f := range crd.Spec.Fields {
					req := ""
					if f.Required {
						req = "Yes"
					}
					goType := f.GoType
					if len(f.Enum) > 0 {
						goType += " enum: " + strings.Join(f.Enum, ", ")
					}
					fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", f.JSONName, goType, req)
				}
				b.WriteString("\n")
			}

			// ID field mappings
			if len(crd.IDFieldMappings) > 0 {
				b.WriteString("**ID field mappings:** ")
				mappings := make([]string, 0, len(crd.IDFieldMappings))
				for _, m := range crd.IDFieldMappings {
					mappings = append(mappings, fmt.Sprintf("`{%s}` -> `%s`", m.PathParam, m.BodyField))
				}
				b.WriteString(strings.Join(mappings, ", "))
				b.WriteString("\n\n")
			}

			if crd.NeedsExternalIDRef {
				b.WriteString("**Note:** Uses `externalIDRef` to reference existing resources (no path params for identification)\n\n")
			}
		}
	}

	// Query Endpoints
	if len(queries) > 0 {
		fmt.Fprintf(&b, "## Query Endpoints (GET-only) — %d\n", len(queries))
		b.WriteString("Read-only endpoints that periodically fetch data.\n\n")
		for _, crd := range queries {
			fmt.Fprintf(&b, "### %s\n", crd.Kind)
			fmt.Fprintf(&b, "**Path:** `GET %s`\n", crd.QueryPath)
			if crd.Description != "" {
				fmt.Fprintf(&b, "%s\n", crd.Description)
			}
			b.WriteString("\n")

			if crd.ResponseType != "" {
				fmt.Fprintf(&b, "**Response type:** `%s`", crd.ResponseType)
				if crd.ResponseIsArray {
					b.WriteString(" (array)")
				}
				b.WriteString("\n")
				if crd.UsesSharedType {
					fmt.Fprintf(&b, "**Reuses type from:** %s\n", crd.ResultItemType)
				}
				b.WriteString("\n")
			}

			// Query parameters
			if len(crd.QueryParams) > 0 {
				b.WriteString("**Query parameters:**\n\n")
				b.WriteString("| Parameter | Type | Required |\n")
				b.WriteString("|-----------|------|----------|\n")
				for _, qp := range crd.QueryParams {
					req := ""
					if qp.Required {
						req = "Yes"
					}
					goType := qp.GoType
					if qp.IsArray {
						goType = "[]" + qp.ItemType
					}
					fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", qp.JSONName, goType, req)
				}
				b.WriteString("\n")
			}

			// Path parameters for query endpoints
			if len(crd.QueryPathParams) > 0 {
				b.WriteString("**Path parameters:**\n\n")
				b.WriteString("| Parameter | Type | Required |\n")
				b.WriteString("|-----------|------|----------|\n")
				for _, pp := range crd.QueryPathParams {
					req := ""
					if pp.Required {
						req = "Yes"
					}
					fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", pp.JSONName, pp.GoType, req)
				}
				b.WriteString("\n")
			}

			// Result fields (when not using a shared type)
			if !crd.UsesSharedType && len(crd.ResultFields) > 0 {
				b.WriteString("**Result fields:**\n\n")
				b.WriteString("| Field | Type |\n")
				b.WriteString("|-------|------|\n")
				for _, f := range crd.ResultFields {
					fmt.Fprintf(&b, "| `%s` | `%s` |\n", f.JSONName, f.GoType)
				}
				b.WriteString("\n")
			}
		}
	}

	// Action Endpoints
	if len(actions) > 0 {
		fmt.Fprintf(&b, "## Action Endpoints (POST/PUT-only) — %d\n", len(actions))
		b.WriteString("One-shot or periodic operations.\n\n")
		for _, crd := range actions {
			fmt.Fprintf(&b, "### %s\n", crd.Kind)
			fmt.Fprintf(&b, "**Path:** `%s %s`\n", crd.ActionMethod, crd.ActionPath)
			if crd.Description != "" {
				fmt.Fprintf(&b, "%s\n", crd.Description)
			}
			b.WriteString("\n")

			if crd.ParentResource != "" {
				fmt.Fprintf(&b, "**Parent resource:** %s (via `%s`)\n", crd.ParentResource, crd.ParentIDParam)
			}
			if crd.HasBinaryBody {
				fmt.Fprintf(&b, "**Binary upload:** `%s`\n", crd.BinaryContentType)
			}

			// Spec fields for action
			if crd.Spec != nil && len(crd.Spec.Fields) > 0 {
				b.WriteString("\n**Request fields:**\n\n")
				b.WriteString("| Field | Type | Required |\n")
				b.WriteString("|-------|------|----------|\n")
				for _, f := range crd.Spec.Fields {
					req := ""
					if f.Required {
						req = "Yes"
					}
					fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", f.JSONName, f.GoType, req)
				}
			}
			b.WriteString("\n")
		}
	}

	// Summary
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "**Total CRDs:** %d", len(crds))
	if len(resources) > 0 || len(queries) > 0 || len(actions) > 0 {
		fmt.Fprintf(&b, " (%d resources, %d queries, %d actions)", len(resources), len(queries), len(actions))
	}
	b.WriteString("\n")

	return mcp.NewToolResultText(b.String()), nil
}

// handleGenerate runs the full generation pipeline.
func (h *handlers) handleGenerate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Build config from request parameters
	cfg, err := h.configFromRequest(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid configuration: %v", err)), nil
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
