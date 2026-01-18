package cel

import (
	"encoding/json"
	"testing"

	"github.com/google/cel-go/common/types"
)

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name    string
		val     any
		want    float64
		wantOk  bool
	}{
		{
			name:   "float64",
			val:    float64(3.14),
			want:   3.14,
			wantOk: true,
		},
		{
			name:   "float32",
			val:    float32(2.5),
			want:   2.5,
			wantOk: true,
		},
		{
			name:   "int",
			val:    int(42),
			want:   42.0,
			wantOk: true,
		},
		{
			name:   "int32",
			val:    int32(100),
			want:   100.0,
			wantOk: true,
		},
		{
			name:   "int64",
			val:    int64(999),
			want:   999.0,
			wantOk: true,
		},
		{
			name:   "uint",
			val:    uint(50),
			want:   50.0,
			wantOk: true,
		},
		{
			name:   "uint32",
			val:    uint32(200),
			want:   200.0,
			wantOk: true,
		},
		{
			name:   "uint64",
			val:    uint64(1000),
			want:   1000.0,
			wantOk: true,
		},
		{
			name:   "json.Number valid",
			val:    json.Number("123.45"),
			want:   123.45,
			wantOk: true,
		},
		{
			name:   "json.Number integer",
			val:    json.Number("42"),
			want:   42.0,
			wantOk: true,
		},
		{
			name:   "json.Number invalid",
			val:    json.Number("not-a-number"),
			want:   0,
			wantOk: false,
		},
		{
			name:   "string - not convertible",
			val:    "hello",
			want:   0,
			wantOk: false,
		},
		{
			name:   "nil",
			val:    nil,
			want:   0,
			wantOk: false,
		},
		{
			name:   "bool - not convertible",
			val:    true,
			want:   0,
			wantOk: false,
		},
		{
			name:   "slice - not convertible",
			val:    []int{1, 2, 3},
			want:   0,
			wantOk: false,
		},
		{
			name:   "negative int",
			val:    int(-50),
			want:   -50.0,
			wantOk: true,
		},
		{
			name:   "negative float",
			val:    float64(-3.14),
			want:   -3.14,
			wantOk: true,
		},
		{
			name:   "zero int",
			val:    int(0),
			want:   0.0,
			wantOk: true,
		},
		{
			name:   "zero float",
			val:    float64(0.0),
			want:   0.0,
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ToFloat64(tt.val)
			if ok != tt.wantOk {
				t.Errorf("ToFloat64() ok = %v, wantOk %v", ok, tt.wantOk)
				return
			}
			if ok && got != tt.want {
				t.Errorf("ToFloat64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSumList(t *testing.T) {
	tests := []struct {
		name    string
		values  []any
		want    float64
		wantErr bool
	}{
		{
			name:   "integers",
			values: []any{int64(1), int64(2), int64(3), int64(4), int64(5)},
			want:   15.0,
		},
		{
			name:   "floats",
			values: []any{1.5, 2.5, 3.0},
			want:   7.0,
		},
		{
			name:   "mixed numeric types",
			values: []any{int64(1), float64(2.5), int32(3)},
			want:   6.5,
		},
		{
			name:   "empty list",
			values: []any{},
			want:   0.0,
		},
		{
			name:   "single value",
			values: []any{float64(42)},
			want:   42.0,
		},
		{
			name:   "with non-numeric values (ignored)",
			values: []any{int64(1), "text", int64(2), nil, int64(3)},
			want:   6.0,
		},
		{
			name:   "negative values",
			values: []any{float64(-5), float64(10), float64(-3)},
			want:   2.0,
		},
		{
			name:   "large numbers",
			values: []any{float64(1000000), float64(2000000), float64(3000000)},
			want:   6000000.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a CEL list from the values
			celList := types.DefaultTypeAdapter.NativeToValue(tt.values)

			result := SumList(celList)

			if types.IsError(result) {
				if !tt.wantErr {
					t.Errorf("SumList() returned error: %v", result)
				}
				return
			}

			got, ok := result.Value().(float64)
			if !ok {
				t.Errorf("SumList() result is not float64: %T", result.Value())
				return
			}

			if got != tt.want {
				t.Errorf("SumList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSumList_InvalidInput(t *testing.T) {
	// Test with non-list input
	result := SumList(types.String("not a list"))

	if !types.IsError(result) {
		t.Error("SumList() should return error for non-list input")
	}
}

func TestMaxList(t *testing.T) {
	tests := []struct {
		name   string
		values []any
		want   float64
	}{
		{
			name:   "positive integers",
			values: []any{int64(1), int64(5), int64(3), int64(9), int64(2)},
			want:   9.0,
		},
		{
			name:   "floats",
			values: []any{1.5, 2.5, 9.9, 3.0},
			want:   9.9,
		},
		{
			name:   "negative values",
			values: []any{float64(-5), float64(-1), float64(-10)},
			want:   -1.0,
		},
		{
			name:   "mixed positive and negative",
			values: []any{float64(-5), float64(10), float64(-3), float64(7)},
			want:   10.0,
		},
		{
			name:   "empty list",
			values: []any{},
			want:   0.0,
		},
		{
			name:   "single value",
			values: []any{float64(42)},
			want:   42.0,
		},
		{
			name:   "all same values",
			values: []any{float64(5), float64(5), float64(5)},
			want:   5.0,
		},
		{
			name:   "with non-numeric values (ignored)",
			values: []any{int64(1), "text", int64(100), nil},
			want:   100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			celList := types.DefaultTypeAdapter.NativeToValue(tt.values)

			result := MaxList(celList)

			if types.IsError(result) {
				t.Errorf("MaxList() returned error: %v", result)
				return
			}

			got, ok := result.Value().(float64)
			if !ok {
				t.Errorf("MaxList() result is not float64: %T", result.Value())
				return
			}

			if got != tt.want {
				t.Errorf("MaxList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMaxList_InvalidInput(t *testing.T) {
	result := MaxList(types.Int(42))

	if !types.IsError(result) {
		t.Error("MaxList() should return error for non-list input")
	}
}

func TestMinList(t *testing.T) {
	tests := []struct {
		name   string
		values []any
		want   float64
	}{
		{
			name:   "positive integers",
			values: []any{int64(5), int64(3), int64(1), int64(9), int64(2)},
			want:   1.0,
		},
		{
			name:   "floats",
			values: []any{1.5, 0.5, 2.5, 3.0},
			want:   0.5,
		},
		{
			name:   "negative values",
			values: []any{float64(-5), float64(-1), float64(-10)},
			want:   -10.0,
		},
		{
			name:   "mixed positive and negative",
			values: []any{float64(-5), float64(10), float64(-3), float64(7)},
			want:   -5.0,
		},
		{
			name:   "empty list",
			values: []any{},
			want:   0.0,
		},
		{
			name:   "single value",
			values: []any{float64(42)},
			want:   42.0,
		},
		{
			name:   "all same values",
			values: []any{float64(5), float64(5), float64(5)},
			want:   5.0,
		},
		{
			name:   "with non-numeric values (ignored)",
			values: []any{int64(100), "text", int64(1), nil},
			want:   1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			celList := types.DefaultTypeAdapter.NativeToValue(tt.values)

			result := MinList(celList)

			if types.IsError(result) {
				t.Errorf("MinList() returned error: %v", result)
				return
			}

			got, ok := result.Value().(float64)
			if !ok {
				t.Errorf("MinList() result is not float64: %T", result.Value())
				return
			}

			if got != tt.want {
				t.Errorf("MinList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMinList_InvalidInput(t *testing.T) {
	result := MinList(types.Bool(true))

	if !types.IsError(result) {
		t.Error("MinList() should return error for non-list input")
	}
}

func TestAvgList(t *testing.T) {
	tests := []struct {
		name   string
		values []any
		want   float64
	}{
		{
			name:   "integers",
			values: []any{int64(2), int64(4), int64(6), int64(8)},
			want:   5.0,
		},
		{
			name:   "floats",
			values: []any{1.0, 2.0, 3.0},
			want:   2.0,
		},
		{
			name:   "mixed values",
			values: []any{int64(1), float64(2.5), int64(3)},
			want:   6.5 / 3.0,
		},
		{
			name:   "empty list",
			values: []any{},
			want:   0.0,
		},
		{
			name:   "single value",
			values: []any{float64(42)},
			want:   42.0,
		},
		{
			name:   "negative values",
			values: []any{float64(-4), float64(4)},
			want:   0.0,
		},
		{
			name:   "with non-numeric values (ignored)",
			values: []any{int64(10), "text", int64(20), nil},
			want:   15.0,
		},
		{
			name:   "fractional result",
			values: []any{int64(1), int64(2)},
			want:   1.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			celList := types.DefaultTypeAdapter.NativeToValue(tt.values)

			result := AvgList(celList)

			if types.IsError(result) {
				t.Errorf("AvgList() returned error: %v", result)
				return
			}

			got, ok := result.Value().(float64)
			if !ok {
				t.Errorf("AvgList() result is not float64: %T", result.Value())
				return
			}

			// Use tolerance for floating point comparison
			diff := got - tt.want
			if diff < -0.0001 || diff > 0.0001 {
				t.Errorf("AvgList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAvgList_InvalidInput(t *testing.T) {
	result := AvgList(types.Double(3.14))

	if !types.IsError(result) {
		t.Error("AvgList() should return error for non-list input")
	}
}

func TestAggregateFunctions(t *testing.T) {
	// Test that AggregateFunctions returns the expected number of functions
	funcs := AggregateFunctions()

	// Should have 4 functions: sum, max, min, avg
	if len(funcs) != 4 {
		t.Errorf("AggregateFunctions() returned %d options, want 4", len(funcs))
	}
}

func TestAggregateFunctions_Integration(t *testing.T) {
	// Test that the functions work in a CEL environment
	env, err := NewEnvironment([]string{})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	vars := map[string]any{
		"resources": []map[string]any{
			{"value": int64(10)},
			{"value": int64(20)},
			{"value": int64(30)},
		},
		"summary": map[string]int64{"total": 3},
	}

	tests := []struct {
		name       string
		expression string
		want       float64
	}{
		{
			name:       "sum with map",
			expression: "sum(resources.map(r, r.value))",
			want:       60.0,
		},
		{
			name:       "max with map",
			expression: "max(resources.map(r, r.value))",
			want:       30.0,
		},
		{
			name:       "min with map",
			expression: "min(resources.map(r, r.value))",
			want:       10.0,
		},
		{
			name:       "avg with map",
			expression: "avg(resources.map(r, r.value))",
			want:       20.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Evaluate(env, tt.expression, vars)
			if result.Error != nil {
				t.Errorf("Evaluate() error = %v", result.Error)
				return
			}

			got, ok := result.Value.(float64)
			if !ok {
				t.Errorf("Evaluate() result is not float64: %T", result.Value)
				return
			}

			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAggregateFunctions_EdgeCases(t *testing.T) {
	env, err := NewEnvironment([]string{})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	vars := map[string]any{
		"resources": []map[string]any{},
		"summary":   map[string]int64{"total": 0},
	}

	tests := []struct {
		name       string
		expression string
		want       float64
	}{
		{
			name:       "sum of empty list",
			expression: "sum([])",
			want:       0.0,
		},
		{
			name:       "max of empty list",
			expression: "max([])",
			want:       0.0,
		},
		{
			name:       "min of empty list",
			expression: "min([])",
			want:       0.0,
		},
		{
			name:       "avg of empty list",
			expression: "avg([])",
			want:       0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Evaluate(env, tt.expression, vars)
			if result.Error != nil {
				t.Errorf("Evaluate() error = %v", result.Error)
				return
			}

			got, ok := result.Value.(float64)
			if !ok {
				t.Errorf("Evaluate() result is not float64: %T", result.Value)
				return
			}

			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}
