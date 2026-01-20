package bundle

import (
	"reflect"
	"sort"
	"testing"
)

func TestIsValidResourceID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"order-1", true},
		{"pet", true},
		{"my-resource-123", true},
		{"a", true},
		{"", false},
		{"Order", false},           // uppercase
		{"1order", false},          // starts with number
		{"-order", false},          // starts with hyphen
		{"order_1", false},         // underscore not allowed
		{"order.1", false},         // dot not allowed
		{"order 1", false},         // space not allowed
		{"order-with-CAPS", false}, // uppercase not allowed
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsValidResourceID(tt.input)
			if got != tt.want {
				t.Errorf("IsValidResourceID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsIdentChar(t *testing.T) {
	tests := []struct {
		input byte
		want  bool
	}{
		{'a', true},
		{'z', true},
		{'0', true},
		{'9', true},
		{'-', true},
		{'A', false},
		{'Z', false},
		{'_', false},
		{'.', false},
		{' ', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := IsIdentChar(tt.input)
			if got != tt.want {
				t.Errorf("IsIdentChar(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindResourceReferences(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		includeBare bool
		want        []string
	}{
		{
			name:        "no references",
			input:       "just a plain string",
			includeBare: true,
			want:        []string{},
		},
		{
			name:        "single ${} reference",
			input:       "child-of-${resources.parent.status.externalID}",
			includeBare: false,
			want:        []string{"parent"},
		},
		{
			name:        "multiple ${} references",
			input:       `{"name": "${resources.pet.status.name}", "id": "${resources.order.status.externalID}"}`,
			includeBare: false,
			want:        []string{"order", "pet"},
		},
		{
			name:        "bare reference without includeBare",
			input:       "resources.pet.status.state == 'Synced'",
			includeBare: false,
			want:        []string{},
		},
		{
			name:        "bare reference with includeBare",
			input:       "resources.pet.status.state == 'Synced'",
			includeBare: true,
			want:        []string{"pet"},
		},
		{
			name:        "mixed references",
			input:       `resources.a.status.ready && ${resources.b.status.externalID} != ""`,
			includeBare: true,
			want:        []string{"a", "b"},
		},
		{
			name:        "invalid resource ID",
			input:       "${resources.Invalid.status.field}",
			includeBare: false,
			want:        []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindResourceReferences(tt.input, tt.includeBare)

			// Convert to slice for comparison
			gotSlice := make([]string, 0, len(got))
			for k := range got {
				gotSlice = append(gotSlice, k)
			}
			sort.Strings(gotSlice)
			sort.Strings(tt.want)

			if !reflect.DeepEqual(gotSlice, tt.want) {
				t.Errorf("FindResourceReferences() = %v, want %v", gotSlice, tt.want)
			}
		})
	}
}

func TestExtractDependenciesFromMap(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		want []string
	}{
		{
			name: "no dependencies",
			data: map[string]interface{}{"name": "test"},
			want: []string{},
		},
		{
			name: "single dependency",
			data: map[string]interface{}{
				"name": "child-of-${resources.parent.status.externalID}",
			},
			want: []string{"parent"},
		},
		{
			name: "nested dependency",
			data: map[string]interface{}{
				"nested": map[string]interface{}{
					"ref": "${resources.inner.status.field}",
				},
			},
			want: []string{"inner"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractDependenciesFromMap(tt.data, false)
			sort.Strings(got)
			sort.Strings(tt.want)

			if len(got) != len(tt.want) {
				t.Errorf("ExtractDependenciesFromMap() = %v, want %v", got, tt.want)
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractDependenciesFromMap() = %v, want %v", got, tt.want)
					return
				}
			}
		})
	}
}

func TestExtractAllDependencies(t *testing.T) {
	tests := []struct {
		name string
		res  ResourceSpec
		opts DependencyExtractorOptions
		want []string
	}{
		{
			name: "explicit only",
			res: ResourceSpec{
				ID:        "child",
				DependsOn: []string{"parent"},
				Spec:      map[string]interface{}{"name": "${resources.other.status.id}"},
			},
			opts: DependencyExtractorOptions{IncludeExplicit: true},
			want: []string{"parent"},
		},
		{
			name: "spec refs only",
			res: ResourceSpec{
				ID:        "child",
				DependsOn: []string{"parent"},
				Spec:      map[string]interface{}{"name": "${resources.other.status.id}"},
			},
			opts: DependencyExtractorOptions{IncludeSpecRefs: true},
			want: []string{"other"},
		},
		{
			name: "all sources",
			res: ResourceSpec{
				ID:        "child",
				DependsOn: []string{"explicit"},
				Spec:      map[string]interface{}{"name": "${resources.spec-ref.status.id}"},
				ReadyWhen: []string{"resources.condition-ref.status.ready"},
			},
			opts: DefaultExtractorOptions(),
			want: []string{"condition-ref", "explicit", "spec-ref"},
		},
		{
			name: "removes self-reference",
			res: ResourceSpec{
				ID:        "self",
				DependsOn: []string{"self", "other"},
			},
			opts: DependencyExtractorOptions{IncludeExplicit: true},
			want: []string{"other"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractAllDependencies(tt.res, tt.opts)

			gotSlice := make([]string, 0, len(got))
			for k := range got {
				gotSlice = append(gotSlice, k)
			}
			sort.Strings(gotSlice)
			sort.Strings(tt.want)

			if !reflect.DeepEqual(gotSlice, tt.want) {
				t.Errorf("ExtractAllDependencies() = %v, want %v", gotSlice, tt.want)
			}
		})
	}
}

func TestBuildExecutionOrder(t *testing.T) {
	tests := []struct {
		name      string
		resources []ResourceSpec
		wantOrder []string
		wantErr   bool
	}{
		{
			name:      "empty resources",
			resources: []ResourceSpec{},
			wantOrder: []string{},
		},
		{
			name: "single resource",
			resources: []ResourceSpec{
				{ID: "order-1", Kind: "Order"},
			},
			wantOrder: []string{"order-1"},
		},
		{
			name: "no dependencies - alphabetical order",
			resources: []ResourceSpec{
				{ID: "pet-1", Kind: "Pet"},
				{ID: "order-1", Kind: "Order"},
			},
			wantOrder: []string{"order-1", "pet-1"},
		},
		{
			name: "simple dependency chain",
			resources: []ResourceSpec{
				{ID: "child", Kind: "Pet", DependsOn: []string{"parent"}},
				{ID: "parent", Kind: "Order"},
			},
			wantOrder: []string{"parent", "child"},
		},
		{
			name: "complex dependencies",
			resources: []ResourceSpec{
				{ID: "grandchild", Kind: "User", DependsOn: []string{"child"}},
				{ID: "child", Kind: "Pet", DependsOn: []string{"parent"}},
				{ID: "parent", Kind: "Order"},
			},
			wantOrder: []string{"parent", "child", "grandchild"},
		},
		{
			name: "circular dependency",
			resources: []ResourceSpec{
				{ID: "a", Kind: "Order", DependsOn: []string{"b"}},
				{ID: "b", Kind: "Pet", DependsOn: []string{"a"}},
			},
			wantErr: true,
		},
		{
			name: "unknown dependency",
			resources: []ResourceSpec{
				{ID: "a", Kind: "Order", DependsOn: []string{"nonexistent"}},
			},
			wantErr: true,
		},
		{
			name: "implicit dependency from spec",
			resources: []ResourceSpec{
				{ID: "child", Kind: "Pet", Spec: map[string]interface{}{
					"name": "child-of-${resources.parent.status.externalID}",
				}},
				{ID: "parent", Kind: "Order"},
			},
			wantOrder: []string{"parent", "child"},
		},
		{
			name: "diamond dependency",
			resources: []ResourceSpec{
				{ID: "d", Kind: "User", DependsOn: []string{"b", "c"}},
				{ID: "c", Kind: "Pet", DependsOn: []string{"a"}},
				{ID: "b", Kind: "Order", DependsOn: []string{"a"}},
				{ID: "a", Kind: "Order"},
			},
			wantOrder: []string{"a", "b", "c", "d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildExecutionOrderSimple(tt.resources)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildExecutionOrder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.wantOrder) {
				t.Errorf("BuildExecutionOrder() = %v, want %v", got, tt.wantOrder)
			}
		})
	}
}

