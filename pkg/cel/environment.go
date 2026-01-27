package cel

import (
	"strings"

	"github.com/google/cel-go/cel"
)

// NewEnvironment creates a new CEL environment with standard variables, aggregate functions, and datetime functions.
// kindNames is a list of kind-specific variable names (e.g., ["orders", "pets", "users"]).
// The environment includes:
//   - resources: list of all resource maps
//   - summary: map with total, synced, failed, pending counts
//   - kind-specific variables (e.g., orders, pets) for each kind name provided
//   - Aggregate functions: sum(), max(), min(), avg()
//   - DateTime functions: now(), nowUnix(), formatTime(), parseTime(), addDuration(), timeSince(), etc.
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

	// Add datetime functions
	opts = append(opts, DateTimeFunctions()...)

	return cel.NewEnv(opts...)
}

// NewEnvironmentWithResources creates a new CEL environment with standard variables, aggregate functions,
// datetime functions, and resource-specific variables using the naming convention {kind}_{name}.
// kindNames is a list of kind-specific variable names (e.g., ["orders", "pets", "users"]).
// resourceKeys is a list of resource-specific variable names (e.g., ["pet_fluffy", "order_1"]).
// The environment includes:
//   - resources: list of all resource maps
//   - summary: map with total, synced, failed, pending counts
//   - kind-specific variables (e.g., orders, pets) for each kind name provided
//   - resource-specific variables (e.g., pet_fluffy, storeinventoryquery_sample)
//   - Aggregate functions: sum(), max(), min(), avg()
//   - DateTime functions: now(), nowUnix(), formatTime(), parseTime(), addDuration(), timeSince(), etc.
func NewEnvironmentWithResources(kindNames []string, resourceKeys []string) (*cel.Env, error) {
	// Build base options with standard variables
	opts := []cel.EnvOption{
		cel.Variable("resources", cel.ListType(cel.MapType(cel.StringType, cel.DynType))),
		cel.Variable("summary", cel.MapType(cel.StringType, cel.IntType)),
	}

	// Add kind-specific variables (lists)
	for _, kindName := range kindNames {
		opts = append(opts, cel.Variable(kindName, cel.ListType(cel.MapType(cel.StringType, cel.DynType))))
	}

	// Add resource-specific variables (individual maps)
	// These use the {kind}-{name} naming convention
	for _, resourceKey := range resourceKeys {
		opts = append(opts, cel.Variable(resourceKey, cel.MapType(cel.StringType, cel.DynType)))
	}

	// Add aggregate functions
	opts = append(opts, AggregateFunctions()...)

	// Add datetime functions
	opts = append(opts, DateTimeFunctions()...)

	return cel.NewEnv(opts...)
}

// ResourceKey generates a CEL variable name for a specific resource using the {kind}_{name} convention.
// The kind is lowercased and the name has hyphens replaced with underscores since CEL doesn't allow hyphens in identifiers.
// Example: ResourceKey("StoreInventoryQuery", "sample") returns "storeinventoryquery_sample"
// Example: ResourceKey("Order", "order-001") returns "order_order_001"
func ResourceKey(kind, name string) string {
	// Replace hyphens with underscores in the name since CEL treats hyphens as subtraction
	sanitizedName := strings.ReplaceAll(name, "-", "_")
	return strings.ToLower(kind) + "_" + sanitizedName
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

	// Add datetime functions
	opts = append(opts, DateTimeFunctions()...)

	// Add any additional options
	opts = append(opts, additionalOpts...)

	return cel.NewEnv(opts...)
}

// NewBundleConditionEnvironment creates a CEL environment for evaluating Bundle readyWhen/skipWhen conditions.
// Unlike the standard environment where `resources` is a list, this environment treats `resources` as a map
// where keys are resource IDs and values are maps with status information.
// This allows expressions like: resources.backend.status.state == 'Synced'
func NewBundleConditionEnvironment() (*cel.Env, error) {
	opts := []cel.EnvOption{
		// resources is a map where keys are resource IDs
		cel.Variable("resources", cel.MapType(cel.StringType, cel.DynType)),
	}

	// Add datetime functions (useful for time-based conditions)
	opts = append(opts, DateTimeFunctions()...)

	return cel.NewEnv(opts...)
}
