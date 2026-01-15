package templates

import (
	"bytes"
	"strings"
	"testing"
	"text/template"
)

// =============================================================================
// Template Loading Tests - Verify templates are embedded correctly
// =============================================================================

func TestTypesTemplateLoaded(t *testing.T) {
	if TypesTemplate == "" {
		t.Error("TypesTemplate is empty - embed may have failed")
	}
	if !strings.Contains(TypesTemplate, "package") {
		t.Error("TypesTemplate doesn't contain expected 'package' keyword")
	}
}

func TestGroupVersionInfoTemplateLoaded(t *testing.T) {
	if GroupVersionInfoTemplate == "" {
		t.Error("GroupVersionInfoTemplate is empty - embed may have failed")
	}
	if !strings.Contains(GroupVersionInfoTemplate, "SchemeBuilder") {
		t.Error("GroupVersionInfoTemplate doesn't contain expected 'SchemeBuilder' keyword")
	}
}

func TestControllerTemplateLoaded(t *testing.T) {
	if ControllerTemplate == "" {
		t.Error("ControllerTemplate is empty - embed may have failed")
	}
	if !strings.Contains(ControllerTemplate, "Reconcile") {
		t.Error("ControllerTemplate doesn't contain expected 'Reconcile' keyword")
	}
}

func TestQueryControllerTemplateLoaded(t *testing.T) {
	if QueryControllerTemplate == "" {
		t.Error("QueryControllerTemplate is empty - embed may have failed")
	}
	if !strings.Contains(QueryControllerTemplate, "Reconcile") {
		t.Error("QueryControllerTemplate doesn't contain expected 'Reconcile' keyword")
	}
	if !strings.Contains(QueryControllerTemplate, "executeQuery") {
		t.Error("QueryControllerTemplate doesn't contain expected 'executeQuery' keyword")
	}
}

func TestActionControllerTemplateLoaded(t *testing.T) {
	if ActionControllerTemplate == "" {
		t.Error("ActionControllerTemplate is empty - embed may have failed")
	}
	if !strings.Contains(ActionControllerTemplate, "Reconcile") {
		t.Error("ActionControllerTemplate doesn't contain expected 'Reconcile' keyword")
	}
	if !strings.Contains(ActionControllerTemplate, "executeAction") {
		t.Error("ActionControllerTemplate doesn't contain expected 'executeAction' keyword")
	}
}

func TestCRDYAMLTemplateLoaded(t *testing.T) {
	if CRDYAMLTemplate == "" {
		t.Error("CRDYAMLTemplate is empty - embed may have failed")
	}
	if !strings.Contains(CRDYAMLTemplate, "apiVersion") {
		t.Error("CRDYAMLTemplate doesn't contain expected 'apiVersion' keyword")
	}
	if !strings.Contains(CRDYAMLTemplate, "CustomResourceDefinition") {
		t.Error("CRDYAMLTemplate doesn't contain expected 'CustomResourceDefinition' keyword")
	}
}

func TestMainTemplateLoaded(t *testing.T) {
	if MainTemplate == "" {
		t.Error("MainTemplate is empty - embed may have failed")
	}
	if !strings.Contains(MainTemplate, "func main()") {
		t.Error("MainTemplate doesn't contain expected 'func main()' keyword")
	}
}

// =============================================================================
// Template Parsing Tests - Verify templates are valid Go templates
// =============================================================================

func TestTypesTemplateParseable(t *testing.T) {
	_, err := template.New("types").Parse(TypesTemplate)
	if err != nil {
		t.Errorf("Failed to parse TypesTemplate: %v", err)
	}
}

func TestGroupVersionInfoTemplateParseable(t *testing.T) {
	_, err := template.New("groupversion").Parse(GroupVersionInfoTemplate)
	if err != nil {
		t.Errorf("Failed to parse GroupVersionInfoTemplate: %v", err)
	}
}

func TestControllerTemplateParseable(t *testing.T) {
	_, err := template.New("controller").Parse(ControllerTemplate)
	if err != nil {
		t.Errorf("Failed to parse ControllerTemplate: %v", err)
	}
}

func TestQueryControllerTemplateParseable(t *testing.T) {
	_, err := template.New("querycontroller").Parse(QueryControllerTemplate)
	if err != nil {
		t.Errorf("Failed to parse QueryControllerTemplate: %v", err)
	}
}

