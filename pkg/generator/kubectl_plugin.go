package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/bluecontainer/openapi-operator-gen/internal/config"
	"github.com/bluecontainer/openapi-operator-gen/pkg/mapper"
	"github.com/bluecontainer/openapi-operator-gen/pkg/templates"
)

// KubectlPluginGenerator generates kubectl plugin code
type KubectlPluginGenerator struct {
	config *config.Config
}

// NewKubectlPluginGenerator creates a new kubectl plugin generator
func NewKubectlPluginGenerator(cfg *config.Config) *KubectlPluginGenerator {
	return &KubectlPluginGenerator{config: cfg}
}

// KindInfo holds information about a CRD kind for the kubectl plugin
type KindInfo struct {
	Kind       string   // e.g., "Pet"
	KindLower  string   // e.g., "pet"
	Plural     string   // e.g., "pets"
	ShortNames []string // e.g., ["pet"]
}

// KubectlPluginTemplateData holds data for kubectl plugin templates
type KubectlPluginTemplateData struct {
	Year             int
	GeneratorVersion string
	APIGroup         string
	APIVersion       string
	APIName          string // e.g., "petstore"
	PluginName       string // e.g., "petstore"
	BinaryName       string // e.g., "kubectl-petstore"
	ModuleName       string // Go module name for the plugin
	ResourceKinds    []KindInfo
	QueryKinds       []KindInfo
	ActionKinds      []KindInfo
	AllKinds         []KindInfo
	HasAggregate     bool
	AggregateKind    string
	HasBundle        bool
	BundleKind       string
}

