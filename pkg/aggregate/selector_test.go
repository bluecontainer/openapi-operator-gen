package aggregate

import (
	"testing"
)

func TestCompileSelector(t *testing.T) {
	tests := []struct {
		name    string
		sel     ResourceSelector
		wantErr bool
	}{
		{
			name: "valid selector with kind only",
			sel: ResourceSelector{
				Kind: "Order",
			},
			wantErr: false,
		},
		{
			name: "valid selector with labels",
			sel: ResourceSelector{
				Kind:        "Pet",
				MatchLabels: map[string]string{"env": "prod"},
			},
			wantErr: false,
		},
		{
			name: "valid selector with name pattern",
			sel: ResourceSelector{
				Kind:        "User",
				NamePattern: "^test-.*",
			},
			wantErr: false,
		},
		{
			name: "valid selector with all fields",
			sel: ResourceSelector{
				Kind:        "Order",
				MatchLabels: map[string]string{"tier": "backend"},
				NamePattern: "order-[0-9]+",
			},
			wantErr: false,
		},
		{
			name: "invalid name pattern regex",
			sel: ResourceSelector{
				Kind:        "Order",
				NamePattern: "[invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := CompileSelector(tt.sel)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompileSelector() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && compiled == nil {
				t.Error("CompileSelector() returned nil for valid selector")
			}
		})
	}
}

