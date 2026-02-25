# MCP Server — Operator Lifecycle Features Roadmap

Features to make the MCP server useful for working with already-generated operators, beyond the initial generation workflow.

## Operator-Aware Tools

### `describe` ✅ Priority
Inspect a generated operator by reading its `.openapi-operator-gen.yaml` config and source files. Show:
- All CRDs with their fields, types, required/optional, enums
- Endpoint mappings (which REST paths map to which CRDs)
- What options were used to generate (aggregate, bundle, filters, etc.)
- File ownership summary (what's safe to customize vs regenerated)

Essentially the "preview" tool but for an already-generated operator, reading from the output directory rather than the spec.

### `regenerate` ✅ Priority
Re-run generation using the saved `.openapi-operator-gen.yaml` config. Accepts override flags to change options on the fly. Useful when the spec changes — the assistant just calls `regenerate` instead of remembering all the original flags.

### `diff` ✅ Priority
Compare the current OpenAPI spec against what was last generated from. Show:
- New/removed/changed endpoints
- New/removed/changed fields on existing resources
- What CRDs would be added/removed/modified
- Breaking changes (field type changes, removed required fields)

Answers "what would change if I regenerate now?"

### `explain`
Given a CRD kind name, explain its reconciliation logic in plain language: what HTTP calls it makes, how it handles create vs update vs delete, what the finalizer does, how drift detection works. Reads the generated controller source and summarizes it.

### `sample`
Generate example CR YAML for a given CRD kind, pre-populated with realistic field values from the OpenAPI spec (using enum values, examples, defaults). Tailored to a specific scenario the user describes.

## Resources (Passive Context)

MCP resources let the assistant pull in context on demand without the user pointing at files:

- **`operator://config`** — The `.openapi-operator-gen.yaml` as structured data
- **`operator://crds`** — List of all CRDs with their schemas
- **`operator://spec`** — The original OpenAPI spec summary
- **`operator://file-ownership`** — What's safe to edit vs what gets overwritten

## Prompts (Guided Workflows)

### `evolve-spec`
Walk through modifying the OpenAPI spec and regenerating. Steps: describe what you want to change → show the current relevant CRDs → suggest spec edits → diff the impact → regenerate → build/test.

### `customize-operator`
Guide through customizing the safe-to-edit parts: deployment resource limits, RBAC rules, kustomize overlays, environment variables. The assistant knows which files are safe because the MCP server tells it.

### `debug-reconciliation`
Help troubleshoot a CR that isn't syncing. Steps: check the CR status conditions → check controller logs → explain the expected reconciliation flow → suggest fixes.

### `deploy`
Walk through deploying to a cluster: build image → push to registry → install CRDs → deploy operator → create sample CR → verify status.
