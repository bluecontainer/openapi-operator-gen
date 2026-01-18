// Package cel provides shared CEL (Common Expression Language) utilities
// for evaluating expressions in aggregate and bundle CRDs.
package cel

import (
	"encoding/json"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// ToFloat64 converts various numeric types to float64.
// Returns the converted value and true if conversion was successful.
func ToFloat64(val any) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	}
	return 0, false
}

// SumList implements the sum() aggregate function for CEL.
// It takes a list of numeric values and returns their sum.
func SumList(val ref.Val) ref.Val {
	iter, ok := val.(traits.Lister)
	if !ok {
		return types.NewErr("sum() requires a list argument")
	}

	var sum float64
	it := iter.Iterator()
	for it.HasNext() == types.True {
		elem := it.Next()
		if num, ok := ToFloat64(elem.Value()); ok {
			sum += num
		}
	}
	return types.Double(sum)
}

// MaxList implements the max() aggregate function for CEL.
// It takes a list of numeric values and returns the maximum.
func MaxList(val ref.Val) ref.Val {
	iter, ok := val.(traits.Lister)
	if !ok {
		return types.NewErr("max() requires a list argument")
	}

	var max float64
	first := true
	it := iter.Iterator()
	for it.HasNext() == types.True {
		elem := it.Next()
		if num, ok := ToFloat64(elem.Value()); ok {
			if first || num > max {
				max = num
				first = false
			}
		}
	}
	if first {
		return types.Double(0)
	}
	return types.Double(max)
}

// MinList implements the min() aggregate function for CEL.
// It takes a list of numeric values and returns the minimum.
func MinList(val ref.Val) ref.Val {
	iter, ok := val.(traits.Lister)
	if !ok {
		return types.NewErr("min() requires a list argument")
	}

	var min float64
	first := true
	it := iter.Iterator()
	for it.HasNext() == types.True {
		elem := it.Next()
		if num, ok := ToFloat64(elem.Value()); ok {
			if first || num < min {
				min = num
				first = false
			}
		}
	}
	if first {
		return types.Double(0)
	}
	return types.Double(min)
}

// AvgList implements the avg() aggregate function for CEL.
// It takes a list of numeric values and returns their average.
func AvgList(val ref.Val) ref.Val {
	iter, ok := val.(traits.Lister)
	if !ok {
		return types.NewErr("avg() requires a list argument")
	}

	var sum float64
	var count int
	it := iter.Iterator()
	for it.HasNext() == types.True {
		elem := it.Next()
		if num, ok := ToFloat64(elem.Value()); ok {
			sum += num
			count++
		}
	}
	if count == 0 {
		return types.Double(0)
	}
	return types.Double(sum / float64(count))
}

// AggregateFunctions returns the CEL function declarations for aggregate functions.
// These should be added to the CEL environment options.
func AggregateFunctions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("sum",
			cel.Overload("sum_list",
				[]*cel.Type{cel.ListType(cel.DynType)},
				cel.DoubleType,
				cel.UnaryBinding(SumList),
			),
		),
		cel.Function("max",
			cel.Overload("max_list",
				[]*cel.Type{cel.ListType(cel.DynType)},
				cel.DoubleType,
				cel.UnaryBinding(MaxList),
			),
		),
		cel.Function("min",
			cel.Overload("min_list",
				[]*cel.Type{cel.ListType(cel.DynType)},
				cel.DoubleType,
				cel.UnaryBinding(MinList),
			),
		),
		cel.Function("avg",
			cel.Overload("avg_list",
				[]*cel.Type{cel.ListType(cel.DynType)},
				cel.DoubleType,
				cel.UnaryBinding(AvgList),
			),
		),
	}
}
