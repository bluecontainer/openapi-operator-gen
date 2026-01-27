package generator

import (
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

// SamplesGenerator generates example CR YAML files
type SamplesGenerator struct {
	config *config.Config
}

// NewSamplesGenerator creates a new samples generator
func NewSamplesGenerator(cfg *config.Config) *SamplesGenerator {
	return &SamplesGenerator{config: cfg}
}

// ExampleCRData holds data for example CR template
type ExampleCRData struct {
	GeneratorVersion string
	APIGroup         string
	APIVersion       string
	Kind             string
	KindLower        string
	IsQuery          bool
	IsAction         bool
	SpecFields       []ExampleFieldData
}

// ExampleFieldData holds field data for example CR
type ExampleFieldData struct {
	JSONName       string
	ExampleValue   string
	AdoptValue     string // Value used in adopt-and-modify samples (shows modification)
	IsTargeting    bool
	IsBinaryData   bool // True if this is a binary data field (shown as comments in samples)
	IncludeInAdopt bool // True if this field should be included in adopt samples (ID or modified field)
}

// ExampleAggregateCRData holds data for example aggregate CR template
type ExampleAggregateCRData struct {
	GeneratorVersion string
	APIGroup         string
	APIVersion       string
	Kind             string
	KindLower        string
	ResourceKinds    []string // CRUD resource kinds
	QueryKinds       []string // Query CRD kinds
	ActionKinds      []string // Action CRD kinds
	AllKinds         []string // All kinds combined
}

// ExampleBundleCRData holds data for example bundle CR template
type ExampleBundleCRData struct {
	GeneratorVersion string
	APIGroup         string
	APIVersion       string
	Kind             string
	KindLower        string
	ResourceKinds    []string                      // CRUD resource kinds
	QueryKinds       []string                      // Query CRD kinds
	ActionKinds      []string                      // Action CRD kinds
	AllKinds         []string                      // All kinds combined
	ResourceSpecs    map[string][]ExampleFieldData // Map of Kind -> spec fields with example values
}

// Generate generates example CR YAML files for all CRDs
// aggregate and bundle are optional - pass nil if not generating those CRDs
func (g *SamplesGenerator) Generate(crds []*mapper.CRDDefinition, aggregate *mapper.AggregateDefinition, bundle *mapper.BundleDefinition) error {
	// Create samples directory
	samplesDir := filepath.Join(g.config.OutputDir, "config", "samples")
	if err := os.MkdirAll(samplesDir, 0755); err != nil {
		return fmt.Errorf("failed to create samples directory: %w", err)
	}

	sampleFiles := make([]string, 0, len(crds)*2)

	for _, crd := range crds {
		// Generate basic example CR
		if err := g.generateExampleCR(samplesDir, crd); err != nil {
			return fmt.Errorf("failed to generate example CR for %s: %w", crd.Kind, err)
		}
		sampleFiles = append(sampleFiles, fmt.Sprintf("%s_%s.yaml", g.config.APIVersion, strings.ToLower(crd.Kind)))

		// Generate example CR with externalIDRef (only for resource CRDs that need it)
		// ExternalIDRef is only needed when there are no path parameters to identify the resource
		if !crd.IsQuery && !crd.IsAction && crd.NeedsExternalIDRef {
			if err := g.generateExampleCRRef(samplesDir, crd); err != nil {
				return fmt.Errorf("failed to generate example CR ref for %s: %w", crd.Kind, err)
			}
			sampleFiles = append(sampleFiles, fmt.Sprintf("%s_%s_ref.yaml", g.config.APIVersion, strings.ToLower(crd.Kind)))
		}

		// Generate adopt-and-modify example CR (only for resource CRDs that can update)
		// This demonstrates adopting an existing resource and modifying a field
		canUpdate := crd.HasPut || crd.HasPatch || crd.UpdateWithPost
		if !crd.IsQuery && !crd.IsAction && canUpdate {
			if err := g.generateExampleCRAdopt(samplesDir, crd); err != nil {
				return fmt.Errorf("failed to generate example CR adopt for %s: %w", crd.Kind, err)
			}
			sampleFiles = append(sampleFiles, fmt.Sprintf("%s_%s_adopt.yaml", g.config.APIVersion, strings.ToLower(crd.Kind)))
		}
	}

	// Generate aggregate sample if aggregate CRD is enabled
	if aggregate != nil {
		if err := g.generateExampleAggregateCR(samplesDir, aggregate); err != nil {
			return fmt.Errorf("failed to generate example aggregate CR: %w", err)
		}
		sampleFiles = append(sampleFiles, fmt.Sprintf("%s_%s.yaml", g.config.APIVersion, strings.ToLower(aggregate.Kind)))
	}

	// Generate bundle sample if bundle CRD is enabled
	if bundle != nil {
		if err := g.generateExampleBundleCR(samplesDir, bundle, crds); err != nil {
			return fmt.Errorf("failed to generate example bundle CR: %w", err)
		}
		sampleFiles = append(sampleFiles, fmt.Sprintf("%s_%s.yaml", g.config.APIVersion, strings.ToLower(bundle.Kind)))
	}

	// Generate kustomization.yaml for samples
	if err := g.generateSamplesKustomization(samplesDir, sampleFiles); err != nil {
		return fmt.Errorf("failed to generate samples kustomization.yaml: %w", err)
	}

	return nil
}

func (g *SamplesGenerator) generateExampleCR(samplesDir string, crd *mapper.CRDDefinition) error {
	data := ExampleCRData{
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         crd.APIGroup,
		APIVersion:       crd.APIVersion,
		Kind:             crd.Kind,
		KindLower:        strings.ToLower(crd.Kind),
		IsQuery:          crd.IsQuery,
		IsAction:         crd.IsAction,
		SpecFields:       g.convertToExampleFields(crd.Spec),
	}

	tmpl, err := template.New("example").Parse(templates.ExampleCRTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.yaml", g.config.APIVersion, strings.ToLower(crd.Kind))
	examplePath := filepath.Join(samplesDir, filename)

	file, err := os.Create(examplePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func (g *SamplesGenerator) generateExampleCRRef(samplesDir string, crd *mapper.CRDDefinition) error {
	data := ExampleCRData{
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         crd.APIGroup,
		APIVersion:       crd.APIVersion,
		Kind:             crd.Kind,
		KindLower:        strings.ToLower(crd.Kind),
		IsQuery:          crd.IsQuery,
		IsAction:         crd.IsAction,
	}

	tmpl, err := template.New("example-ref").Parse(templates.ExampleCRRefTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	filename := fmt.Sprintf("%s_%s_ref.yaml", g.config.APIVersion, strings.ToLower(crd.Kind))
	examplePath := filepath.Join(samplesDir, filename)

	file, err := os.Create(examplePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// generateExampleCRAdopt generates an example CR that demonstrates adopting
// an existing external resource and modifying one of its fields.
// This sample uses onDelete: Restore to show that the original state will be
// restored when the CR is deleted.
func (g *SamplesGenerator) generateExampleCRAdopt(samplesDir string, crd *mapper.CRDDefinition) error {
	data := ExampleCRData{
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         crd.APIGroup,
		APIVersion:       crd.APIVersion,
		Kind:             crd.Kind,
		KindLower:        strings.ToLower(crd.Kind),
		IsQuery:          crd.IsQuery,
		IsAction:         crd.IsAction,
		SpecFields:       g.convertToAdoptFields(crd.Spec),
	}

	tmpl, err := template.New("example-adopt").Parse(templates.ExampleCRAdoptTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	filename := fmt.Sprintf("%s_%s_adopt.yaml", g.config.APIVersion, strings.ToLower(crd.Kind))
	examplePath := filepath.Join(samplesDir, filename)

	file, err := os.Create(examplePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func (g *SamplesGenerator) generateSamplesKustomization(samplesDir string, sampleFiles []string) error {
	data := struct {
		GeneratorVersion string
		SampleFiles      []string
	}{
		GeneratorVersion: g.config.GeneratorVersion,
		SampleFiles:      sampleFiles,
	}

	tmpl, err := template.New("samples-kustomization").Parse(templates.KustomizationSamplesTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	kustomizationPath := filepath.Join(samplesDir, "kustomization.yaml")
	file, err := os.Create(kustomizationPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func (g *SamplesGenerator) convertToExampleFields(spec *mapper.FieldDefinition) []ExampleFieldData {
	if spec == nil {
		return nil
	}

	result := make([]ExampleFieldData, 0, len(spec.Fields))
	for _, f := range spec.Fields {
		// Skip targeting fields - they'll be shown as comments
		isTargeting := g.isTargetingField(f.JSONName)
		isBinaryData := g.isBinaryDataField(f.JSONName)

		// Generate appropriate example value
		exampleValue := g.generateExampleValue(f)
		if isBinaryData {
			exampleValue = g.generateBinaryDataExampleValue(f.JSONName)
		}

		result = append(result, ExampleFieldData{
			JSONName:     f.JSONName,
			ExampleValue: exampleValue,
			IsTargeting:  isTargeting,
			IsBinaryData: isBinaryData,
		})
	}
	return result
}

// convertToAdoptFields generates fields for adopt-and-modify samples.
// Only includes the ID field (to identify the resource) and the modified field.
func (g *SamplesGenerator) convertToAdoptFields(spec *mapper.FieldDefinition) []ExampleFieldData {
	if spec == nil {
		return nil
	}

	result := make([]ExampleFieldData, 0)

	// Find a good field to modify (prefer name, status, or description)
	modifyField := g.selectFieldToModify(spec)

	for _, f := range spec.Fields {
		isTargeting := g.isTargetingField(f.JSONName)
		exampleVal := g.generateExampleValue(f)

		// Determine if this field should be included in the adopt sample
		// Include: ID field (for identifying the resource) or the modified field
		isIDField := f.JSONName == "id" || strings.HasSuffix(strings.ToLower(f.JSONName), "id")
		isModifiedField := f.JSONName == modifyField
		includeInAdopt := (isIDField || isModifiedField) && !isTargeting

		// Generate adopt value - for the selected field, show a modified value
		adoptVal := exampleVal
		if isModifiedField {
			adoptVal = g.generateModifiedValue(f, exampleVal)
		}

		result = append(result, ExampleFieldData{
			JSONName:       f.JSONName,
			ExampleValue:   exampleVal,
			AdoptValue:     adoptVal,
			IsTargeting:    isTargeting,
			IncludeInAdopt: includeInAdopt,
		})
	}
	return result
}

// selectFieldToModify picks a good field to modify for demonstration purposes.
// Prefers descriptive fields over IDs.
func (g *SamplesGenerator) selectFieldToModify(spec *mapper.FieldDefinition) string {
	// Priority order for fields to modify
	priorityFields := []string{"name", "status", "description", "quantity", "complete", "userStatus"}

	for _, priority := range priorityFields {
		for _, f := range spec.Fields {
			if f.JSONName == priority && !g.isTargetingField(f.JSONName) {
				return f.JSONName
			}
		}
	}

	// Fallback: pick first non-ID, non-targeting string or bool field
	for _, f := range spec.Fields {
		if g.isTargetingField(f.JSONName) {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.JSONName), "id") {
			continue
		}
		goType := strings.TrimPrefix(f.GoType, "*")
		if goType == "string" || goType == "bool" || goType == "int" || goType == "int32" || goType == "int64" {
			return f.JSONName
		}
	}

	return ""
}

// generateModifiedValue generates a modified version of the example value
// to show that the field has been changed from its original state.
func (g *SamplesGenerator) generateModifiedValue(f *mapper.FieldDefinition, originalValue string) string {
	goType := strings.TrimPrefix(f.GoType, "*")
	lowerName := strings.ToLower(f.JSONName)

	switch goType {
	case "string":
		// For enum fields with known values, pick a different valid value
		if len(f.Enum) > 1 {
			// Return second enum value (different from default)
			return fmt.Sprintf("%q", f.Enum[1])
		}
		// For name fields, add a suffix
		if strings.Contains(lowerName, "name") {
			// Remove quotes and add suffix
			if strings.HasPrefix(originalValue, `"`) && strings.HasSuffix(originalValue, `"`) {
				name := originalValue[1 : len(originalValue)-1]
				return fmt.Sprintf("%q", name+" (Modified)")
			}
		}
		// For status fields, change to a different common status
		if lowerName == "status" {
			return `"pending"`
		}
		// Generic modification
		if strings.HasPrefix(originalValue, `"`) && strings.HasSuffix(originalValue, `"`) {
			val := originalValue[1 : len(originalValue)-1]
			return fmt.Sprintf("%q", val+"-modified")
		}
		return originalValue + "-modified"

	case "bool":
		// Toggle boolean value
		if originalValue == "true" {
			return "false"
		}
		return "true"

	case "int", "int32", "int64":
		// For quantity/count fields, double the value
		if strings.Contains(lowerName, "quantity") || strings.Contains(lowerName, "count") {
			return "4" // Double the typical quantity of 2
		}
		// For status codes, change to a different value
		if strings.Contains(lowerName, "status") {
			return "2" // Changed from typical status of 1
		}
		return "99" // Generic different value

	default:
		return originalValue
	}
}

func (g *SamplesGenerator) isTargetingField(jsonName string) bool {
	targetingFields := map[string]bool{
		"targetNamespace":   true,
		"targetStatefulSet": true,
		"targetDeployment":  true,
		"targetHelmRelease": true,
		"targetPodOrdinal":  true,
	}
	return targetingFields[jsonName]
}

// isBinaryDataField returns true if the field is a binary data source field
// These are mutually exclusive options shown as comments in samples
func (g *SamplesGenerator) isBinaryDataField(jsonName string) bool {
	binaryFields := map[string]bool{
		"data":           true,
		"dataFrom":       true,
		"dataURL":        true,
		"dataFromVolume": true,
		"contentType":    true,
	}
	return binaryFields[jsonName]
}

// generateBinaryDataExampleValue generates meaningful example values for binary data fields
func (g *SamplesGenerator) generateBinaryDataExampleValue(jsonName string) string {
	switch jsonName {
	case "data":
		// Base64-encoded example (small PNG header as example)
		return `"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="`
	case "dataFrom":
		// ConfigMap reference example
		return `{"configMapRef": {"name": "my-image-data", "key": "image.png"}}`
	case "dataURL":
		return `"https://example.com/images/photo.png"`
	case "dataFromVolume":
		return `{"claimName": "my-pvc", "path": "/data/image.png"}`
	case "contentType":
		return `"image/png"`
	default:
		return `""`
	}
}

// exampleValueMap provides realistic, connected example values for common field names.
// These values are designed to work together across different resource types.
// For example, petId: 10 in Order corresponds to the Pet with id: 10.
var exampleValueMap = map[string]string{
	// Pet-related fields
	"name":      `"Fluffy"`,
	"petId":     "10",
	"photoUrls": `["https://example.com/photos/fluffy.jpg"]`,
	"status":    `"available"`, // Pet status
	"tags":      `[{"id": 1, "name": "friendly"}]`,
	"category":  `{"id": 1, "name": "Dogs"}`,

	// Order-related fields
	"orderId":  "100",
	"quantity": "2",
	"shipDate": "", // Will be generated dynamically
	"complete": "false",

	// User-related fields
	"username":   `"john_doe"`,
	"firstName":  `"John"`,
	"lastName":   `"Doe"`,
	"email":      `"john.doe@example.com"`,
	"password":   `"********"`,
	"phone":      `"+1-555-123-4567"`,
	"userStatus": "1",

	// Query parameters
	"statusValues": `["available", "pending"]`,
	"tagsValues":   `["friendly", "trained"]`,
}

// getConnectedExampleValue returns a connected example value for the given field name,
// or empty string if no specific value is defined.
func getConnectedExampleValue(jsonName string) string {
	// Check for exact match first
	if val, ok := exampleValueMap[jsonName]; ok {
		return val
	}
	return ""
}

func (g *SamplesGenerator) generateExampleValue(f *mapper.FieldDefinition) string {
	// If there's an enum, use the first value
	if len(f.Enum) > 0 {
		return fmt.Sprintf("%q", f.Enum[0])
	}

	// Check for connected example value first
	if connectedVal := getConnectedExampleValue(f.JSONName); connectedVal != "" {
		return connectedVal
	}

	// Remove pointer prefix
	goType := strings.TrimPrefix(f.GoType, "*")

	switch goType {
	case "string":
		// Generate more meaningful example values based on field name patterns
		return g.generateStringExampleValue(f.JSONName)
	case "int", "int32", "int64":
		// Use field name to generate meaningful IDs
		return g.generateIntExampleValue(f.JSONName)
	case "float32", "float64":
		return "1.0"
	case "bool":
		return "true"
	case "[]string":
		return g.generateStringArrayExampleValue(f.JSONName)
	case "[]int", "[]int32", "[]int64":
		return "[1, 2, 3]"
	case "metav1.Time":
		// Generate a valid RFC 3339 timestamp for date-time fields
		return fmt.Sprintf("%q", time.Now().UTC().Format(time.RFC3339))
	case "metav1.Duration":
		// Generate a meaningful duration value
		return g.generateDurationExampleValue(f.JSONName)
	default:
		if strings.HasPrefix(goType, "[]") {
			return "[]"
		}
		if strings.HasPrefix(goType, "map[") {
			return "{}"
		}
		// For complex types, return empty object
		return "{}"
	}
}

// generateStringExampleValue generates a meaningful string value based on field name
func (g *SamplesGenerator) generateStringExampleValue(jsonName string) string {
	lowerName := strings.ToLower(jsonName)

	// Common patterns
	switch {
	case strings.Contains(lowerName, "name"):
		return `"Example Name"`
	case strings.Contains(lowerName, "email"):
		return `"user@example.com"`
	case strings.Contains(lowerName, "phone"):
		return `"+1-555-000-0000"`
	case strings.Contains(lowerName, "url") || strings.Contains(lowerName, "uri"):
		return `"https://example.com/resource"`
	case strings.Contains(lowerName, "description"):
		return `"A sample description"`
	case strings.Contains(lowerName, "password"):
		return `"********"`
	case strings.Contains(lowerName, "status"):
		return `"active"`
	case strings.Contains(lowerName, "type"):
		return `"default"`
	default:
		return fmt.Sprintf("%q", "example-"+jsonName)
	}
}

// generateIntExampleValue generates a meaningful integer value based on field name
func (g *SamplesGenerator) generateIntExampleValue(jsonName string) string {
	lowerName := strings.ToLower(jsonName)

	// Use consistent IDs for related resources
	switch {
	case lowerName == "id":
		return "10" // Default ID
	case strings.HasSuffix(lowerName, "id"):
		// For foreign keys like petId, orderId, userId
		return "10"
	case strings.Contains(lowerName, "quantity") || strings.Contains(lowerName, "count"):
		return "2"
	case strings.Contains(lowerName, "status"):
		return "1"
	case strings.Contains(lowerName, "ordinal") || strings.Contains(lowerName, "index"):
		return "0"
	default:
		return "1"
	}
}

// generateStringArrayExampleValue generates a meaningful string array value based on field name
func (g *SamplesGenerator) generateStringArrayExampleValue(jsonName string) string {
	lowerName := strings.ToLower(jsonName)

	switch {
	case strings.Contains(lowerName, "url") || strings.Contains(lowerName, "photo"):
		return `["https://example.com/images/1.jpg", "https://example.com/images/2.jpg"]`
	case strings.Contains(lowerName, "tag"):
		return `["tag1", "tag2"]`
	case strings.Contains(lowerName, "status"):
		return `["available", "pending"]`
	default:
		return `["item1", "item2"]`
	}
}

// generateDurationExampleValue generates a meaningful duration value based on field name
func (g *SamplesGenerator) generateDurationExampleValue(jsonName string) string {
	lowerName := strings.ToLower(jsonName)

	switch {
	case strings.Contains(lowerName, "interval"):
		return `"5m"` // 5 minutes is a reasonable re-execution interval
	case strings.Contains(lowerName, "timeout"):
		return `"30s"`
	case strings.Contains(lowerName, "delay"):
		return `"10s"`
	default:
		return `"1m"`
	}
}

// sortKindsByDependency sorts resource kinds so that parent resources come before
// children. A child is detected by having foreign key fields (e.g., petId, orderId)
// that reference other resource kinds.
// The sort prioritizes: 1) Referenced parents (kinds that others depend on)
// 2) Unreferenced kinds 3) Children (kinds with foreign keys)
func (g *SamplesGenerator) sortKindsByDependency(kinds []string, resourceSpecs map[string][]ExampleFieldData) []string {
	if len(kinds) <= 1 {
		return kinds
	}

	// Build a set of known kind names (lowercase) for matching foreign keys
	kindSet := make(map[string]string) // lowercase -> original case
	for _, kind := range kinds {
		kindSet[strings.ToLower(kind)] = kind
	}

	// Detect which kinds have foreign key references to other kinds
	// A foreign key field is one ending in "Id" (but not just "id") where the prefix matches a kind
	childToParent := make(map[string][]string) // child kind -> list of parent kinds it references
	referencedBy := make(map[string][]string)  // parent kind -> list of children that reference it

	for _, kind := range kinds {
		specs := resourceSpecs[kind]
		for _, field := range specs {
			if g.isTargetingField(field.JSONName) {
				continue
			}
			lowerName := strings.ToLower(field.JSONName)
			// Check if field ends with "id" and is not just "id"
			if lowerName != "id" && strings.HasSuffix(lowerName, "id") {
				// Extract the prefix (e.g., "petId" -> "pet", "orderId" -> "order")
				prefix := lowerName[:len(lowerName)-2]
				// Check if this prefix matches a known kind
				if parentKind, ok := kindSet[prefix]; ok && parentKind != kind {
					childToParent[kind] = append(childToParent[kind], parentKind)
					referencedBy[parentKind] = append(referencedBy[parentKind], kind)
				}
			}
		}
	}

	// Sort: 1) Referenced parents (kinds that others depend on) first
	// 2) Children (kinds with foreign keys) second - they show the relationship
	// 3) Unreferenced kinds last (standalone resources)
	var referencedParents, children, standalone []string
	for _, kind := range kinds {
		isChild := len(childToParent[kind]) > 0
		isReferenced := len(referencedBy[kind]) > 0

		if isReferenced && !isChild {
			referencedParents = append(referencedParents, kind)
		} else if isChild {
			children = append(children, kind)
		} else {
			standalone = append(standalone, kind)
		}
	}

	// Return: referenced parents, then children, then standalone
	result := make([]string, 0, len(kinds))
	result = append(result, referencedParents...)
	result = append(result, children...)
	result = append(result, standalone...)
	return result
}

func (g *SamplesGenerator) generateExampleAggregateCR(samplesDir string, aggregate *mapper.AggregateDefinition) error {
	data := ExampleAggregateCRData{
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         aggregate.APIGroup,
		APIVersion:       aggregate.APIVersion,
		Kind:             aggregate.Kind,
		KindLower:        strings.ToLower(aggregate.Kind),
		ResourceKinds:    aggregate.ResourceKinds,
		QueryKinds:       aggregate.QueryKinds,
		ActionKinds:      aggregate.ActionKinds,
		AllKinds:         aggregate.AllKinds,
	}

	tmpl, err := template.New("example-aggregate").Funcs(template.FuncMap{
		"lower": strings.ToLower,
	}).Parse(templates.ExampleAggregateCRTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.yaml", g.config.APIVersion, strings.ToLower(aggregate.Kind))
	examplePath := filepath.Join(samplesDir, filename)

	file, err := os.Create(examplePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func (g *SamplesGenerator) generateExampleBundleCR(samplesDir string, bundle *mapper.BundleDefinition, crds []*mapper.CRDDefinition) error {
	// Build map of Kind -> spec fields
	resourceSpecs := make(map[string][]ExampleFieldData)
	for _, crd := range crds {
		resourceSpecs[crd.Kind] = g.convertToExampleFields(crd.Spec)
	}

	// Sort ResourceKinds by dependency order (parents before children)
	// A "child" has foreign key fields (like petId, orderId) referencing other resources
	sortedResourceKinds := g.sortKindsByDependency(bundle.ResourceKinds, resourceSpecs)

	data := ExampleBundleCRData{
		GeneratorVersion: g.config.GeneratorVersion,
		APIGroup:         bundle.APIGroup,
		APIVersion:       bundle.APIVersion,
		Kind:             bundle.Kind,
		KindLower:        strings.ToLower(bundle.Kind),
		ResourceKinds:    sortedResourceKinds,
		QueryKinds:       bundle.QueryKinds,
		ActionKinds:      bundle.ActionKinds,
		AllKinds:         bundle.AllKinds,
		ResourceSpecs:    resourceSpecs,
	}

	tmpl, err := template.New("example-bundle").Funcs(template.FuncMap{
		"lower": strings.ToLower,
		"getSpec": func(kind string) []ExampleFieldData {
			return resourceSpecs[kind]
		},
		"isPrimaryID": func(jsonName string) bool {
			// Only the primary ID field (assigned by external API)
			return jsonName == "id"
		},
		"isParentIDField": func(jsonName string) bool {
			// Fields like petId, orderId, userId that reference parent resources
			lowerName := strings.ToLower(jsonName)
			return jsonName != "id" && strings.HasSuffix(lowerName, "id")
		},
	}).Parse(templates.ExampleBundleCRTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.yaml", g.config.APIVersion, strings.ToLower(bundle.Kind))
	examplePath := filepath.Join(samplesDir, filename)

	file, err := os.Create(examplePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
