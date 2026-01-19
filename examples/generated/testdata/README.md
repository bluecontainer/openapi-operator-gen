# CEL Test Data for petstore Operator

This directory contains test data for validating CEL expressions used in Aggregate and Bundle CRDs.

> **Note:** For complete cel-test CLI documentation including all commands, flags, and advanced usage,
> see the [cel-test README](https://github.com/bluecontainer/openapi-operator-gen/blob/main/cmd/cel-test/README.md).

## Test Data File

- `cel-test-data.json` - Sample data representing a petstore deployment with example resources

## Using the cel-test CLI

Build the CEL test utility from the openapi-operator-gen repository:

```bash
# Clone openapi-operator-gen if not already done
git clone https://github.com/bluecontainer/openapi-operator-gen.git
cd openapi-operator-gen

# Build the CEL test utility
make build-cel-test
```

### Example Expressions

**Summary-based expressions:**

```bash
# Get sync percentage
./bin/cel-test eval "summary.synced * 100 / summary.total" \
  --data path/to/testdata/cel-test-data.json

# Check if any failures exist
./bin/cel-test eval "summary.failed > 0" \
  --data path/to/testdata/cel-test-data.json

# Get pending count with guard for division
./bin/cel-test eval "summary.total > 0 ? summary.pending * 100 / summary.total : 0" \
  --data path/to/testdata/cel-test-data.json
```

**Resource list expressions:**

```bash
# Count total resources
./bin/cel-test eval "resources.size()" \
  --data path/to/testdata/cel-test-data.json

# Count synced resources
./bin/cel-test eval "resources.filter(r, r.status.state == 'Synced').size()" \
  --data path/to/testdata/cel-test-data.json

# Check if all resources are synced
./bin/cel-test eval "resources.all(r, r.status.state == 'Synced')" \
  --data path/to/testdata/cel-test-data.json

# Get names of failed resources
./bin/cel-test eval "resources.filter(r, r.status.state == 'Failed').map(r, r.metadata.name)" \
  --data path/to/testdata/cel-test-data.json
```

**Kind-specific expressions:**

```bash
# Count orders
./bin/cel-test eval "orders.size()" \
  --data path/to/testdata/cel-test-data.json \
  --kinds orders
# Count pets
./bin/cel-test eval "pets.size()" \
  --data path/to/testdata/cel-test-data.json \
  --kinds pets
# Count users
./bin/cel-test eval "users.size()" \
  --data path/to/testdata/cel-test-data.json \
  --kinds users

# Sum values from a numeric field
./bin/cel-test eval "sum(resources.map(r, r.spec.id))" \
  --data path/to/testdata/cel-test-data.json

# Average of numeric values
./bin/cel-test eval "avg(resources.map(r, r.spec.id))" \
  --data path/to/testdata/cel-test-data.json
```

### Interactive Mode

For testing multiple expressions interactively:

```bash
./bin/cel-test interactive \
  --data path/to/testdata/cel-test-data.json \
  --kinds orders,pets,users,petfindbystatusquerys,petfindbytagsquerys,storeinventoryquerys,userloginquerys,userlogoutquerys,petuploadimageactions,usercreatewithlistactions
```

Then enter expressions at the prompt:
```
CEL> summary.total
Result: 12
CEL> resources.size()
Result: ...
CEL> exit
```

## Using in Aggregate/Bundle CRDs

These expressions can be used in the `derivedValues` field:

```yaml
apiVersion: petstore.example.com/v1alpha1
kind: PetstoreAggregate
metadata:
  name: petstore-status
spec:
  resourceRefs:
    - kind: Order
      labelSelector:
        matchLabels:
          app: petstore
    - kind: Pet
      labelSelector:
        matchLabels:
          app: petstore
    - kind: User
      labelSelector:
        matchLabels:
          app: petstore
  derivedValues:
    - name: syncPercentage
      expression: "summary.total > 0 ? summary.synced * 100 / summary.total : 0"
    - name: hasFailures
      expression: "summary.failed > 0"
    - name: totalResources
      expression: "resources.size()"
```

## Available Variables

| Variable | Type | Description |
|----------|------|-------------|
| `resources` | `list<map>` | All resources in the aggregate/bundle |
| `summary` | `map<string,int>` | Counts: total, synced, failed, pending, skipped |
| `orders` | `list<map>` | Order resources only |
| `pets` | `list<map>` | Pet resources only |
| `users` | `list<map>` | User resources only |
| `petfindbystatusquerys` | `list<map>` | PetFindbystatusQuery resources only |
| `petfindbytagsquerys` | `list<map>` | PetFindbytagsQuery resources only |
| `storeinventoryquerys` | `list<map>` | StoreInventoryQuery resources only |
| `userloginquerys` | `list<map>` | UserLoginQuery resources only |
| `userlogoutquerys` | `list<map>` | UserLogoutQuery resources only |
| `petuploadimageactions` | `list<map>` | PetUploadimageAction resources only |
| `usercreatewithlistactions` | `list<map>` | UserCreatewithlistAction resources only |

## Available Functions

| Function | Description | Example |
|----------|-------------|---------|
| `sum(list)` | Sum of numeric values | `sum(resources.map(r, r.spec.quantity))` |
| `max(list)` | Maximum value | `max(resources.map(r, r.spec.id))` |
| `min(list)` | Minimum value | `min(resources.map(r, r.spec.id))` |
| `avg(list)` | Average value | `avg(resources.map(r, r.spec.quantity))` |
| `size()` | List length | `resources.size()` |
| `filter(r, cond)` | Filter list | `resources.filter(r, r.status.state == 'Synced')` |
| `map(r, expr)` | Transform list | `resources.map(r, r.metadata.name)` |
| `exists(r, cond)` | Any match | `resources.exists(r, r.status.state == 'Failed')` |
| `all(r, cond)` | All match | `resources.all(r, r.status.state == 'Synced')` |
| `has(field)` | Check field exists | `has(r.spec.quantity) ? r.spec.quantity : 0` |

## Data Structure

The test data follows this structure:

```json
{
  "summary": {
    "total": 12,
    "synced": 8,
    "failed": 2,
    "pending": 2,
    "skipped": 0
  },
  "resources": [
    {
      "kind": "ResourceKind",
      "metadata": {"name": "...", "namespace": "..."},
      "spec": {"...": "..."},
      "status": {"state": "Synced|Failed|Pending", "externalID": "...", "message": "..."}
    }
  ],
  "kindLists": {
    "orders": [...]
    ,"pets": [...]
    ,"users": [...]
    ,"petfindbystatusquerys": [...]
    ,"petfindbytagsquerys": [...]
    ,"storeinventoryquerys": [...]
    ,"userloginquerys": [...]
    ,"userlogoutquerys": [...]
    ,"petuploadimageactions": [...]
    ,"usercreatewithlistactions": [...]
  }
}
```

## Additional Features

### Mock Data for Testing

Generate mock data without needing actual resources:

```bash
./bin/cel-test expressions --cr aggregate.yaml --mock
```

### Fetching Live Data from Kubernetes

Fetch resources directly from a cluster:

```bash
# Fetch and save as JSON for testing
./bin/cel-test fetch --kinds Order,Pet,User \
  --api-group petstore.example.com \
  --output cel-test-data.json

# Or evaluate expressions directly against live data
./bin/cel-test eval "resources.size()" \
  --kinds Order,Pet,User \
  --api-group petstore.example.com
```

## Troubleshooting

### Empty spec data

When using `--cr` with status data, spec fields are empty by default. Use `--resources` to provide actual resource specs:

```bash
./bin/cel-test eval "orders[0].spec.quantity" --cr aggregate.yaml --resources resources.yaml
```

### Expression errors with optional fields

Use `has()` to check for optional fields before accessing them:

```bash
# Instead of: r.spec.quantity (may error if field missing)
# Use: has(r.spec.quantity) ? r.spec.quantity : 0
```

### Division by zero

Always guard division operations:

```bash
# Safe division
./bin/cel-test eval "summary.total > 0 ? summary.synced * 100 / summary.total : 0"
```

For more troubleshooting tips, see the [cel-test README](https://github.com/bluecontainer/openapi-operator-gen/blob/main/cmd/cel-test/README.md#troubleshooting).
