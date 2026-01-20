package cel

import (
	"github.com/google/cel-go/cel"
)

// NewEnvironment creates a new CEL environment with standard variables and aggregate functions.
// kindNames is a list of kind-specific variable names (e.g., ["orders", "pets", "users"]).
// The environment includes:
//   - resources: list of all resource maps
//   - summary: map with total, synced, failed, pending counts
//   - kind-specific variables (e.g., orders, pets) for each kind name provided
//   - Aggregate functions: sum(), max(), min(), avg()
func NewEnvironment(kindNames []string) (*cel.Env, error) {
	// Build base options with standard variables
	opts := []cel.EnvOption{
		cel.Variable("resources", cel.ListType(cel.MapType(cel.StringType, cel.DynType))),
		cel.Variable("summary", cel.MapType(cel.StringType, cel.IntType)),
	}

	// Add kind-specific variables
	for _, kindName := range kindNames {
		opts = append(opts, cel.Variable(kindName, cel.ListType(cel.MapType(cel.StringType, cel.DynType))))
	}

	// Add aggregate functions
	opts = append(opts, AggregateFunctions()...)

	return cel.NewEnv(opts...)
}

// NewEnvironmentWithOptions creates a new CEL environment with custom options.
// This allows callers to add additional variables or functions beyond the standard set.
func NewEnvironmentWithOptions(kindNames []string, additionalOpts ...cel.EnvOption) (*cel.Env, error) {
	// Build base options with standard variables
	opts := []cel.EnvOption{
		cel.Variable("resources", cel.ListType(cel.MapType(cel.StringType, cel.DynType))),
		cel.Variable("summary", cel.MapType(cel.StringType, cel.IntType)),
	}

	// Add kind-specific variables
	for _, kindName := range kindNames {
		opts = append(opts, cel.Variable(kindName, cel.ListType(cel.MapType(cel.StringType, cel.DynType))))
	}

	// Add aggregate functions
	opts = append(opts, AggregateFunctions()...)

	// Add any additional options
	opts = append(opts, additionalOpts...)

	return cel.NewEnv(opts...)
}