// Generate generates the kubectl plugin code
func (g *KubectlPluginGenerator) Generate(crds []*mapper.CRDDefinition, aggregate *mapper.AggregateDefinition, bundle *mapper.BundleDefinition) error {
	// Create plugin directory structure
	pluginDir := filepath.Join(g.config.OutputDir, "kubectl-plugin")
	dirs := []string{
		pluginDir,
		filepath.Join(pluginDir, "cmd"),
		filepath.Join(pluginDir, "pkg", "client"),
		filepath.Join(pluginDir, "pkg", "output"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Prepare template data
	data := g.prepareTemplateData(crds, aggregate, bundle)

	// Generate all template files
	templateFiles := []struct {
		tmplContent string
		outputPath  string
	}{
		// Core files
		{templates.KubectlPluginMainTemplate, filepath.Join(pluginDir, "main.go")},
		{templates.KubectlPluginRootCmdTemplate, filepath.Join(pluginDir, "cmd", "root.go")},
		// Phase 1: Core Commands
		{templates.KubectlPluginStatusCmdTemplate, filepath.Join(pluginDir, "cmd", "status.go")},
		{templates.KubectlPluginGetCmdTemplate, filepath.Join(pluginDir, "cmd", "get.go")},
		{templates.KubectlPluginDescribeCmdTemplate, filepath.Join(pluginDir, "cmd", "describe.go")},
		// Phase 2: Diagnostic Commands
		{templates.KubectlPluginCompareCmdTemplate, filepath.Join(pluginDir, "cmd", "compare.go")},
		{templates.KubectlPluginDiagnoseCmdTemplate, filepath.Join(pluginDir, "cmd", "diagnose.go")},
		{templates.KubectlPluginDriftCmdTemplate, filepath.Join(pluginDir, "cmd", "drift.go")},
		// Phase 3: Interactive/Management Commands
		{templates.KubectlPluginCreateCmdTemplate, filepath.Join(pluginDir, "cmd", "create.go")},
		{templates.KubectlPluginQueryCmdTemplate, filepath.Join(pluginDir, "cmd", "query.go")},
		{templates.KubectlPluginActionCmdTemplate, filepath.Join(pluginDir, "cmd", "action.go")},
		{templates.KubectlPluginPatchCmdTemplate, filepath.Join(pluginDir, "cmd", "patch.go")},
		{templates.KubectlPluginPauseCmdTemplate, filepath.Join(pluginDir, "cmd", "pause.go")},
		{templates.KubectlPluginCleanupCmdTemplate, filepath.Join(pluginDir, "cmd", "cleanup.go")},
		{templates.KubectlPluginTargetingTemplate, filepath.Join(pluginDir, "cmd", "targeting.go")},
		// Rundeck Integration
		{templates.KubectlPluginNodesCmdTemplate, filepath.Join(pluginDir, "cmd", "nodes.go")},
		// Shared packages
		{templates.KubectlPluginClientTemplate, filepath.Join(pluginDir, "pkg", "client", "client.go")},
		{templates.KubectlPluginOutputTemplate, filepath.Join(pluginDir, "pkg", "output", "output.go")},
		// Build files
		{templates.KubectlPluginGoModTemplate, filepath.Join(pluginDir, "go.mod")},
		{templates.KubectlPluginMakefileTemplate, filepath.Join(pluginDir, "Makefile")},
	}

	funcMap := template.FuncMap{
		"lower":     strings.ToLower,
		"upper":     strings.ToUpper,
		"title":     strings.Title,
		"pluralize": pluralize,
	}

	for _, tf := range templateFiles {
		if err := g.executePluginTemplate(tf.tmplContent, data, tf.outputPath, funcMap); err != nil {
			return fmt.Errorf("failed to generate %s: %w", tf.outputPath, err)
		}
	}

	return nil
}

// prepareTemplateData prepares the template data from CRDs
func (g *KubectlPluginGenerator) prepareTemplateData(crds []*mapper.CRDDefinition, aggregate *mapper.AggregateDefinition, bundle *mapper.BundleDefinition) KubectlPluginTemplateData {
	// Extract API name from API group (e.g., "petstore.example.com" -> "petstore")
	apiName := strings.Split(g.config.APIGroup, ".")[0]

	// Build module name for the plugin
	pluginModuleName := fmt.Sprintf("%s/kubectl-plugin", g.config.ModuleName)

	data := KubectlPluginTemplateData{
		Year:             time.Now().Year(),
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         g.config.APIGroup,
		APIVersion:       g.config.APIVersion,
		APIName:          apiName,
		PluginName:       apiName,
		BinaryName:       "kubectl-" + apiName,
		ModuleName:       pluginModuleName,
		ResourceKinds:    make([]KindInfo, 0),
		QueryKinds:       make([]KindInfo, 0),
		ActionKinds:      make([]KindInfo, 0),
		AllKinds:         make([]KindInfo, 0),
	}

	// Categorize CRDs
	for _, crd := range crds {
		kindInfo := KindInfo{
			Kind:       crd.Kind,
			KindLower:  strings.ToLower(crd.Kind),
			Plural:     crd.Plural,
			ShortNames: []string{strings.ToLower(crd.Kind)},
		}

		data.AllKinds = append(data.AllKinds, kindInfo)

		if crd.IsQuery {
			data.QueryKinds = append(data.QueryKinds, kindInfo)
		} else if crd.IsAction {
			data.ActionKinds = append(data.ActionKinds, kindInfo)
		} else {
			data.ResourceKinds = append(data.ResourceKinds, kindInfo)
		}
	}

	// Add aggregate info
	if aggregate != nil {
		data.HasAggregate = true
		data.AggregateKind = aggregate.Kind
		aggregateInfo := KindInfo{
			Kind:       aggregate.Kind,
			KindLower:  strings.ToLower(aggregate.Kind),
			Plural:     aggregate.Plural,
			ShortNames: []string{strings.ToLower(aggregate.Kind[:3])}, // e.g., "sta" for StatusAggregate
		}
		data.AllKinds = append(data.AllKinds, aggregateInfo)
	}

	// Add bundle info
	if bundle != nil {
		data.HasBundle = true
		data.BundleKind = bundle.Kind
		bundleInfo := KindInfo{
			Kind:       bundle.Kind,
			KindLower:  strings.ToLower(bundle.Kind),
			Plural:     bundle.Plural,
			ShortNames: []string{strings.ToLower(bundle.Kind[:3])}, // e.g., "pet" for PetstoreBundle
		}
		data.AllKinds = append(data.AllKinds, bundleInfo)
	}

	return data
}

// executePluginTemplate executes a template and writes to output file
func (g *KubectlPluginGenerator) executePluginTemplate(tmplContent string, data interface{}, outputPath string, funcMap template.FuncMap) error {
	tmpl, err := template.New("kubectl-plugin").Funcs(funcMap).Parse(tmplContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}
