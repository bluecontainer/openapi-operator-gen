package nodes

import (
	"testing"
)

func TestNewFilter(t *testing.T) {
	tests := []struct {
		name    string
		opts    DiscoverOptions
		wantErr bool
	}{
		{
			name: "empty options",
			opts: DiscoverOptions{},
		},
		{
			name: "with types",
			opts: DiscoverOptions{
				Types: []string{"statefulset", "deployment"},
			},
		},
		{
			name: "with exclude labels",
			opts: DiscoverOptions{
				ExcludeLabels: []string{"app=operator", "tier=control-plane"},
			},
		},
		{
			name: "invalid label selector",
			opts: DiscoverOptions{
				ExcludeLabels: []string{"invalid===selector"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewFilter(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFilter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFilter_ShouldIncludeType(t *testing.T) {
	tests := []struct {
		name         string
		types        []string
		excludeTypes []string
		workloadType string
		want         bool
	}{
		{
			name:         "empty filters - include all",
			workloadType: "statefulset",
			want:         true,
		},
		{
			name:         "types filter - included",
			types:        []string{"statefulset", "deployment"},
			workloadType: "statefulset",
			want:         true,
		},
		{
			name:         "types filter - excluded",
			types:        []string{"statefulset"},
			workloadType: "deployment",
			want:         false,
		},
		{
			name:         "exclude-types filter",
			excludeTypes: []string{"deployment"},
			workloadType: "deployment",
			want:         false,
		},
		{
			name:         "exclude-types filter - not excluded",
			excludeTypes: []string{"deployment"},
			workloadType: "statefulset",
			want:         true,
		},
		{
			name:         "types and exclude-types - exclude takes precedence",
			types:        []string{"statefulset", "deployment"},
			excludeTypes: []string{"deployment"},
			workloadType: "deployment",
			want:         false,
		},
		{
			name:         "helm-release type",
			types:        []string{"helm-release"},
			workloadType: "helm-release",
			want:         true,
		},
		{
			name:         "case insensitive",
			types:        []string{"StatefulSet"},
			workloadType: "statefulset",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := NewFilter(DiscoverOptions{
				Types:        tt.types,
				ExcludeTypes: tt.excludeTypes,
			})
			if got := f.ShouldIncludeType(tt.workloadType); got != tt.want {
				t.Errorf("ShouldIncludeType(%q) = %v, want %v", tt.workloadType, got, tt.want)
			}
		})
	}
}

func TestFilter_ShouldExcludeByLabels(t *testing.T) {
	tests := []struct {
		name           string
		excludeLabels  []string
		workloadLabels map[string]string
		want           bool
	}{
		{
			name:           "no exclude labels",
			workloadLabels: map[string]string{"app": "myapp"},
			want:           false,
		},
		{
			name:           "matching exclude label",
			excludeLabels:  []string{"app=operator"},
			workloadLabels: map[string]string{"app": "operator"},
			want:           true,
		},
		{
			name:           "non-matching exclude label",
			excludeLabels:  []string{"app=operator"},
			workloadLabels: map[string]string{"app": "myapp"},
			want:           false,
		},
		{
			name:           "multiple exclude labels - one matches",
			excludeLabels:  []string{"app=operator", "tier=control-plane"},
			workloadLabels: map[string]string{"tier": "control-plane"},
			want:           true,
		},
		{
			name:           "set-based selector",
			excludeLabels:  []string{"environment in (prod,staging)"},
			workloadLabels: map[string]string{"environment": "prod"},
			want:           true,
		},
		{
			name:           "set-based selector - no match",
			excludeLabels:  []string{"environment in (prod,staging)"},
			workloadLabels: map[string]string{"environment": "dev"},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := NewFilter(DiscoverOptions{
				ExcludeLabels: tt.excludeLabels,
			})
			if err != nil {
				t.Fatalf("NewFilter() error = %v", err)
			}
			if got := f.ShouldExcludeByLabels(tt.workloadLabels); got != tt.want {
				t.Errorf("ShouldExcludeByLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilter_ShouldExcludeOperator(t *testing.T) {
	tests := []struct {
		name            string
		excludeOperator bool
		workloadName    string
		workloadLabels  map[string]string
		want            bool
	}{
		{
			name:            "exclude disabled",
			excludeOperator: false,
			workloadName:    "myapp-controller-manager",
			want:            false,
		},
		{
			name:            "exclude by component label",
			excludeOperator: true,
			workloadName:    "myapp",
			workloadLabels:  map[string]string{"app.kubernetes.io/component": "operator"},
			want:            true,
		},
		{
			name:            "exclude by control-plane label",
			excludeOperator: true,
			workloadName:    "myapp",
			workloadLabels:  map[string]string{"control-plane": "controller-manager"},
			want:            true,
		},
		{
			name:            "exclude by name pattern - controller-manager",
			excludeOperator: true,
			workloadName:    "petstore-controller-manager",
			workloadLabels:  map[string]string{},
			want:            true,
		},
		{
			name:            "exclude by name pattern - operator",
			excludeOperator: true,
			workloadName:    "my-operator",
			workloadLabels:  map[string]string{},
			want:            true,
		},
		{
			name:            "not excluded - normal workload",
			excludeOperator: true,
			workloadName:    "myapp-backend",
			workloadLabels:  map[string]string{"app": "myapp"},
			want:            false,
		},
		{
			name:            "not excluded - name contains operator but doesn't end with it",
			excludeOperator: true,
			workloadName:    "operator-metrics",
			workloadLabels:  map[string]string{},
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := NewFilter(DiscoverOptions{
				ExcludeOperator: tt.excludeOperator,
			})
			if got := f.ShouldExcludeOperator(tt.workloadName, tt.workloadLabels); got != tt.want {
				t.Errorf("ShouldExcludeOperator(%q, %v) = %v, want %v",
					tt.workloadName, tt.workloadLabels, got, tt.want)
			}
		})
	}
}

func TestFilter_ShouldExcludeByHealth(t *testing.T) {
	tests := []struct {
		name        string
		healthyOnly bool
		healthyPods int
		totalPods   int
		want        bool
	}{
		{
			name:        "healthy-only disabled",
			healthyOnly: false,
			healthyPods: 1,
			totalPods:   3,
			want:        false,
		},
		{
			name:        "healthy-only - all healthy",
			healthyOnly: true,
			healthyPods: 3,
			totalPods:   3,
			want:        false,
		},
		{
			name:        "healthy-only - some unhealthy",
			healthyOnly: true,
			healthyPods: 2,
			totalPods:   3,
			want:        true,
		},
		{
			name:        "healthy-only - none healthy",
			healthyOnly: true,
			healthyPods: 0,
			totalPods:   3,
			want:        true,
		},
		{
			name:        "healthy-only - zero replicas",
			healthyOnly: true,
			healthyPods: 0,
			totalPods:   0,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := NewFilter(DiscoverOptions{
				HealthyOnly: tt.healthyOnly,
			})
			if got := f.ShouldExcludeByHealth(tt.healthyPods, tt.totalPods); got != tt.want {
				t.Errorf("ShouldExcludeByHealth(%d, %d) = %v, want %v",
					tt.healthyPods, tt.totalPods, got, tt.want)
			}
		})
	}
}

func TestFilter_ShouldIncludeHelmRelease(t *testing.T) {
	tests := []struct {
		name        string
		types       []string
		healthyOnly bool
		info        *helmInfo
		want        bool
	}{
		{
			name: "no filters",
			info: &helmInfo{
				release:     "myrelease",
				totalPods:   3,
				healthyPods: 3,
			},
			want: true,
		},
		{
			name:  "helm-release type allowed",
			types: []string{"helm-release"},
			info: &helmInfo{
				release:     "myrelease",
				totalPods:   3,
				healthyPods: 3,
			},
			want: true,
		},
		{
			name:  "helm-release type not allowed",
			types: []string{"statefulset"},
			info: &helmInfo{
				release:     "myrelease",
				totalPods:   3,
				healthyPods: 3,
			},
			want: false,
		},
		{
			name:        "healthy-only - healthy",
			healthyOnly: true,
			info: &helmInfo{
				release:     "myrelease",
				totalPods:   3,
				healthyPods: 3,
			},
			want: true,
		},
		{
			name:        "healthy-only - unhealthy",
			healthyOnly: true,
			info: &helmInfo{
				release:     "myrelease",
				totalPods:   3,
				healthyPods: 2,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := NewFilter(DiscoverOptions{
				Types:       tt.types,
				HealthyOnly: tt.healthyOnly,
			})
			if got := f.ShouldIncludeHelmRelease(tt.info); got != tt.want {
				t.Errorf("ShouldIncludeHelmRelease() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidTypes(t *testing.T) {
	types := ValidTypes()
	if len(types) != 3 {
		t.Errorf("ValidTypes() returned %d types, want 3", len(types))
	}

	expected := map[string]bool{
		TypeHelmRelease: true,
		TypeStatefulSet: true,
		TypeDeployment:  true,
	}

	for _, typ := range types {
		if !expected[typ] {
			t.Errorf("Unexpected type in ValidTypes(): %s", typ)
		}
	}
}
