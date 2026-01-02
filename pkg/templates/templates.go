package templates

import (
	_ "embed"
)

// TypesTemplate is the template for generating CRD types
//
//go:embed types.go.tmpl
var TypesTemplate string

// GroupVersionInfoTemplate is the template for groupversion_info.go
//
//go:embed groupversion_info.go.tmpl
var GroupVersionInfoTemplate string

// ControllerTemplate is the template for generating controller reconciliation logic
//
//go:embed controller.go.tmpl
var ControllerTemplate string

// CRDYAMLTemplate is the template for generating CRD YAML manifests
//
//go:embed crd.yaml.tmpl
var CRDYAMLTemplate string

// EndpointResolverTemplate is the template for StatefulSet endpoint discovery
//
//go:embed endpoint_resolver.go.tmpl
var EndpointResolverTemplate string

// MainTemplate is the template for the main.go of generated operator
//
//go:embed main.go.tmpl
var MainTemplate string