func TestCompiledSelector_Matches(t *testing.T) {
	tests := []struct {
		name           string
		sel            ResourceSelector
		kind           string
		resourceName   string
		resourceLabels map[string]string
		want           bool
	}{
		{
			name: "matches kind only",
			sel: ResourceSelector{
				Kind: "Order",
			},
			kind:           "Order",
			resourceName:   "any-name",
			resourceLabels: nil,
			want:           true,
		},
		{
			name: "does not match different kind",
			sel: ResourceSelector{
				Kind: "Order",
			},
			kind:           "Pet",
			resourceName:   "any-name",
			resourceLabels: nil,
			want:           false,
		},
		{
			name: "matches labels",
			sel: ResourceSelector{
				Kind:        "Order",
				MatchLabels: map[string]string{"env": "prod"},
			},
			kind:           "Order",
			resourceName:   "order-1",
			resourceLabels: map[string]string{"env": "prod", "tier": "backend"},
			want:           true,
		},
		{
			name: "does not match missing label",
			sel: ResourceSelector{
				Kind:        "Order",
				MatchLabels: map[string]string{"env": "prod"},
			},
			kind:           "Order",
			resourceName:   "order-1",
			resourceLabels: map[string]string{"tier": "backend"},
			want:           false,
		},
		{
			name: "does not match wrong label value",
			sel: ResourceSelector{
				Kind:        "Order",
				MatchLabels: map[string]string{"env": "prod"},
			},
			kind:           "Order",
			resourceName:   "order-1",
			resourceLabels: map[string]string{"env": "dev"},
			want:           false,
		},
		{
			name: "matches name pattern",
			sel: ResourceSelector{
				Kind:        "Order",
				NamePattern: "^test-.*",
			},
			kind:           "Order",
			resourceName:   "test-order-1",
			resourceLabels: nil,
			want:           true,
		},
		{
			name: "does not match name pattern",
			sel: ResourceSelector{
				Kind:        "Order",
				NamePattern: "^test-.*",
			},
			kind:           "Order",
			resourceName:   "prod-order-1",
			resourceLabels: nil,
			want:           false,
		},
		{
			name: "matches all criteria",
			sel: ResourceSelector{
				Kind:        "Order",
				MatchLabels: map[string]string{"env": "test"},
				NamePattern: "^order-[0-9]+$",
			},
			kind:           "Order",
			resourceName:   "order-123",
			resourceLabels: map[string]string{"env": "test"},
			want:           true,
		},
		{
			name: "fails if any criteria not met - wrong kind",
			sel: ResourceSelector{
				Kind:        "Order",
				MatchLabels: map[string]string{"env": "test"},
				NamePattern: "^order-[0-9]+$",
			},
			kind:           "Pet",
			resourceName:   "order-123",
			resourceLabels: map[string]string{"env": "test"},
			want:           false,
		},
		{
			name: "fails if any criteria not met - wrong label",
			sel: ResourceSelector{
				Kind:        "Order",
				MatchLabels: map[string]string{"env": "test"},
				NamePattern: "^order-[0-9]+$",
			},
			kind:           "Order",
			resourceName:   "order-123",
			resourceLabels: map[string]string{"env": "prod"},
			want:           false,
		},
		{
			name: "fails if any criteria not met - wrong name",
			sel: ResourceSelector{
				Kind:        "Order",
				MatchLabels: map[string]string{"env": "test"},
				NamePattern: "^order-[0-9]+$",
			},
			kind:           "Order",
			resourceName:   "order-abc",
			resourceLabels: map[string]string{"env": "test"},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := CompileSelector(tt.sel)
			if err != nil {
				t.Fatalf("CompileSelector() error = %v", err)
			}

			got := compiled.Matches(tt.kind, tt.resourceName, tt.resourceLabels)
			if got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseResourceReference(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		want ResourceReference
	}{
		{
			name: "full reference",
			m: map[string]interface{}{
				"kind":      "Order",
				"name":      "my-order",
				"namespace": "production",
			},
			want: ResourceReference{
				Kind:      "Order",
				Name:      "my-order",
				Namespace: "production",
			},
		},
		{
			name: "reference without namespace",
			m: map[string]interface{}{
				"kind": "Pet",
				"name": "fluffy",
			},
			want: ResourceReference{
				Kind:      "Pet",
				Name:      "fluffy",
				Namespace: "",
			},
		},
		{
			name: "empty map",
			m:    map[string]interface{}{},
			want: ResourceReference{},
		},
		{
			name: "wrong types ignored",
			m: map[string]interface{}{
				"kind": 123,
				"name": true,
			},
			want: ResourceReference{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseResourceReference(tt.m)
			if got != tt.want {
				t.Errorf("ParseResourceReference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseResourceSelector(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		want ResourceSelector
	}{
		{
			name: "kind only",
			m: map[string]interface{}{
				"kind": "Order",
			},
			want: ResourceSelector{
				Kind: "Order",
			},
		},
		{
			name: "with match labels",
			m: map[string]interface{}{
				"kind": "Pet",
				"matchLabels": map[string]interface{}{
					"env":  "prod",
					"tier": "backend",
				},
			},
			want: ResourceSelector{
				Kind: "Pet",
				MatchLabels: map[string]string{
					"env":  "prod",
					"tier": "backend",
				},
			},
		},
		{
			name: "with name pattern",
			m: map[string]interface{}{
				"kind":        "User",
				"namePattern": "^test-.*",
			},
			want: ResourceSelector{
				Kind:        "User",
				NamePattern: "^test-.*",
			},
		},
		{
			name: "all fields",
			m: map[string]interface{}{
				"kind": "Order",
				"matchLabels": map[string]interface{}{
					"env": "test",
				},
				"namePattern": "^order-[0-9]+$",
			},
			want: ResourceSelector{
				Kind:        "Order",
				MatchLabels: map[string]string{"env": "test"},
				NamePattern: "^order-[0-9]+$",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseResourceSelector(tt.m)
			if got.Kind != tt.want.Kind {
				t.Errorf("Kind = %v, want %v", got.Kind, tt.want.Kind)
			}
			if got.NamePattern != tt.want.NamePattern {
				t.Errorf("NamePattern = %v, want %v", got.NamePattern, tt.want.NamePattern)
			}
			if len(got.MatchLabels) != len(tt.want.MatchLabels) {
				t.Errorf("MatchLabels length = %v, want %v", len(got.MatchLabels), len(tt.want.MatchLabels))
			}
			for k, v := range tt.want.MatchLabels {
				if got.MatchLabels[k] != v {
					t.Errorf("MatchLabels[%s] = %v, want %v", k, got.MatchLabels[k], v)
				}
			}
		})
	}
}

func TestResourceReference_IsValid(t *testing.T) {
	tests := []struct {
		name string
		ref  ResourceReference
		want bool
	}{
		{
			name: "valid with all fields",
			ref:  ResourceReference{Kind: "Order", Name: "my-order", Namespace: "default"},
			want: true,
		},
		{
			name: "valid without namespace",
			ref:  ResourceReference{Kind: "Order", Name: "my-order"},
			want: true,
		},
		{
			name: "invalid - missing kind",
			ref:  ResourceReference{Name: "my-order"},
			want: false,
		},
		{
			name: "invalid - missing name",
			ref:  ResourceReference{Kind: "Order"},
			want: false,
		},
		{
			name: "invalid - empty",
			ref:  ResourceReference{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceSelector_IsValid(t *testing.T) {
	tests := []struct {
		name string
		sel  ResourceSelector
		want bool
	}{
		{
			name: "valid with kind only",
			sel:  ResourceSelector{Kind: "Order"},
			want: true,
		},
		{
			name: "valid with all fields",
			sel: ResourceSelector{
				Kind:        "Order",
				MatchLabels: map[string]string{"env": "prod"},
				NamePattern: "^test-.*",
			},
			want: true,
		},
		{
			name: "invalid - missing kind",
			sel:  ResourceSelector{MatchLabels: map[string]string{"env": "prod"}},
			want: false,
		},
		{
			name: "invalid - empty",
			sel:  ResourceSelector{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sel.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		defaultNs string
		want      string
	}{
		{
			name:      "uses provided namespace",
			namespace: "production",
			defaultNs: "staging",
			want:      "production",
		},
		{
			name:      "uses default when namespace empty",
			namespace: "",
			defaultNs: "staging",
			want:      "staging",
		},
		{
			name:      "uses 'default' when both empty",
			namespace: "",
			defaultNs: "",
			want:      "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultNamespace(tt.namespace, tt.defaultNs); got != tt.want {
				t.Errorf("DefaultNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceKey(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		namespace string
		resName   string
		want      string
	}{
		{
			name:      "standard key",
			kind:      "Order",
			namespace: "default",
			resName:   "my-order",
			want:      "Order/default/my-order",
		},
		{
			name:      "different namespace",
			kind:      "Pet",
			namespace: "production",
			resName:   "fluffy",
			want:      "Pet/production/fluffy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResourceKey(tt.kind, tt.namespace, tt.resName); got != tt.want {
				t.Errorf("ResourceKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKindToVariableName(t *testing.T) {
	tests := []struct {
		kind string
		want string
	}{
		{"Order", "orders"},
		{"Pet", "pets"},
		{"User", "users"},
		{"PetFindbystatusQuery", "petfindbystatusquerys"},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			if got := KindToVariableName(tt.kind); got != tt.want {
				t.Errorf("KindToVariableName(%q) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

func TestKindToResourceName(t *testing.T) {
	tests := []struct {
		kind string
		want string
	}{
		{"Order", "orders"},
		{"Pet", "pets"},
		{"User", "users"},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			if got := KindToResourceName(tt.kind); got != tt.want {
				t.Errorf("KindToResourceName(%q) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

func TestCompiledSelector_LabelSelectorString(t *testing.T) {
	tests := []struct {
		name string
		sel  ResourceSelector
		want string
	}{
		{
			name: "no labels - matches everything",
			sel:  ResourceSelector{Kind: "Order"},
			want: "",
		},
		{
			name: "single label",
			sel: ResourceSelector{
				Kind:        "Order",
				MatchLabels: map[string]string{"env": "prod"},
			},
			want: "env=prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := CompileSelector(tt.sel)
			if err != nil {
				t.Fatalf("CompileSelector() error = %v", err)
			}

			got := compiled.LabelSelectorString()
			// For empty selector, both "" and empty are acceptable
			if tt.want == "" && got != "" {
				// labels.Everything() returns empty string
				if got != "" {
					t.Errorf("LabelSelectorString() = %v, want empty", got)
				}
			} else if tt.want != "" && got != tt.want {
				t.Errorf("LabelSelectorString() = %v, want %v", got, tt.want)
			}
		})
	}
}
