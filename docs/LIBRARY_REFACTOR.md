# Library Refactor Analysis

This document analyzes which functions from the generated controller templates can be moved to a shared runtime library.

## Functions That CAN Move to Library

| Function | Template | Reason |
|----------|----------|--------|
| `countResults` | query_controller | Generic JSON parsing - no type dependencies |
| `isZeroValue` | action_controller (referenced but not defined) | Generic reflection helper - should be in library |

## Functions That COULD Move with Interface/Refactoring

| Function | Template(s) | Change Required |
|----------|-------------|-----------------|
| `getBaseURL` | all 3 | Extract: `func GetBaseURL(resolver *endpoint.Resolver, staticURL string) (string, error)` |
| `getBaseURLByOrdinal` | all 3 | Extract: `func GetBaseURLByOrdinal(resolver *endpoint.Resolver, staticURL string, ordinal *int32) (string, error)` |
| HTTP execution core | all 3 | Extract: `func DoJSONRequest(ctx, client, method, url, body) ([]byte, int, error)` |
| Drift comparison logic | controller | Extract: `func CompareMapFields(spec, response map[string]interface{}, excludeKeys []string) bool` |

## Functions That CANNOT Move (Type-Specific)

| Function | Template | Why |
|----------|----------|-----|
| `buildResourceURL` | controller | Uses `instance.Spec.{{ .GoName }}` - template-generated field access |
| `buildResourceURLForCreate` | controller | Same |
| `buildQueryURL` | query | Uses reflection on `instance.Spec` with field iteration |
| `buildActionURL` | action | Uses `instance.Spec.{{ .ParentIDField }}` |
| `buildRequestBody` | action | Template-generated field mapping |
| `getExternalID` | controller | Accesses `instance.Spec.ExternalIDRef`, `instance.Status.ExternalID` |
| `resolveBaseURL` | all 3 | Accesses `instance.Spec.TargetHelmRelease/StatefulSet/Deployment/Namespace` |
| `marshalSpecForAPI` | controller | Template-generated `delete(specMap, ...)` calls |
| `getResource` | controller | Uses `buildResourceURL` and type-specific error handling |
| `createResource` | controller | Type-specific status updates |
| `updateResource` | controller | Type-specific status updates |
| `deleteFromEndpoint` | controller | Type-specific |
| `observeResource` | controller | Updates `instance.Status.*` fields |
| `syncToEndpoint` | controller | Complex type-specific orchestration |
| `syncResource` | controller | Type-specific |
| `finalizeResource` | controller | Type-specific |
| `executeQueryToEndpoint` | query | Uses `buildQueryURL` |
| `executeQuery` | query | Type-specific status/response handling |
| `executeActionToEndpoint` | action | Uses `buildActionURL` |
| `executeAction` | action | Type-specific status/response handling |
| `updateStatus` | all 3 | Different signatures, updates type-specific `instance.Status.*` |
| `parseResults` / `parseResult` | query/action | Unmarshals to `{{ .APIVersion }}.{{ .ResultItemType }}` |

## Recommended Library Package

### pkg/runtime/http.go

```go
package runtime

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "reflect"
)

// DoJSONRequest executes an HTTP request and returns response body, status code, and error
func DoJSONRequest(ctx context.Context, client *http.Client, method, url string, body []byte) ([]byte, int, error) {
    var bodyReader io.Reader
    if body != nil {
        bodyReader = bytes.NewReader(body)
    }

    req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
    if err != nil {
        return nil, 0, fmt.Errorf("failed to create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "application/json")

    resp, err := client.Do(req)
    if err != nil {
        return nil, 0, fmt.Errorf("failed to execute request: %w", err)
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
    }

    return respBody, resp.StatusCode, nil
}

// CountJSONResults counts items in a JSON response (array or object with items/data/results)
func CountJSONResults(body []byte) int {
    // Try to parse as array
    var arr []interface{}
    if err := json.Unmarshal(body, &arr); err == nil {
        return len(arr)
    }

    // Try to parse as object with items/data/results field
    var obj map[string]interface{}
    if err := json.Unmarshal(body, &obj); err == nil {
        for _, key := range []string{"items", "data", "results", "content"} {
            if items, ok := obj[key].([]interface{}); ok {
                return len(items)
            }
        }
        // If it's an object, count as 1 result
        return 1
    }

    return 0
}

// IsZeroValue checks if a value is the zero value for its type
func IsZeroValue(v interface{}) bool {
    if v == nil {
        return true
    }
    val := reflect.ValueOf(v)
    return val.IsZero()
}

// CompareMapFields compares two maps, excluding specified keys
// Returns true if there is a difference (drift detected)
func CompareMapFields(spec, response map[string]interface{}, excludeKeys []string) bool {
    // Create a set of keys to exclude
    exclude := make(map[string]bool)
    for _, key := range excludeKeys {
        exclude[key] = true
    }

    // Compare each field in spec with response
    for key, specValue := range spec {
        if exclude[key] {
            continue
        }
        apiValue, exists := response[key]
        if !exists {
            // Field exists in spec but not in response - could be drift or API doesn't return this field
            continue
        }
        if !reflect.DeepEqual(specValue, apiValue) {
            return true // Drift detected
        }
    }

    return false // No drift
}
```

### pkg/runtime/endpoint.go

```go
package runtime

import (
    "fmt"

    "github.com/bluecontainer/openapi-operator-gen/pkg/endpoint"
)

// GetBaseURL returns the base URL from resolver or static URL
func GetBaseURL(resolver *endpoint.Resolver, staticURL string) (string, error) {
    if resolver != nil {
        return resolver.GetEndpoint()
    }
    if staticURL == "" {
        return "", fmt.Errorf("no base URL configured")
    }
    return staticURL, nil
}

// GetBaseURLByOrdinal returns the base URL for a specific pod ordinal
func GetBaseURLByOrdinal(resolver *endpoint.Resolver, staticURL string, ordinal *int32) (string, error) {
    if resolver != nil && resolver.IsByOrdinalStrategy() {
        if ordinal == nil {
            return "", fmt.Errorf("targetPodOrdinal is required when using by-ordinal strategy")
        }
        return resolver.GetEndpointByOrdinal(int(*ordinal))
    }
    return GetBaseURL(resolver, staticURL)
}
```

## Summary

The key insight: **most controller logic is tightly coupled to generated types**, making only ~4-6 helper functions suitable for extraction:

1. `CountJSONResults` - Generic JSON array/object counting
2. `IsZeroValue` - Generic reflection helper
3. `DoJSONRequest` - Generic HTTP request execution
4. `CompareMapFields` - Generic map comparison with exclusions
5. `GetBaseURL` - Endpoint resolution helper
6. `GetBaseURLByOrdinal` - Ordinal-based endpoint resolution

The remaining functions are inherently type-specific because they:
- Access template-generated struct fields (`instance.Spec.{{ .GoName }}`)
- Update type-specific status fields (`instance.Status.*`)
- Unmarshal to generated types (`{{ .APIVersion }}.{{ .ResultItemType }}`)
- Have template-generated delete/exclude lists
