# Zero-Downtime Configuration: Pattern Selection by Application Type

## Baseline Assumption

Every application discussed here **reads a configuration file at startup** (YAML, JSON, properties, etc.). This is the common starting point. The question is: what happens when that configuration needs to change while the application is already running?

Two patterns address this. Which one to use — or whether to invest in enabling one — depends entirely on what the application already supports.

| Pattern | Mechanism | Requires from the Application |
|---------|-----------|-------------------------------|
| **Dynamic Configuration Change (DCC)** | ConfigMap mounted as volume; Kubelet refreshes file on disk | File watcher + in-memory reload logic |
| **Operator-Driven Configuration** (openapi-operator-gen) | Custom Resource → generated operator → REST API call to application | HTTP endpoint that accepts config updates |

---

## Application Types and Pattern Selection

### Type A: Static Config Only

**Capabilities:** Reads config file at startup. No file watcher. No config API.

**What happens today:** Configuration changes require a pod restart. The ConfigMap (or image) is updated, and a RollingUpdate replaces pods. New pods read the updated file on startup.

**Zero-downtime possible?** No — neither pattern works without application changes.

**Recommendation to unlock zero-downtime:** Add a config API (Type C), not a file watcher (Type B).

| Approach | What to Add | Lines of Code | Complexity | What You Get |
|----------|-------------|---------------|------------|--------------|
| Add file watcher (→ Type B) | Watcher library + reload logic + checksum validation + fail-safe fallback + observability hooks | ~200–500 | High | ConfigMap-based hot-reload with up to 2 min latency, no drift detection |
| **Add config API (→ Type C)** | **HTTP handler for GET + PUT on the existing config struct** | **~20–50** | **Low** | **Immediate config delivery, drift detection, CRD validation, GitOps-native, auto re-apply after restart** |

**Why the config API path is recommended:**

1. **The application already has a config struct.** It parses the startup file into an internal structure. A config API simply exposes that struct over HTTP — read it with GET, replace it with PUT. The parsing logic already exists.

2. **No new concurrency concerns.** A file watcher introduces a background thread that must coordinate with the main application loop, handle partial writes, validate checksums, and implement atomic swaps. An HTTP handler runs in the application's existing request lifecycle.

3. **The startup config file still works.** The application boots from its config file exactly as before. The config API is only used for runtime changes *after* startup. The operator detects the pod is running, GETs the current config, compares it to the CR spec, and PUTs any differences.

4. **OpenAPI spec can be generated from the config struct.** Tools like `swaggo/swag` (Go), `springdoc` (Java), or `fastapi` (Python) can generate the OpenAPI spec directly from the config model, which openapi-operator-gen then uses to produce the operator.

### Type B: File Watcher / Hot-Reload

**Capabilities:** Reads config file at startup. Watches the filesystem for changes. Reloads config in-memory when the file changes.

**Pattern:** Dynamic Configuration Change (DCC)

```
Git commit → CI/CD pipeline → ConfigMap updated → Kubelet refreshes symlink
    → Application file watcher detects change → Validates → Reloads in-memory
```

**This works today with no additional changes.** The application already does the hard part (watching + reloading). The DCC pattern is the right fit.

**Constraints to be aware of:**

| Constraint | Impact |
|-----------|--------|
| Kubelet sync latency | Up to 2 min between ConfigMap update and file change on disk |
| `subPath` volume mounts | Will **not** auto-update — only full directory mounts refresh |
| ConfigMap size limit | 1MB maximum |
| Namespace isolation | ConfigMap must be in the same namespace as the pod |
| No drift detection | If config is changed outside the ConfigMap (e.g., `kubectl exec`), nothing corrects it |
| Full-file reload disruption | The application reloads the entire config file, not individual fields. If reloading involves reinitializing connections, restarting listeners, or resetting internal state, the reload itself disrupts the application's service — making it little different from a deployment upgrade. Additionally, not all settings may be safely modifiable at runtime (e.g., listen ports, storage backends), meaning some changes still require a restart regardless. |
| Observability | Application must implement custom logging (`config_reloaded` events, `reload_duration_ms` metrics) |

**Examples of applications that are naturally Type B:** Prometheus, Envoy, Nginx (with reload), Fluentd, Grafana.

### Type C: Config API

**Capabilities:** Reads config file at startup. Exposes REST endpoints to read and write configuration at runtime (e.g., `GET /config`, `PUT /config`). Does not need to watch files.

**Pattern:** Operator-Driven Configuration (openapi-operator-gen)