func TestResolveExpressions(t *testing.T) {
	tests := []struct {
		name      string
		spec      map[string]interface{}
		statusMap map[string]map[string]interface{}
		wantKey   string
		wantValue interface{}
		wantErr   bool
	}{
		{
			name: "no expressions",
			spec: map[string]interface{}{"name": "test"},
			statusMap: map[string]map[string]interface{}{
				"parent": {"status": map[string]interface{}{"externalID": "123"}},
			},
			wantKey:   "name",
			wantValue: "test",
		},
		{
			name: "full expression replacement",
			spec: map[string]interface{}{"petId": "${resources.parent.status.externalID}"},
			statusMap: map[string]map[string]interface{}{
				"parent": {"status": map[string]interface{}{"externalID": "123"}},
			},
			wantKey:   "petId",
			wantValue: "123",
		},
		{
			name: "embedded expression",
			spec: map[string]interface{}{"name": "child-of-${resources.parent.status.externalID}"},
			statusMap: map[string]map[string]interface{}{
				"parent": {"status": map[string]interface{}{"externalID": "123"}},
			},
			wantKey:   "name",
			wantValue: "child-of-123",
		},
		{
			name: "nested map resolution",
			spec: map[string]interface{}{
				"nested": map[string]interface{}{
					"ref": "${resources.parent.status.externalID}",
				},
			},
			statusMap: map[string]map[string]interface{}{
				"parent": {"status": map[string]interface{}{"externalID": "456"}},
			},
			wantKey:   "nested",
			wantValue: map[string]interface{}{"ref": "456"},
		},
		{
			name: "missing resource",
			spec: map[string]interface{}{"petId": "${resources.missing.status.externalID}"},
			statusMap: map[string]map[string]interface{}{
				"parent": {"status": map[string]interface{}{"externalID": "123"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveExpressions(tt.spec, tt.statusMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveExpressions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got[tt.wantKey], tt.wantValue) {
				t.Errorf("ResolveExpressions()[%s] = %v, want %v", tt.wantKey, got[tt.wantKey], tt.wantValue)
			}
		})
	}
}

func TestNavigatePath(t *testing.T) {
	data := map[string]interface{}{
		"status": map[string]interface{}{
			"externalID": "123",
			"nested": map[string]interface{}{
				"deep": "value",
			},
		},
		"simple": "test",
	}

	tests := []struct {
		path string
		want interface{}
	}{
		{"simple", "test"},
		{"status.externalID", "123"},
		{"status.nested.deep", "value"},
		{"nonexistent", nil},
		{"status.nonexistent", nil},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := NavigatePath(data, tt.path)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NavigatePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestResourceStatusToMap(t *testing.T) {
	status := &ResourceStatus{
		ID:         "test",
		Kind:       "Order",
		Name:       "test-order",
		State:      "Synced",
		ExternalID: "123",
		Message:    "OK",
		Ready:      true,
		Skipped:    false,
	}

	got := status.ToMap()

	statusMap, ok := got["status"].(map[string]interface{})
	if !ok {
		t.Fatal("status field not found or not a map")
	}

	if statusMap["state"] != "Synced" {
		t.Errorf("status.state = %v, want Synced", statusMap["state"])
	}
	if statusMap["externalID"] != "123" {
		t.Errorf("status.externalID = %v, want 123", statusMap["externalID"])
	}
	if statusMap["ready"] != true {
		t.Errorf("status.ready = %v, want true", statusMap["ready"])
	}
}