func TestActionControllerTemplateParseable(t *testing.T) {
	_, err := template.New("actioncontroller").Parse(ActionControllerTemplate)
	if err != nil {
		t.Errorf("Failed to parse ActionControllerTemplate: %v", err)
	}
}

func TestCRDYAMLTemplateParseable(t *testing.T) {
	_, err := template.New("crdyaml").Parse(CRDYAMLTemplate)
	if err != nil {
		t.Errorf("Failed to parse CRDYAMLTemplate: %v", err)
	}
}

func TestMainTemplateParseable(t *testing.T) {
	_, err := template.New("main").Parse(MainTemplate)
	if err != nil {
		t.Errorf("Failed to parse MainTemplate: %v", err)
	}
}

// =============================================================================
// Template Execution Tests - Verify templates produce valid output with sample data
// =============================================================================

// FieldData mimics the field data structure used in templates
type FieldData struct {
	Name        string
	JSONName    string
	GoType      string
	Description string
	Required    bool
	Validation  *ValidationData
	Enum        []string
}

// ValidationData mimics validation rules
type ValidationData struct {
	MinLength *int64
	MaxLength *int64
	Minimum   *float64
	Maximum   *float64
	Pattern   string
	Enum      []string
	MinItems  *int64
	MaxItems  *int64
}

// SpecData mimics spec structure
type SpecData struct {
	Fields []FieldData
}

// CRDTypeData mimics CRD type data for types template
type CRDTypeData struct {
	Kind            string
	Plural          string
	ShortNames      []string
	Spec            *SpecData
	IsQuery         bool
	QueryPath       string
	ResponseType    string
	ResponseIsArray bool
	ResultItemType  string
	ResultFields    []FieldData
	UsesSharedType  bool

	// Action endpoint fields
	IsAction       bool
	ActionPath     string
	ActionMethod   string
	ParentResource string
	ParentIDParam  string
	ActionName     string

	// HTTP method availability
	HasDelete bool
	HasPost   bool

	// ExternalIDRef handling
	NeedsExternalIDRef bool
}

// NestedTypeData mimics nested type data
type NestedTypeData struct {
	Name   string
	Fields []FieldData
}

// TypesTemplateData mimics the data structure for types template
type TypesTemplateData struct {
	Year             int
	GeneratorVersion string
	APIVersion       string
	APIGroup         string
	ModuleName       string
	CRDs             []CRDTypeData
	NestedTypes      []NestedTypeData
}