```
CR applied (GitOps) → Operator watches CR → GET /config from app → Compare with CR spec
    → Drift detected? → PUT /config to app → Update CR status
```

**How it works with the startup config file:**

1. Application starts, reads its config file as usual — this provides the **initial** state
2. Operator's first reconcile GETs the current config from the API
3. If the CR spec differs from what the application loaded at startup, the operator PUTs the desired state
4. Application applies the new config in-memory via its HTTP handler
5. If the pod restarts later, it boots from the original file, and the operator re-applies the CR-desired state on the next reconcile

The startup config file serves as the **bootstrap default**. The CR is the **desired state**. The operator ensures they converge.

**What this provides over the DCC pattern:**

| Capability | File Watcher (Type B) | Config API (Type C) |
|-----------|----------------------|---------------------|
| Config delivery latency | Up to 2 min | Immediate |
| Drift detection | None | Every reconcile loop |
| Re-apply after pod restart | Automatic (file remounted) | Automatic (operator re-applies via API) |
| Granular updates | No — entire file is replaced | Yes — API design can accept partial/field-level changes (PATCH) |
| Rollback to initial state | Must revert ConfigMap to original content and wait for sync | Delete the CR — operator stops reconciling, next pod restart loads bootstrap defaults |
| Validation before delivery | App-side only | CRD admission (schema-level) + app-side |
| Multi-pod fan-out | All pods (volume mount) | All pods (endpoint discovery with configurable strategy) |
| Cross-namespace | Not possible | Supported via operator's endpoint resolver |
| Config size limit | 1MB (ConfigMap) | None (CR in etcd, default 1.5MB but configurable) |
| GitOps artifact | ConfigMap YAML | Custom Resource YAML |
| Observability | Custom app metrics | Native CR status (`lastSyncTime`, `conditions`, `kubectl get`) |

### Type D: Both File Watcher and Config API

**Capabilities:** Reads config file at startup. Watches filesystem. Also exposes a config API.

In practice, the config API and the config file often **do not cover the same settings**. The API may expose runtime-tunable parameters (log levels, feature flags, connection pool sizes) while the config file controls bootstrap settings (listen addresses, TLS certificates, storage paths, plugin loading). These are complementary, not redundant.

**Recommendation:** Use **both patterns together**, each for the configuration domain it covers:

| Config Domain | Delivered Via | Pattern | Example Settings |
|--------------|--------------|---------|-----------------|
| Runtime-tunable settings | Config API → operator CR | openapi-operator-gen | Log level, feature flags, rate limits, cache TTL |
| Bootstrap / infrastructure settings | Config file → ConfigMap volume | DCC pattern | Listen address, TLS cert paths, database connection strings, plugin config |

**Why this matters:** Using only the operator pattern would leave bootstrap settings unmanageable at runtime. Using only the file watcher pattern would impose 2-min latency and no drift detection on settings that could be changed immediately via API.

**Combined workflow:**

```
Bootstrap settings (file-based):
  ConfigMap updated → Kubelet refreshes file → File watcher reloads → Settings applied

Runtime settings (API-based):
  CR spec updated → Operator reconciles → PUT /config → Settings applied immediately
                                                        ↓
  Pod restarts → Operator detects drift → Re-applies runtime settings via API
  Bootstrap settings → Loaded from config file automatically
```

**Rollback behavior in the combined model:**
- Delete the CR → the operator's finalizer fires, calling the config API to reset runtime settings back to the application's startup defaults — **no pod restart required**. Bootstrap settings remain as defined in the ConfigMap.
- Revert the ConfigMap → bootstrap settings reload via file watcher (up to 2 min); runtime settings managed by the operator are unaffected

---

## Recommendation for Type A Applications: What to Add

For applications that currently only read config at startup, the **config API path requires the least change** and provides the most capability.

- The application already has a config struct — a config API simply exposes it over HTTP (`GET` to read, `PUT` to replace). The parsing logic already exists.
- Adding an HTTP listener is a small change for applications that don't already serve HTTP, but it is a well-understood pattern with minimal dependencies — no background threads, no filesystem coordination, no checksum validation.
- The startup config file still works — the application boots from its file as before. The config API is only used for runtime changes after startup.
- An OpenAPI spec can be generated from the config struct using standard tooling (`swaggo/swag` for Go, `springdoc` for Java, `fastapi` for Python), which openapi-operator-gen then uses to produce the operator.

---

## Initial Configuration: How Each Pattern Handles Startup

Both patterns assume the application reads a config file at startup. Here's how each handles the relationship between that file and runtime config changes.

