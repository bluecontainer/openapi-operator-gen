# ArgoCD Integration Options

This document explores integration options between openapi-operator-gen and ArgoCD, extending the [ArgoCD Application Pattern Analysis](argocd-application-pattern-analysis.md) with practical integration scenarios.

## Table of Contents

- [Overview](#overview)
- [Integration Architecture](#integration-architecture)
- [Custom Health Checks for Generated CRDs](#custom-health-checks-for-generated-crds)
- [ApplicationSet for Multi-Cluster Deployment](#applicationset-for-multi-cluster-deployment)
- [Progressive Delivery with Argo Rollouts](#progressive-delivery-with-argo-rollouts)
- [Sync Waves and Hooks](#sync-waves-and-hooks)
- [Notifications Integration](#notifications-integration)
- [Resource Customizations](#resource-customizations)
- [Generated Artifacts for ArgoCD](#generated-artifacts-for-argocd)
- [Implementation Roadmap](#implementation-roadmap)

## Overview

ArgoCD provides GitOps-based continuous delivery for Kubernetes. Generated operators from openapi-operator-gen can integrate with ArgoCD at multiple levels:

| Integration Level | Description | Benefit |
|-------------------|-------------|---------|
| **Deployment** | ArgoCD deploys the generated operator | GitOps lifecycle management |
| **Health Checks** | Custom Lua scripts for CRD health | Accurate sync status in ArgoCD UI |
| **ApplicationSet** | Multi-cluster operator deployment | Consistent operators across environments |
| **Progressive Delivery** | Canary/Blue-Green for operator updates | Safe operator rollouts |
| **Notifications** | Webhook events on CRD state changes | External system integration |
| **Sync Waves** | Ordered deployment of operator + CRs | Dependency management |

## Integration Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              ARGOCD CONTROL PLANE                               │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                         ApplicationSet                                   │   │
│  │  generators:                                                             │   │
│  │    - clusters: {}  # Deploy to all registered clusters                  │   │
│  │  template:                                                               │   │
│  │    spec:                                                                 │   │
│  │      source:                                                             │   │
│  │        repoURL: github.com/org/petstore-operator                         │   │
│  │        path: config/                                                     │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                      │                                          │
│                                      │ Generates Applications                   │
│                                      ▼                                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                          │
│  │ Application  │  │ Application  │  │ Application  │                          │
│  │ (cluster-1)  │  │ (cluster-2)  │  │ (cluster-3)  │                          │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘                          │
│         │                 │                 │                                   │
└─────────┼─────────────────┼─────────────────┼───────────────────────────────────┘
          │                 │                 │
          ▼                 ▼                 ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│   Cluster 1     │ │   Cluster 2     │ │   Cluster 3     │
│                 │ │                 │ │                 │
│ ┌─────────────┐ │ │ ┌─────────────┐ │ │ ┌─────────────┐ │
│ │  Operator   │ │ │ │  Operator   │ │ │ │  Operator   │ │
│ │ Deployment  │ │ │ │ Deployment  │ │ │ │ Deployment  │ │
│ └──────┬──────┘ │ │ └──────┬──────┘ │ │ └──────┬──────┘ │
│        │        │ │        │        │ │        │        │
│        ▼        │ │        ▼        │ │        ▼        │
│ ┌─────────────┐ │ │ ┌─────────────┐ │ │ ┌─────────────┐ │
│ │  Pet CRD    │ │ │ │  Pet CRD    │ │ │ │  Pet CRD    │ │
│ │  Order CRD  │ │ │ │  Order CRD  │ │ │ │  Order CRD  │ │
│ │  Bundle CRD │ │ │ │  Bundle CRD │ │ │ │  Bundle CRD │ │
│ └─────────────┘ │ │ └─────────────┘ │ │ └─────────────┘ │
│                 │ │                 │ │                 │
│    ┌────────┐   │ │    ┌────────┐   │ │    ┌────────┐   │
│    │ Health │   │ │    │ Health │   │ │    │ Health │   │
│    │ Check  │   │ │    │ Check  │   │ │    │ Check  │   │
│    │ (Lua)  │   │ │    │ (Lua)  │   │ │    │ (Lua)  │   │
│    └────────┘   │ │    └────────┘   │ │    └────────┘   │
└─────────────────┘ └─────────────────┘ └─────────────────┘
```

## Custom Health Checks for Generated CRDs

ArgoCD uses Lua scripts to determine the health of custom resources. Generated operators should include health check scripts that ArgoCD can use.

### Health Check Script Structure

For each generated CRD type, create a corresponding Lua health check:

**Resource CRDs (Pet, Order, User):**

```lua
-- health.lua for petstore.example.com/Pet
local health_status = {}

if obj.status == nil then
  health_status.status = "Progressing"
  health_status.message = "Waiting for status"
  return health_status
end

local state = obj.status.state

if state == "Synced" then
  health_status.status = "Healthy"
  health_status.message = "Resource synced with REST API"
elseif state == "Pending" or state == "Syncing" then
  health_status.status = "Progressing"
  health_status.message = obj.status.message or "Sync in progress"
elseif state == "Failed" then
  health_status.status = "Degraded"
  health_status.message = obj.status.message or "Sync failed"
elseif state == "NotFound" then
  health_status.status = "Missing"
  health_status.message = "Resource not found in REST API"
else
  health_status.status = "Unknown"
  health_status.message = "Unknown state: " .. (state or "nil")
end

return health_status
```

**Query CRDs (PetFindbystatusQuery):**

```lua
-- health.lua for petstore.example.com/PetFindbystatusQuery
local health_status = {}

if obj.status == nil then
  health_status.status = "Progressing"
  health_status.message = "Waiting for initial query"
  return health_status
end

local state = obj.status.state

if state == "Queried" or state == "Observed" then
  health_status.status = "Healthy"
  health_status.message = "Query completed successfully"
elseif state == "Querying" then
  health_status.status = "Progressing"
  health_status.message = "Query in progress"
elseif state == "Failed" then
  health_status.status = "Degraded"
  health_status.message = obj.status.message or "Query failed"
else
  health_status.status = "Unknown"
  health_status.message = "Unknown state: " .. (state or "nil")
end

return health_status
```

**Action CRDs (PetUploadimageAction):**

```lua
-- health.lua for petstore.example.com/PetUploadimageAction
local health_status = {}

if obj.status == nil then
  health_status.status = "Progressing"
  health_status.message = "Waiting for execution"
  return health_status
end

local state = obj.status.state

if state == "Completed" or state == "Succeeded" then
  health_status.status = "Healthy"
  health_status.message = "Action completed successfully"
elseif state == "Executing" or state == "Pending" then
  health_status.status = "Progressing"
  health_status.message = obj.status.message or "Action in progress"
elseif state == "Failed" then
  health_status.status = "Degraded"
  health_status.message = obj.status.message or "Action failed"
else
  health_status.status = "Unknown"
  health_status.message = "Unknown state: " .. (state or "nil")
end

return health_status
```

**Bundle CRDs:**

```lua
-- health.lua for petstore.example.com/PetstoreBundle
local health_status = {}

if obj.status == nil then
  health_status.status = "Progressing"
  health_status.message = "Waiting for bundle initialization"
  return health_status
end

local state = obj.status.state

if state == "Synced" then
  -- Check if all resources are healthy
  local summary = obj.status.summary
  if summary and summary.failed > 0 then
    health_status.status = "Degraded"
    health_status.message = string.format("%d of %d resources failed", summary.failed, summary.total)
  elseif summary and summary.pending > 0 then
    health_status.status = "Progressing"
    health_status.message = string.format("%d of %d resources pending", summary.pending, summary.total)
  else
    health_status.status = "Healthy"
    health_status.message = "All bundle resources synced"
  end
elseif state == "Syncing" or state == "Pending" then
  health_status.status = "Progressing"
  local summary = obj.status.summary
  if summary then
    health_status.message = string.format("Syncing: %d/%d complete", summary.synced or 0, summary.total or 0)
  else
    health_status.message = "Bundle sync in progress"
  end
elseif state == "Failed" then
  health_status.status = "Degraded"
  health_status.message = obj.status.message or "Bundle sync failed"
elseif state == "Paused" then
  health_status.status = "Suspended"
  health_status.message = "Bundle reconciliation paused"
else
  health_status.status = "Unknown"
  health_status.message = "Unknown state: " .. (state or "nil")
end

return health_status
```

**Aggregate CRDs:**

```lua
-- health.lua for petstore.example.com/PetstoreAggregate
local health_status = {}

if obj.status == nil then
  health_status.status = "Progressing"
  health_status.message = "Waiting for aggregation"
  return health_status
end

-- Use aggregated health if available
if obj.status.aggregatedHealth then
  local aggHealth = obj.status.aggregatedHealth.status
  if aggHealth == "Healthy" then
    health_status.status = "Healthy"
    health_status.message = "All observed resources healthy"
  elseif aggHealth == "Degraded" then
    health_status.status = "Degraded"
    health_status.message = obj.status.aggregatedHealth.message or "Some resources degraded"
  elseif aggHealth == "Progressing" then
    health_status.status = "Progressing"
    health_status.message = "Resources syncing"
  else
    health_status.status = "Unknown"
    health_status.message = "Unknown aggregated health"
  end
  return health_status
end

-- Fallback to summary-based health
local summary = obj.status.summary
if summary then
  if summary.failed > 0 then
    health_status.status = "Degraded"
    health_status.message = string.format("%d resources failed", summary.failed)
  elseif summary.pending > 0 then
    health_status.status = "Progressing"
    health_status.message = string.format("%d resources pending", summary.pending)
  else
    health_status.status = "Healthy"
    health_status.message = "All resources synced"
  end
else
  health_status.status = "Unknown"
  health_status.message = "No status summary available"
end

return health_status
```

### ArgoCD ConfigMap Configuration

Add health checks to `argocd-cm` ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  resource.customizations.health.petstore.example.com_Pet: |
    hs = {}
    if obj.status == nil then
      hs.status = "Progressing"
      hs.message = "Waiting for status"
      return hs
    end
    if obj.status.state == "Synced" then
      hs.status = "Healthy"
      hs.message = "Resource synced"
    elseif obj.status.state == "Failed" then
      hs.status = "Degraded"
      hs.message = obj.status.message or "Sync failed"
    else
      hs.status = "Progressing"
      hs.message = "Syncing"
    end
    return hs

  resource.customizations.health.petstore.example.com_Order: |
    -- Same pattern as Pet
    ...

  resource.customizations.health.petstore.example.com_PetstoreBundle: |
    -- Bundle health check (see above)
    ...
```

### Wildcard Health Check

For operators with many CRD kinds, use a wildcard pattern:

```yaml
data:
  # Matches all CRDs in petstore.example.com group
  resource.customizations.health.petstore.example.com_*: |
    hs = {}
    if obj.status == nil then
      hs.status = "Progressing"
      hs.message = "Waiting for status"
      return hs
    end
    local state = obj.status.state
    if state == "Synced" or state == "Queried" or state == "Completed" then
      hs.status = "Healthy"
    elseif state == "Failed" then
      hs.status = "Degraded"
    elseif state == "Paused" then
      hs.status = "Suspended"
    else
      hs.status = "Progressing"
    end
    hs.message = obj.status.message or state
    return hs
```

## ApplicationSet for Multi-Cluster Deployment

Deploy generated operators across multiple clusters using ApplicationSet.

### Cluster Generator Pattern

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: petstore-operator
  namespace: argocd
spec:
  generators:
    # Deploy to all clusters with 'operator-enabled' label
    - clusters:
        selector:
          matchLabels:
            operator-enabled: "true"

  template:
    metadata:
      name: 'petstore-operator-{{name}}'
      namespace: argocd
    spec:
      project: default
      source:
        repoURL: https://github.com/org/petstore-operator
        targetRevision: HEAD
        path: config/
        # Or use Helm chart
        # path: chart/petstore
        # helm:
        #   valueFiles:
        #     - values-{{name}}.yaml
      destination:
        server: '{{server}}'
        namespace: petstore-operator-system
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        syncOptions:
          - CreateNamespace=true
```

### Matrix Generator: Clusters x Environments

Deploy operators with environment-specific configuration:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: petstore-operator-matrix
  namespace: argocd
spec:
  generators:
    - matrix:
        generators:
          # Cluster generator
          - clusters:
              selector:
                matchLabels:
                  tier: production
          # Git generator for environments
          - git:
              repoURL: https://github.com/org/petstore-operator
              revision: HEAD
              directories:
                - path: environments/*

  template:
    metadata:
      name: 'petstore-{{path.basename}}-{{name}}'
    spec:
      source:
        repoURL: https://github.com/org/petstore-operator
        path: 'environments/{{path.basename}}'
        helm:
          values: |
            targetCluster: {{name}}
            environment: {{path.basename}}
      destination:
        server: '{{server}}'
        namespace: 'petstore-{{path.basename}}'
```

### Pull Request Generator for Operator Testing

Test operator changes before merging:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: petstore-operator-pr-preview
  namespace: argocd
spec:
  generators:
    - pullRequest:
        github:
          owner: org
          repo: petstore-operator
          labels:
            - preview
        requeueAfterSeconds: 60

  template:
    metadata:
      name: 'petstore-pr-{{number}}'
    spec:
      source:
        repoURL: 'https://github.com/org/petstore-operator'
        targetRevision: '{{head_sha}}'
        path: config/
      destination:
        server: https://kubernetes.default.svc
        namespace: 'petstore-pr-{{number}}'
      syncPolicy:
        automated:
          prune: true
```

## Progressive Delivery with Argo Rollouts

Use Argo Rollouts for safe operator deployments.

### Blue-Green Operator Deployment

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: petstore-operator
  namespace: petstore-operator-system
spec:
  replicas: 2
  selector:
    matchLabels:
      app: petstore-operator
  template:
    metadata:
      labels:
        app: petstore-operator
    spec:
      serviceAccountName: petstore-controller-manager
      containers:
        - name: manager
          image: petstore-operator:latest
          args:
            - --leader-elect
          ports:
            - containerPort: 8080
              name: metrics
          resources:
            limits:
              cpu: 500m
              memory: 128Mi

  strategy:
    blueGreen:
      activeService: petstore-operator-active
      previewService: petstore-operator-preview
      autoPromotionEnabled: false
      # Verify CRD health before promotion
      prePromotionAnalysis:
        templates:
          - templateName: crd-health-check
        args:
          - name: service-name
            value: petstore-operator-preview
```

### Canary Operator Deployment

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: petstore-operator
  namespace: petstore-operator-system
spec:
  replicas: 3
  selector:
    matchLabels:
      app: petstore-operator
  template:
    # ... same as above

  strategy:
    canary:
      steps:
        - setWeight: 20
        - pause: {duration: 5m}
        - setWeight: 50
        - pause: {duration: 5m}
        - setWeight: 80
        - pause: {duration: 5m}

      # Verify reconciliation success rate during rollout
      analysis:
        templates:
          - templateName: reconciliation-success-rate
        startingStep: 1
        args:
          - name: operator-namespace
            value: petstore-operator-system
```

### Analysis Template for Operator Health

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AnalysisTemplate
metadata:
  name: reconciliation-success-rate
spec:
  args:
    - name: operator-namespace
  metrics:
    - name: reconcile-success-rate
      interval: 1m
      successCondition: result[0] >= 0.95
      failureLimit: 3
      provider:
        prometheus:
          address: http://prometheus:9090
          query: |
            sum(rate(controller_runtime_reconcile_total{
              controller=~"pet|order|user",
              result="success",
              namespace="{{args.operator-namespace}}"
            }[5m])) /
            sum(rate(controller_runtime_reconcile_total{
              controller=~"pet|order|user",
              namespace="{{args.operator-namespace}}"
            }[5m]))

    - name: error-rate
      interval: 1m
      successCondition: result[0] <= 0.01
      failureLimit: 2
      provider:
        prometheus:
          address: http://prometheus:9090
          query: |
            sum(rate(controller_runtime_reconcile_errors_total{
              namespace="{{args.operator-namespace}}"
            }[5m])) /
            sum(rate(controller_runtime_reconcile_total{
              namespace="{{args.operator-namespace}}"
            }[5m]))
```

## Sync Waves and Hooks

Order operator deployment using sync waves.

### Operator Deployment Order

```yaml
# Wave -2: Namespace
apiVersion: v1
kind: Namespace
metadata:
  name: petstore-operator-system
  annotations:
    argocd.argoproj.io/sync-wave: "-2"
---
# Wave -1: CRDs (must exist before operator starts)
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: pets.petstore.example.com
  annotations:
    argocd.argoproj.io/sync-wave: "-1"
spec:
  # ...
---
# Wave 0: RBAC
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: petstore-manager-role
  annotations:
    argocd.argoproj.io/sync-wave: "0"
# ...
---
# Wave 1: Operator Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: petstore-controller-manager
  namespace: petstore-operator-system
  annotations:
    argocd.argoproj.io/sync-wave: "1"
spec:
  # ...
---
# Wave 2: Sample CRs (after operator is ready)
apiVersion: petstore.example.com/v1alpha1
kind: Pet
metadata:
  name: sample-pet
  annotations:
    argocd.argoproj.io/sync-wave: "2"
spec:
  name: "Fluffy"
```

### PreSync Hook: Validate API Connectivity

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: validate-api-connectivity
  annotations:
    argocd.argoproj.io/hook: PreSync
    argocd.argoproj.io/hook-delete-policy: HookSucceeded
spec:
  template:
    spec:
      containers:
        - name: validate
          image: curlimages/curl:latest
          command:
            - /bin/sh
            - -c
            - |
              echo "Validating Petstore API connectivity..."
              curl -sf "${PETSTORE_API_URL}/pet/1" || exit 1
              echo "API is reachable"
          env:
            - name: PETSTORE_API_URL
              valueFrom:
                secretKeyRef:
                  name: petstore-api-config
                  key: base-url
      restartPolicy: Never
  backoffLimit: 3
```

### PostSync Hook: Verify CRD Health

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: verify-crd-health
  annotations:
    argocd.argoproj.io/hook: PostSync
    argocd.argoproj.io/hook-delete-policy: HookSucceeded
spec:
  template:
    spec:
      serviceAccountName: crd-health-checker
      containers:
        - name: verify
          image: bitnami/kubectl:latest
          command:
            - /bin/sh
            - -c
            - |
              echo "Waiting for operator to be ready..."
              kubectl wait --for=condition=available \
                deployment/petstore-controller-manager \
                -n petstore-operator-system \
                --timeout=120s

              echo "Verifying CRDs are registered..."
              kubectl get crd pets.petstore.example.com || exit 1
              kubectl get crd orders.petstore.example.com || exit 1

              echo "All CRDs verified"
      restartPolicy: Never
  backoffLimit: 2
```

### SyncFail Hook: Cleanup on Failure

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: cleanup-on-failure
  annotations:
    argocd.argoproj.io/hook: SyncFail
    argocd.argoproj.io/hook-delete-policy: HookSucceeded
spec:
  template:
    spec:
      serviceAccountName: cleanup-sa
      containers:
        - name: cleanup
          image: bitnami/kubectl:latest
          command:
            - /bin/sh
            - -c
            - |
              echo "Sync failed, cleaning up orphaned resources..."
              # Delete any CRs that might be in bad state
              kubectl delete pets.petstore.example.com --all \
                -n petstore-operator-system \
                --ignore-not-found
              echo "Cleanup complete"
      restartPolicy: Never
```

## Notifications Integration

Configure ArgoCD to send notifications on CRD state changes.

### Notification Configuration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
  namespace: argocd
data:
  # Webhook service for external integrations
  service.webhook.petstore-events: |
    url: https://events.example.com/argocd
    headers:
      - name: Authorization
        value: Bearer $webhook-token

  # Slack integration
  service.slack: |
    token: $slack-token

  # Triggers for CRD events
  trigger.on-crd-synced: |
    - when: app.status.resources[*].health.status == 'Healthy'
      send: [crd-synced]

  trigger.on-crd-degraded: |
    - when: app.status.resources[*].health.status == 'Degraded'
      send: [crd-degraded]

  trigger.on-bundle-complete: |
    - when: |
        app.status.resources[?(@.kind == 'PetstoreBundle')].health.status == 'Healthy'
      send: [bundle-complete]

  # Notification templates
  template.crd-synced: |
    webhook:
      petstore-events:
        method: POST
        body: |
          {
            "event": "crd-synced",
            "application": "{{.app.metadata.name}}",
            "resources": {{.app.status.resources | toJson}},
            "timestamp": "{{.app.status.operationState.finishedAt}}"
          }
    slack:
      attachments: |
        [{
          "color": "#18be52",
          "title": "CRDs Synced: {{.app.metadata.name}}",
          "text": "All CRDs are now synced with the REST API",
          "fields": [
            {"title": "Application", "value": "{{.app.metadata.name}}", "short": true},
            {"title": "Cluster", "value": "{{.app.spec.destination.server}}", "short": true}
          ]
        }]

  template.crd-degraded: |
    webhook:
      petstore-events:
        method: POST
        body: |
          {
            "event": "crd-degraded",
            "application": "{{.app.metadata.name}}",
            "message": "{{.app.status.operationState.message}}",
            "resources": {{.app.status.resources | toJson}}
          }
    slack:
      attachments: |
        [{
          "color": "#E96D76",
          "title": "CRD Degraded: {{.app.metadata.name}}",
          "text": "One or more CRDs have failed to sync",
          "fields": [
            {"title": "Message", "value": "{{.app.status.operationState.message}}", "short": false}
          ]
        }]

  template.bundle-complete: |
    webhook:
      petstore-events:
        method: POST
        body: |
          {
            "event": "bundle-complete",
            "application": "{{.app.metadata.name}}",
            "bundle": "{{(index .app.status.resources 0).name}}"
          }
```

### Subscribing Applications to Notifications

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: petstore-operator
  namespace: argocd
  annotations:
    # Subscribe to notifications
    notifications.argoproj.io/subscribe.on-crd-synced.slack: petstore-alerts
    notifications.argoproj.io/subscribe.on-crd-degraded.slack: petstore-alerts
    notifications.argoproj.io/subscribe.on-crd-degraded.webhook.petstore-events: ""
spec:
  # ...
```

## Resource Customizations

Configure ArgoCD to handle generated CRDs properly.

### Ignore Differences

Ignore status and controller-managed fields:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  resource.customizations.ignoreDifferences.petstore.example.com_Pet: |
    jqPathExpressions:
      - .status
      - .metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"]
      - .metadata.generation
      - .metadata.resourceVersion
      - .metadata.uid

  resource.customizations.ignoreDifferences.petstore.example.com_PetstoreBundle: |
    jqPathExpressions:
      - .status
      # Ignore child resource names (generated by controller)
      - .status.resources[].name
```

### Custom Actions

Add custom actions for CRDs in ArgoCD UI:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  resource.customizations.actions.petstore.example.com_Pet: |
    discovery.lua: |
      actions = {}
      actions["force-sync"] = {
        ["disabled"] = obj.status.state == "Syncing"
      }
      actions["pause"] = {
        ["disabled"] = obj.spec.paused == true
      }
      actions["resume"] = {
        ["disabled"] = obj.spec.paused ~= true
      }
      return actions
    definitions:
      - name: force-sync
        action.lua: |
          obj.metadata.annotations = obj.metadata.annotations or {}
          obj.metadata.annotations["petstore.example.com/force-sync"] = os.date("!%Y-%m-%dT%H:%M:%SZ")
          return obj
      - name: pause
        action.lua: |
          obj.spec.paused = true
          return obj
      - name: resume
        action.lua: |
          obj.spec.paused = false
          return obj

  resource.customizations.actions.petstore.example.com_PetstoreBundle: |
    discovery.lua: |
      actions = {}
      actions["pause-bundle"] = {["disabled"] = obj.spec.paused == true}
      actions["resume-bundle"] = {["disabled"] = obj.spec.paused ~= true}
      actions["retry-failed"] = {
        ["disabled"] = obj.status == nil or obj.status.summary == nil or obj.status.summary.failed == 0
      }
      return actions
    definitions:
      - name: pause-bundle
        action.lua: |
          obj.spec.paused = true
          return obj
      - name: resume-bundle
        action.lua: |
          obj.spec.paused = false
          return obj
      - name: retry-failed
        action.lua: |
          obj.metadata.annotations = obj.metadata.annotations or {}
          obj.metadata.annotations["petstore.example.com/retry-failed"] = os.date("!%Y-%m-%dT%H:%M:%SZ")
          return obj
```

## Generated Artifacts for ArgoCD

The generator could produce ArgoCD-specific artifacts.

### Proposed Generator Output

```
generated/
├── config/                      # Existing kustomize manifests
├── chart/                       # Existing Helm chart
└── argocd/                      # NEW: ArgoCD-specific artifacts
    ├── health-checks/
    │   ├── pet.lua
    │   ├── order.lua
    │   ├── petfindbystatusquery.lua
    │   ├── petuploadimageaction.lua
    │   ├── petstoreaggregate.lua
    │   └── petstorebundle.lua
    ├── resource-customizations/
    │   ├── ignore-differences.yaml
    │   └── actions.yaml
    ├── applicationset.yaml       # Template ApplicationSet
    ├── notifications.yaml        # Notification templates
    └── kustomization.yaml        # Patch for argocd-cm
```

### Generator Flag

```bash
openapi-operator-gen \
  --spec petstore.yaml \
  --output ./generated \
  --argocd                       # NEW: Generate ArgoCD artifacts
```

### Generated ApplicationSet Template

```yaml
# generated/argocd/applicationset.yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: petstore-operator
  namespace: argocd
spec:
  generators:
    - clusters:
        selector:
          matchLabels:
            petstore-operator: enabled
  template:
    metadata:
      name: 'petstore-operator-{{name}}'
    spec:
      project: default
      source:
        repoURL: <REPO_URL>  # Replace with your repository
        targetRevision: HEAD
        path: config/
      destination:
        server: '{{server}}'
        namespace: petstore-operator-system
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        syncOptions:
          - CreateNamespace=true
          - ServerSideApply=true
```

## Implementation Roadmap

| Phase | Feature | Complexity | Value |
|-------|---------|------------|-------|
| **1** | Generate Lua health checks for all CRD types | Low | High |
| **2** | Generate argocd-cm patch with health checks | Low | High |
| **3** | Add `--argocd` flag to generator | Low | Medium |
| **4** | Generate ApplicationSet template | Low | Medium |
| **5** | Generate notification templates | Medium | Medium |
| **6** | Generate resource actions | Medium | Low |
| **7** | Argo Rollouts templates | Medium | Medium |

### Code Changes Required

**1. New template files:**
```
pkg/templates/argocd/
├── health_resource.lua.tmpl
├── health_query.lua.tmpl
├── health_action.lua.tmpl
├── health_bundle.lua.tmpl
├── health_aggregate.lua.tmpl
├── applicationset.yaml.tmpl
├── notifications.yaml.tmpl
└── argocd-cm-patch.yaml.tmpl
```

**2. Generator extension:**
```go
// pkg/generator/argocd.go
func (g *Generator) GenerateArgoCDArtifacts(defs []*mapper.CRDDefinition) error {
    // Generate health checks
    // Generate ApplicationSet template
    // Generate argocd-cm patch
    // Generate notifications config
}
```

**3. CLI flag:**
```go
// cmd/openapi-operator-gen/main.go
var argoCDFlag = flag.Bool("argocd", false, "Generate ArgoCD integration artifacts")
```

## Summary

ArgoCD integration provides significant value for generated operators:

| Integration | Benefit |
|-------------|---------|
| **Health Checks** | Accurate operator status in ArgoCD UI |
| **ApplicationSet** | Multi-cluster operator deployment |
| **Sync Waves** | Proper deployment ordering |
| **Hooks** | Pre/post deployment validation |
| **Notifications** | External system integration |
| **Progressive Delivery** | Safe operator updates |

The generator could produce all necessary ArgoCD artifacts, making it trivial to deploy and manage generated operators via GitOps.

## References

- [ArgoCD Resource Health](https://argo-cd.readthedocs.io/en/latest/operator-manual/health/)
- [ArgoCD ApplicationSet](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/)
- [ArgoCD Sync Waves and Hooks](https://argo-cd.readthedocs.io/en/stable/user-guide/sync-waves/)
- [ArgoCD Notifications](https://argo-cd.readthedocs.io/en/stable/operator-manual/notifications/)
- [Argo Rollouts](https://argoproj.github.io/argo-rollouts/)
- [ArgoCD Custom Health Checks](https://www.patrickdap.com/post/argocd-health-checks/)