func TestTypesTemplateExecution(t *testing.T) {
	tmpl, err := template.New("types").Parse(TypesTemplate)
	if err != nil {
		t.Fatalf("Failed to parse TypesTemplate: %v", err)
	}

	data := TypesTemplateData{
		Year:             2024,
		GeneratorVersion: "v0.0.1",
		APIVersion:       "v1alpha1",
		APIGroup:         "example.com",
		ModuleName:       "github.com/example/operator",
		CRDs: []CRDTypeData{
			{
				Kind:       "Pet",
				Plural:     "pets",
				ShortNames: []string{"pt"},
				Spec: &SpecData{
					Fields: []FieldData{
						{
							Name:        "Name",
							JSONName:    "name",
							GoType:      "string",
							Description: "Name of the pet",
							Required:    true,
						},
						{
							Name:        "Age",
							JSONName:    "age",
							GoType:      "*int32",
							Description: "Age of the pet",
							Required:    false,
						},
					},
				},
				IsQuery:   false,
				HasDelete: true,
				HasPost:   true,
			},
		},
		NestedTypes: []NestedTypeData{},
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		t.Errorf("Failed to execute TypesTemplate: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "package v1alpha1") {
		t.Error("Output doesn't contain expected package declaration")
	}
	if !strings.Contains(output, "PetSpec") {
		t.Error("Output doesn't contain expected PetSpec type")
	}
	if !strings.Contains(output, "PetStatus") {
		t.Error("Output doesn't contain expected PetStatus type")
	}
}

func TestTypesTemplateQueryCRDExecution(t *testing.T) {
	tmpl, err := template.New("types").Parse(TypesTemplate)
	if err != nil {
		t.Fatalf("Failed to parse TypesTemplate: %v", err)
	}

	data := TypesTemplateData{
		Year:             2024,
		GeneratorVersion: "v0.0.1",
		APIVersion:       "v1alpha1",
		APIGroup:         "example.com",
		ModuleName:       "github.com/example/operator",
		CRDs: []CRDTypeData{
			{
				Kind:            "PetFindByTags",
				Plural:          "petfindbytags",
				ShortNames:      []string{"pfbt"},
				IsQuery:         true,
				QueryPath:       "/pet/findByTags",
				ResponseType:    "[]Pet",
				ResponseIsArray: true,
				ResultItemType:  "Pet",
				UsesSharedType:  true,
				Spec: &SpecData{
					Fields: []FieldData{
						{
							Name:        "Tags",
							JSONName:    "tags",
							GoType:      "[]string",
							Description: "Tags to filter by",
							Required:    false,
						},
					},
				},
			},
		},
		NestedTypes: []NestedTypeData{},
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		t.Errorf("Failed to execute TypesTemplate for query CRD: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "PetFindByTagsSpec") {
		t.Error("Output doesn't contain expected PetFindByTagsSpec type")
	}
	if !strings.Contains(output, "PetFindByTagsStatus") {
		t.Error("Output doesn't contain expected PetFindByTagsStatus type")
	}
	if !strings.Contains(output, "ResultCount") {
		t.Error("Output doesn't contain expected ResultCount field for query CRD")
	}
}

// GroupVersionInfoData mimics the data structure for groupversion_info template
type GroupVersionInfoData struct {
	Year             int
	GeneratorVersion string
	APIVersion       string
	APIGroup         string
	GroupName        string
}

func TestGroupVersionInfoTemplateExecution(t *testing.T) {
	tmpl, err := template.New("groupversion").Parse(GroupVersionInfoTemplate)
	if err != nil {
		t.Fatalf("Failed to parse GroupVersionInfoTemplate: %v", err)
	}

	data := GroupVersionInfoData{
		Year:             2024,
		GeneratorVersion: "v0.0.1",
		APIVersion:       "v1alpha1",
		APIGroup:         "petstore.example.com",
		GroupName:        "petstore",
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		t.Errorf("Failed to execute GroupVersionInfoTemplate: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "package v1alpha1") {
		t.Error("Output doesn't contain expected package declaration")
	}
	if !strings.Contains(output, "petstore.example.com") {
		t.Error("Output doesn't contain expected API group")
	}
	if !strings.Contains(output, "SchemeBuilder") {
		t.Error("Output doesn't contain expected SchemeBuilder")
	}
}

// ActionPathParam for action controller templates
type ActionPathParam struct {
	Name   string
	GoName string
}

// ActionRequestBodyField for action controller templates
type ActionRequestBodyField struct {
	JSONName string
	GoName   string
}

type ResourceQueryParam struct {
	Name     string // Parameter name as it appears in URL (e.g., "status")
	JSONName string // JSON field name (e.g., "status")
	GoName   string // Go field name (e.g., "Status")
	GoType   string // Go type (e.g., "string", "int64")
	IsArray  bool   // True if this is an array parameter
}

// QueryParamField represents a query/path parameter for query controllers
type QueryParamField struct {
	Name        string // Go field name (e.g., "ServiceName")
	JSONName    string // JSON field name (e.g., "serviceName")
	GoType      string // Go type (e.g., "string", "int64")
	Description string
	Required    bool
	IsArray     bool   // True if this is an array parameter
	ItemType    string // Type of array items if IsArray is true
}

// ControllerTemplateData mimics the data structure for controller template
type ControllerTemplateData struct {
	Year               int
	GeneratorVersion   string
	APIGroup           string
	APIVersion         string
	ModuleName         string
	Kind               string
	KindLower          string
	Plural             string
	BasePath           string
	ResourcePath       string
	IsQuery            bool
	QueryPath          string
	QueryPathParams    []QueryParamField // Path parameters for query endpoints
	QueryParams        []QueryParamField // Query parameters for query endpoints
	ResponseType       string
	ResponseIsArray    bool
	ResultItemType     string
	HasTypedResults    bool
	UsesSharedType     bool
	IsPrimitiveArray   bool
	PrimitiveArrayType string

	// Action endpoint fields
	IsAction            bool
	ActionPath          string
	ActionMethod        string
	ParentResource      string
	ParentIDParam       string
	ParentIDField       string
	HasParentID         bool
	ActionName          string
	PathParams          []ActionPathParam
	RequestBodyFields   []ActionRequestBodyField
	HasRequestBody      bool
	ResourcePathParams  []ActionPathParam
	ResourceQueryParams []ResourceQueryParam
	HasResourceParams   bool

	// HTTP method availability
	HasDelete bool
	HasPost   bool
	HasPut    bool

	// UpdateWithPost enables using POST for updates when PUT is not available
	UpdateWithPost bool

	// Per-method paths (when different methods use different paths)
	GetPath        string
	PutPath        string
	DeletePath     string
	PutPathDiffers bool

	// ExternalIDRef handling
	NeedsExternalIDRef bool
}

func TestControllerTemplateExecution(t *testing.T) {
	tmpl, err := template.New("controller").Parse(ControllerTemplate)
	if err != nil {
		t.Fatalf("Failed to parse ControllerTemplate: %v", err)
	}

	data := ControllerTemplateData{
		Year:              2024,
		GeneratorVersion:  "v0.0.1",
		APIGroup:          "petstore.example.com",
		APIVersion:        "v1alpha1",
		ModuleName:        "github.com/example/petstore-operator",
		Kind:              "Pet",
		KindLower:         "pet",
		Plural:            "pets",
		BasePath:          "/pet",
		IsQuery:           false,
		HasResourceParams: false,
		HasDelete:         true,
		HasPost:           true,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		t.Errorf("Failed to execute ControllerTemplate: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "package controller") {
		t.Error("Output doesn't contain expected package declaration")
	}
	if !strings.Contains(output, "PetReconciler") {
		t.Error("Output doesn't contain expected PetReconciler type")
	}
	if !strings.Contains(output, "func (r *PetReconciler) Reconcile") {
		t.Error("Output doesn't contain expected Reconcile function")
	}
	if !strings.Contains(output, "petFinalizer") {
		t.Error("Output doesn't contain expected finalizer constant")
	}
}

func TestQueryControllerTemplateExecution(t *testing.T) {
	tmpl, err := template.New("querycontroller").Parse(QueryControllerTemplate)
	if err != nil {
		t.Fatalf("Failed to parse QueryControllerTemplate: %v", err)
	}

	data := ControllerTemplateData{
		Year:             2024,
		GeneratorVersion: "v0.0.1",
		APIGroup:         "petstore.example.com",
		APIVersion:       "v1alpha1",
		ModuleName:       "github.com/example/petstore-operator",
		Kind:             "PetFindByTags",
		KindLower:        "petfindbytags",
		Plural:           "petfindbytags",
		BasePath:         "/pet",
		IsQuery:          true,
		QueryPath:        "/pet/findByTags",
		ResponseType:     "[]Pet",
		ResponseIsArray:  true,
		ResultItemType:   "Pet",
		HasTypedResults:  true,
		UsesSharedType:   true,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		t.Errorf("Failed to execute QueryControllerTemplate: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "package controller") {
		t.Error("Output doesn't contain expected package declaration")
	}
	if !strings.Contains(output, "PetFindByTagsReconciler") {
		t.Error("Output doesn't contain expected PetFindByTagsReconciler type")
	}
	if !strings.Contains(output, "executeQuery") {
		t.Error("Output doesn't contain expected executeQuery function")
	}
	if !strings.Contains(output, "parseResults") {
		t.Error("Output doesn't contain expected parseResults function for typed results")
	}
}

func TestQueryControllerTemplateWithoutTypedResults(t *testing.T) {
	tmpl, err := template.New("querycontroller").Parse(QueryControllerTemplate)
	if err != nil {
		t.Fatalf("Failed to parse QueryControllerTemplate: %v", err)
	}

	data := ControllerTemplateData{
		Year:             2024,
		GeneratorVersion: "v0.0.1",
		APIGroup:         "petstore.example.com",
		APIVersion:       "v1alpha1",
		ModuleName:       "github.com/example/petstore-operator",
		Kind:             "SearchQuery",
		KindLower:        "searchquery",
		Plural:           "searchqueries",
		BasePath:         "/search",
		IsQuery:          true,
		QueryPath:        "/search",
		HasTypedResults:  false,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		t.Errorf("Failed to execute QueryControllerTemplate without typed results: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "SearchQueryReconciler") {
		t.Error("Output doesn't contain expected SearchQueryReconciler type")
	}
	if !strings.Contains(output, "countResults") {
		t.Error("Output doesn't contain expected countResults function for untyped results")
	}
}

func TestActionControllerTemplateExecution(t *testing.T) {
	tmpl, err := template.New("actioncontroller").Parse(ActionControllerTemplate)
	if err != nil {
		t.Fatalf("Failed to parse ActionControllerTemplate: %v", err)
	}

	data := ControllerTemplateData{
		Year:             2024,
		GeneratorVersion: "v0.0.1",
		APIGroup:         "petstore.example.com",
		APIVersion:       "v1alpha1",
		ModuleName:       "github.com/example/petstore-operator",
		Kind:             "PetUploadImage",
		KindLower:        "petuploadimage",
		Plural:           "petuploadimages",
		BasePath:         "/pet",
		IsAction:         true,
		ActionPath:       "/pet/{petId}/uploadImage",
		ActionMethod:     "POST",
		ParentResource:   "Pet",
		ParentIDParam:    "petId",
		ParentIDField:    "PetId",
		HasParentID:      true,
		ActionName:       "uploadImage",
		HasRequestBody:   true,
		RequestBodyFields: []ActionRequestBodyField{
			{JSONName: "additionalMetadata", GoName: "AdditionalMetadata"},
			{JSONName: "file", GoName: "File"},
		},
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		t.Errorf("Failed to execute ActionControllerTemplate: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "package controller") {
		t.Error("Output doesn't contain expected package declaration")
	}
	if !strings.Contains(output, "PetUploadImageReconciler") {
		t.Error("Output doesn't contain expected PetUploadImageReconciler type")
	}
	if !strings.Contains(output, "executeAction") {
		t.Error("Output doesn't contain expected executeAction function")
	}
	if !strings.Contains(output, "buildActionURL") {
		t.Error("Output doesn't contain expected buildActionURL function")
	}
	if !strings.Contains(output, "buildRequestBody") {
		t.Error("Output doesn't contain expected buildRequestBody function")
	}
}

func TestActionControllerTemplateWithTypedResults(t *testing.T) {
	tmpl, err := template.New("actioncontroller").Parse(ActionControllerTemplate)
	if err != nil {
		t.Fatalf("Failed to parse ActionControllerTemplate: %v", err)
	}

	data := ControllerTemplateData{
		Year:             2024,
		GeneratorVersion: "v0.0.1",
		APIGroup:         "petstore.example.com",
		APIVersion:       "v1alpha1",
		ModuleName:       "github.com/example/petstore-operator",
		Kind:             "PetUploadImage",
		KindLower:        "petuploadimage",
		Plural:           "petuploadimages",
		BasePath:         "/pet",
		IsAction:         true,
		ActionPath:       "/pet/{petId}/uploadImage",
		ActionMethod:     "POST",
		ParentResource:   "Pet",
		ParentIDParam:    "petId",
		ParentIDField:    "PetId",
		HasParentID:      true,
		ActionName:       "uploadImage",
		HasRequestBody:   true,
		HasTypedResults:  true,
		ResponseIsArray:  false,
		ResultItemType:   "PetUploadImageResult",
		RequestBodyFields: []ActionRequestBodyField{
			{JSONName: "additionalMetadata", GoName: "AdditionalMetadata"},
		},
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		t.Errorf("Failed to execute ActionControllerTemplate with typed results: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "parseResult") {
		t.Error("Output doesn't contain expected parseResult function for typed results")
	}
	if !strings.Contains(output, "v1alpha1.PetUploadImageResult") {
		t.Error("Output doesn't contain expected package-qualified result type")
	}
	if !strings.Contains(output, "var firstResult *v1alpha1.PetUploadImageResult") {
		t.Error("Output doesn't contain expected firstResult declaration with package prefix")
	}
}

func TestActionControllerTemplateWithArrayTypedResults(t *testing.T) {
	tmpl, err := template.New("actioncontroller").Parse(ActionControllerTemplate)
	if err != nil {
		t.Fatalf("Failed to parse ActionControllerTemplate: %v", err)
	}

	data := ControllerTemplateData{
		Year:             2024,
		GeneratorVersion: "v0.0.1",
		APIGroup:         "petstore.example.com",
		APIVersion:       "v1alpha1",
		ModuleName:       "github.com/example/petstore-operator",
		Kind:             "PetBatchUpdate",
		KindLower:        "petbatchupdate",
		Plural:           "petbatchupdates",
		BasePath:         "/pet",
		IsAction:         true,
		ActionPath:       "/pet/batch",
		ActionMethod:     "POST",
		ParentResource:   "Pet",
		ParentIDParam:    "",
		ParentIDField:    "",
		HasParentID:      false,
		ActionName:       "batch",
		HasRequestBody:   false,
		HasTypedResults:  true,
		ResponseIsArray:  true,
		ResultItemType:   "PetBatchUpdateResult",
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		t.Errorf("Failed to execute ActionControllerTemplate with array typed results: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "parseResult") {
		t.Error("Output doesn't contain expected parseResult function for typed results")
	}
	if !strings.Contains(output, "[]v1alpha1.PetBatchUpdateResult") {
		t.Error("Output doesn't contain expected array result type with package prefix")
	}
	if !strings.Contains(output, "var firstResult []v1alpha1.PetBatchUpdateResult") {
		t.Error("Output doesn't contain expected firstResult array declaration with package prefix")
	}
}

func TestTypesTemplateActionCRDExecution(t *testing.T) {
	tmpl, err := template.New("types").Parse(TypesTemplate)
	if err != nil {
		t.Fatalf("Failed to parse TypesTemplate: %v", err)
	}

	data := TypesTemplateData{
		Year:             2024,
		GeneratorVersion: "v0.0.1",
		APIVersion:       "v1alpha1",
		APIGroup:         "example.com",
		ModuleName:       "github.com/example/operator",
		CRDs: []CRDTypeData{
			{
				Kind:           "PetUploadImage",
				Plural:         "petuploadimages",
				ShortNames:     []string{"pui"},
				IsAction:       true,
				ActionPath:     "/pet/{petId}/uploadImage",
				ActionMethod:   "POST",
				ParentResource: "Pet",
				ParentIDParam:  "petId",
				ActionName:     "uploadImage",
				Spec: &SpecData{
					Fields: []FieldData{
						{
							Name:        "PetId",
							JSONName:    "petId",
							GoType:      "string",
							Description: "ID of pet to update",
							Required:    true,
						},
						{
							Name:        "AdditionalMetadata",
							JSONName:    "additionalMetadata",
							GoType:      "string",
							Description: "Additional data to pass to server",
							Required:    false,
						},
					},
				},
			},
		},
		NestedTypes: []NestedTypeData{},
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		t.Errorf("Failed to execute TypesTemplate for action CRD: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "PetUploadImageSpec") {
		t.Error("Output doesn't contain expected PetUploadImageSpec type")
	}
	if !strings.Contains(output, "PetUploadImageStatus") {
		t.Error("Output doesn't contain expected PetUploadImageStatus type")
	}
	if !strings.Contains(output, "ExecutedAt") {
		t.Error("Output doesn't contain expected ExecutedAt field for action CRD")
	}
	if !strings.Contains(output, "CompletedAt") {
		t.Error("Output doesn't contain expected CompletedAt field for action CRD")
	}
	if !strings.Contains(output, "HTTPStatusCode") {
		t.Error("Output doesn't contain expected HTTPStatusCode field for action CRD")
	}
}

// CRDYAMLFieldData mimics field data for CRD YAML template
type CRDYAMLFieldData struct {
	Name        string
	JSONName    string
	GoType      string
	SchemaType  string
	Description string
	Required    bool
	Enum        []string
}

// CRDYAMLSpecData mimics spec data for CRD YAML template
type CRDYAMLSpecData struct {
	Fields []CRDYAMLFieldData
}

// CRDYAMLData mimics the data structure for CRD YAML template
type CRDYAMLData struct {
	GeneratorVersion string
	APIGroup         string
	APIVersion       string
	Kind             string
	KindLower        string
	Plural           string
	Singular         string
	ShortNames       []string
	Scope            string
	Spec             *CRDYAMLSpecData
}

func TestCRDYAMLTemplateExecution(t *testing.T) {
	tmpl, err := template.New("crdyaml").Parse(CRDYAMLTemplate)
	if err != nil {
		t.Fatalf("Failed to parse CRDYAMLTemplate: %v", err)
	}

	data := CRDYAMLData{
		GeneratorVersion: "v0.0.1",
		APIGroup:         "petstore.example.com",
		APIVersion:       "v1alpha1",
		Kind:             "Pet",
		KindLower:        "pet",
		Plural:           "pets",
		Singular:         "pet",
		ShortNames:       []string{"pt", "pet"},
		Scope:            "Namespaced",
		Spec: &CRDYAMLSpecData{
			Fields: []CRDYAMLFieldData{
				{
					Name:        "Name",
					JSONName:    "name",
					GoType:      "string",
					SchemaType:  "string",
					Description: "Name of the pet",
					Required:    true,
				},
			},
		},
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		t.Errorf("Failed to execute CRDYAMLTemplate: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "apiVersion: apiextensions.k8s.io/v1") {
		t.Error("Output doesn't contain expected apiVersion")
	}
	if !strings.Contains(output, "kind: CustomResourceDefinition") {
		t.Error("Output doesn't contain expected kind")
	}
	if !strings.Contains(output, "petstore.example.com") {
		t.Error("Output doesn't contain expected API group")
	}
}

// MainTemplateData mimics the data structure for main template
type CRDMainData struct {
	Kind     string
	IsQuery  bool
	IsAction bool
}

type MainTemplateData struct {
	Year             int
	GeneratorVersion string
	APIVersion       string
	ModuleName       string
	AppName          string
	CRDs             []CRDMainData
	HasAggregate     bool
	AggregateKind    string
	HasBundle        bool
	BundleKind       string
}

func TestMainTemplateExecution(t *testing.T) {
	tmpl, err := template.New("main").Parse(MainTemplate)
	if err != nil {
		t.Fatalf("Failed to parse MainTemplate: %v", err)
	}

	data := MainTemplateData{
		Year:             2024,
		GeneratorVersion: "v0.0.1",
		APIVersion:       "v1alpha1",
		ModuleName:       "github.com/example/petstore-operator",
		AppName:          "petstore",
		CRDs: []CRDMainData{
			{Kind: "Pet", IsQuery: false},
			{Kind: "User", IsQuery: false},
			{Kind: "PetFindByTags", IsQuery: true},
		},
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		t.Errorf("Failed to execute MainTemplate: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "package main") {
		t.Error("Output doesn't contain expected package declaration")
	}
	if !strings.Contains(output, "func main()") {
		t.Error("Output doesn't contain expected main function")
	}
	if !strings.Contains(output, "PetReconciler") {
		t.Error("Output doesn't contain expected PetReconciler setup")
	}
	if !strings.Contains(output, "UserReconciler") {
		t.Error("Output doesn't contain expected UserReconciler setup")
	}
	if !strings.Contains(output, "PetFindByTagsReconciler") {
		t.Error("Output doesn't contain expected PetFindByTagsReconciler setup")
	}
}

func TestMainTemplateWithSingleCRD(t *testing.T) {
	tmpl, err := template.New("main").Parse(MainTemplate)
	if err != nil {
		t.Fatalf("Failed to parse MainTemplate: %v", err)
	}

	data := MainTemplateData{
		Year:             2024,
		GeneratorVersion: "v0.0.1",
		APIVersion:       "v1alpha1",
		ModuleName:       "github.com/example/simple-operator",
		AppName:          "simple",
		CRDs: []CRDMainData{
			{Kind: "Resource", IsQuery: false},
		},
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		t.Errorf("Failed to execute MainTemplate with single CRD: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "ResourceReconciler") {
		t.Error("Output doesn't contain expected ResourceReconciler setup")
	}
}

// =============================================================================
// Template Content Validation Tests
// =============================================================================

func TestTypesTemplateContainsRequiredImports(t *testing.T) {
	requiredImports := []string{
		"metav1",
		"k8s.io/apimachinery",
	}

	for _, imp := range requiredImports {
		if !strings.Contains(TypesTemplate, imp) {
			t.Errorf("TypesTemplate missing required import: %s", imp)
		}
	}
}

func TestControllerTemplateContainsRequiredImports(t *testing.T) {
	requiredImports := []string{
		"context",
		"controller-runtime",
		"k8s.io/apimachinery",
	}

	for _, imp := range requiredImports {
		if !strings.Contains(ControllerTemplate, imp) {
			t.Errorf("ControllerTemplate missing required import: %s", imp)
		}
	}
}

func TestQueryControllerTemplateContainsRequiredImports(t *testing.T) {
	requiredImports := []string{
		"context",
		"net/http",
		"controller-runtime",
	}

	for _, imp := range requiredImports {
		if !strings.Contains(QueryControllerTemplate, imp) {
			t.Errorf("QueryControllerTemplate missing required import: %s", imp)
		}
	}
}

func TestTypesTemplateContainsKubebuilderMarkers(t *testing.T) {
	markers := []string{
		"+kubebuilder:object:root=true",
		"+kubebuilder:subresource:status",
	}

	for _, marker := range markers {
		if !strings.Contains(TypesTemplate, marker) {
			t.Errorf("TypesTemplate missing kubebuilder marker: %s", marker)
		}
	}
}

func TestControllerTemplateContainsRBACMarkers(t *testing.T) {
	if !strings.Contains(ControllerTemplate, "+kubebuilder:rbac") {
		t.Error("ControllerTemplate missing RBAC markers")
	}
}

func TestQueryControllerTemplateContainsRBACMarkers(t *testing.T) {
	if !strings.Contains(QueryControllerTemplate, "+kubebuilder:rbac") {
		t.Error("QueryControllerTemplate missing RBAC markers")
	}
}

func TestActionControllerTemplateContainsRBACMarkers(t *testing.T) {
	if !strings.Contains(ActionControllerTemplate, "+kubebuilder:rbac") {
		t.Error("ActionControllerTemplate missing RBAC markers")
	}
}

func TestActionControllerTemplateContainsRequiredImports(t *testing.T) {
	requiredImports := []string{
		"context",
		"net/http",
		"controller-runtime",
		"encoding/json",
	}

	for _, imp := range requiredImports {
		if !strings.Contains(ActionControllerTemplate, imp) {
			t.Errorf("ActionControllerTemplate missing required import: %s", imp)
		}
	}
}

// =============================================================================
// Template Placeholder Tests - Ensure all expected placeholders exist
// =============================================================================

func TestTypesTemplatePlaceholders(t *testing.T) {
	placeholders := []string{
		"{{ .Year }}",
		"{{ .APIVersion }}",
		"range .CRDs", // uses range syntax
	}

	for _, ph := range placeholders {
		if !strings.Contains(TypesTemplate, ph) {
			t.Errorf("TypesTemplate missing placeholder: %s", ph)
		}
	}
}

func TestControllerTemplatePlaceholders(t *testing.T) {
	placeholders := []string{
		"{{ .Year }}",
		"{{ .Kind }}",
		"{{ .KindLower }}",
		"{{ .APIVersion }}",
		"{{ .APIGroup }}",
		"{{ .ModuleName }}",
	}

	for _, ph := range placeholders {
		if !strings.Contains(ControllerTemplate, ph) {
			t.Errorf("ControllerTemplate missing placeholder: %s", ph)
		}
	}
}

func TestQueryControllerTemplatePlaceholders(t *testing.T) {
	placeholders := []string{
		"{{ .Year }}",
		"{{ .Kind }}",
		"{{ .KindLower }}",
		"{{ .QueryPath }}",
		"{{ .APIVersion }}",
	}

	for _, ph := range placeholders {
		if !strings.Contains(QueryControllerTemplate, ph) {
			t.Errorf("QueryControllerTemplate missing placeholder: %s", ph)
		}
	}
}

func TestActionControllerTemplatePlaceholders(t *testing.T) {
	placeholders := []string{
		"{{ .Year }}",
		"{{ .Kind }}",
		"{{ .KindLower }}",
		"{{ .ActionPath }}",
		"{{ .ActionMethod }}",
		"{{ .APIVersion }}",
	}

	for _, ph := range placeholders {
		if !strings.Contains(ActionControllerTemplate, ph) {
			t.Errorf("ActionControllerTemplate missing placeholder: %s", ph)
		}
	}
}

func TestMainTemplatePlaceholders(t *testing.T) {
	placeholders := []string{
		"{{ .Year }}",
		"{{ .APIVersion }}",
		"{{ .ModuleName }}",
		"range .CRDs", // uses range syntax
	}

	for _, ph := range placeholders {
		if !strings.Contains(MainTemplate, ph) {
			t.Errorf("MainTemplate missing placeholder: %s", ph)
		}
	}
}
