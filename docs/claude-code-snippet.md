# Claude Code Snippet for openapi-operator-gen

Copy the section below into your project's `CLAUDE.md` file (create one if it
doesn't exist). Replace the placeholder values in `[BRACKETS]` with your
project-specific values.

If you have the MCP server configured (`openapi-operator-gen setup claude-code`),
Claude can use the validate/preview/generate tools directly and this snippet is
optional but still useful for documenting your project's conventions.

---

```markdown
## Kubernetes Operator Generation

This project uses [openapi-operator-gen](https://github.com/bluecontainer/openapi-operator-gen)
to generate a Kubernetes operator from an OpenAPI specification.

### Setup

Install the generator:
```
go install github.com/bluecontainer/openapi-operator-gen@latest
```

### Spec and Configuration

- OpenAPI spec: `[./path/to/openapi.yaml]`
- Output directory: `[./operator]`
- API group: `[myapp.example.com]`
- Go module: `[github.com/myorg/myapp-operator]`

### Generate

```
openapi-operator-gen generate \
  --spec [./path/to/openapi.yaml] \
  --output [./operator] \
  --group [myapp.example.com] \
  --version v1alpha1 \
  --module [github.com/myorg/myapp-operator]
```

### Build and Test

```
cd [./operator]
go mod tidy
make generate
make manifests
make build
make test
```

### Regeneration

When the OpenAPI spec changes, re-run the generate command above. These files
are regenerated and should not be hand-edited:
- `api/` — CRD Go types
- `internal/controller/` — Reconciliation logic
- `config/crd/` — CRD YAML manifests
- `main.go`, `Dockerfile`, `Makefile`, `go.mod`

These files are safe to customize:
- `config/manager/` — Deployment settings
- `config/rbac/` — RBAC rules
- `config/samples/` — Example CRs
- `config/default/` — Kustomize overlays
```