### Dynamic Configuration Change Pattern (Type B)

```
Startup:  Pod created → ConfigMap mounted as volume → App reads /etc/app/config/settings.yaml
Runtime:  ConfigMap updated → Kubelet refreshes file → App watcher detects → Reloads
Restart:  Pod recreated → ConfigMap remounted → App reads updated file at startup
```

The ConfigMap is both the startup source and the runtime update channel. They are the same file.

### Operator Pattern (Type C)

```
Startup:  Pod created → ConfigMap mounted as volume → App reads /etc/app/config/settings.yaml (bootstrap defaults)
Runtime:  CR spec applied → Operator calls PUT /config → App applies in-memory
Restart:  Pod recreated → ConfigMap remounted → App reads startup file (bootstrap defaults) → Operator detects
          drift between startup state and CR spec → PUT /config → App converges
```

The startup config file provides **sane defaults**. The CR spec provides the **desired state**. They can differ — the operator always reconciles toward the CR. This means:

- The startup file doesn't need to be updated when config changes
- The startup file can remain static in the container image
- No ConfigMap volume mount is required for runtime config (though it can still be used for bootstrap)

### Granular Configuration Updates

With the file watcher pattern, every change replaces the entire config file. There is no mechanism to express "change only this one field" — the ConfigMap contains the full file, and the application reloads the whole thing.

With a config API, the application can expose fine-grained endpoints that accept partial updates. Depending on the API design:

- `PUT /config` — full replacement (same granularity as file-based)
- `PATCH /config` — merge only the fields provided, leave the rest unchanged
- `PUT /config/logging/level` — update a single setting by path

The generated operator maps these directly. Each API path becomes a CR field, and the operator only sends what the CR specifies. This gives operators precise control over individual settings without risk of accidentally overwriting unrelated configuration.

### Rollback by CR Deletion

The operator pattern provides a clean rollback mechanism that the file watcher pattern does not: **delete the CR to revert to the application's initial state.**

```
With CR:     Operator continuously reconciles app config toward CR spec
Delete CR:   Operator's finalizer calls the config API to reset runtime settings
             back to the application's startup defaults → immediate rollback, no pod restart
```

This is not possible with the ConfigMap pattern. Deleting a ConfigMap that is mounted as a volume would cause the mount to become empty, breaking the application entirely. Reverting requires restoring the original ConfigMap content — and knowing what that content was.

With the operator pattern, the startup config file is always the known-good baseline. The CR represents overrides. Removing the CR triggers the finalizer, which resets the application to its startup configuration via the API — without a pod restart. This gives operators a safe, one-command rollback: `kubectl delete appconfig my-app-config`.

---

## Decision Flowchart

```
Application reads config file at startup. What else can it do?

  Can it watch the filesystem and hot-reload?
  ├── YES ──→ Type B: Use DCC pattern (ConfigMap + file watcher)
  │           Works today, no changes needed.
  │
  └── NO

  Does it expose a config API?
  ├── YES ──→ Type C: Use openapi-operator-gen (CR + generated operator)
  │           Works today, no changes needed.
  │
  └── NO ──→ Type A: Needs modification for zero-downtime config.
             │
             │  RECOMMENDATION: Add a config API (→ Type C)
             │  - Less code (~20-50 lines vs ~200-500)
             │  - No new dependencies or background threads
             │  - Reuses existing config struct and parsing logic
             │  - Unlocks immediate delivery, drift detection, CRD validation
             │
             │  Alternative: Add file watcher (→ Type B)
             │  - More code and complexity
             │  - Requires concurrency management
             │  - Limited to 2 min latency, no drift detection
             │  - Appropriate only if HTTP is not available in the runtime
```

---

## Summary

| Application Type | Existing Capabilities | Pattern | Changes Required |
|-----------------|----------------------|---------|-----------------|
| **Type A** | Startup config file only | RollingUpdate (no zero-downtime) | Add config API (recommended) or file watcher |
| **Type B** | Startup config + file watcher | DCC pattern (ConfigMap) | None |
| **Type C** | Startup config + config API | openapi-operator-gen (CR + operator) | None |
| **Type D** | Startup config + both | openapi-operator-gen (preferred) | None |

For Type A applications, the config API path is recommended because it requires less application code, no new concurrency, no new dependencies, reuses existing config parsing logic, and unlocks a strictly more capable pattern (immediate delivery, drift detection, CRD validation, GitOps-native custom resources, granular field-level updates, and safe rollback to initial state by CR deletion).
