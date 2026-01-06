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

// QueryControllerTemplate is the template for generating query-only controller reconciliation logic
//
//go:embed query_controller.go.tmpl
var QueryControllerTemplate string

// ActionControllerTemplate is the template for generating action controller reconciliation logic
//
//go:embed action_controller.go.tmpl
var ActionControllerTemplate string

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

// UtilsTemplate is the template for utility functions shared across controllers
//
//go:embed utils.go.tmpl
var UtilsTemplate string
