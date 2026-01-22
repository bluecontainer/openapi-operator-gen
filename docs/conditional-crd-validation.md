# Conditional CRD Field Validation

> **Status**: ✅ Implemented in openapi-operator-gen

This document analyzes approaches for conditionally requiring CRD fields based on whether a resource is being created new or referencing an existing external resource.

## Problem Statement

When generating CRDs from OpenAPI specs, fields marked as `required` in the OpenAPI schema become required in the CRD. However, there are scenarios where these fields should become optional:

1. **Path Parameter Resources**: When a path like `/pet/{petId}` is used, specifying `petId` means you're referencing an existing resource, so other fields like `name`, `category`, etc. should become optional.

2. **POST-Created Resources**: When `externalIDRef` is set (referencing an existing resource by its external ID), the fields required for creation should become optional.

## Solution: CEL Validation Rules

Kubernetes 1.25+ supports CEL (Common Expression Language) validation rules via `x-kubernetes-validations` markers. This is the cleanest approach as it keeps validation declarative and in the CRD itself.

### Scenario 1: Path Parameter Resources

For resources identified by path parameters (e.g., `/pet/{petId}`):

```go
// +kubebuilder:validation:XValidation:rule="has(self.petId) || has(self.name)",message="name is required when petId is not specified"
// +kubebuilder:validation:XValidation:rule="has(self.petId) || has(self.category)",message="category is required when petId is not specified"
// +kubebuilder:validation:XValidation:rule="has(self.petId) || has(self.photoUrls)",message="photoUrls is required when petId is not specified"
type PetSpec struct {
    // PetId references an existing pet - when set, other fields become optional
    // +optional
    PetId *int64 `json:"petId,omitempty"`

    // Name of the pet - required for creation, optional when petId is set
    // +optional
    Name *string `json:"name,omitempty"`

    // Category of the pet - required for creation, optional when petId is set
    // +optional
    Category *Category `json:"category,omitempty"`

    // PhotoUrls for the pet - required for creation, optional when petId is set
    // +optional
    PhotoUrls []string `json:"photoUrls,omitempty"`
}
```

### Scenario 2: ExternalIDRef Resources

For resources that support referencing by external ID:

```go
// +kubebuilder:validation:XValidation:rule="has(self.externalIDRef) || has(self.name)",message="name is required when externalIDRef is not specified"
// +kubebuilder:validation:XValidation:rule="has(self.externalIDRef) || has(self.category)",message="category is required when externalIDRef is not specified"
type PetSpec struct {
    // ExternalIDRef references an existing resource by its external API ID
    // +optional
    ExternalIDRef *string `json:"externalIDRef,omitempty"`

    // Name - required for creation, optional when referencing existing
    // +optional
    Name *string `json:"name,omitempty"`

    // Category - required for creation, optional when referencing existing
    // +optional
    Category *Category `json:"category,omitempty"`
}
```

### Combined Scenario

When both path parameters AND externalIDRef might be present:

```go
// +kubebuilder:validation:XValidation:rule="has(self.petId) || has(self.externalIDRef) || has(self.name)",message="name is required when neither petId nor externalIDRef is specified"
type PetSpec struct {
    // +optional
    PetId *int64 `json:"petId,omitempty"`

    // +optional
    ExternalIDRef *string `json:"externalIDRef,omitempty"`

    // +optional
    Name *string `json:"name,omitempty"`
}
```

## Alternative Approaches

### 1. Validating Admission Webhook

A webhook can implement complex conditional logic but requires:
- Additional deployment (webhook server)
- TLS certificate management
- More operational complexity

### 2. Controller-Level Validation

Validate in the controller's `Reconcile` function:
- Simpler to implement
- But allows invalid resources to be created (fails at reconcile time, not admission time)
- User experience is worse (delayed error feedback)

## Implementation Considerations for the Generator

To implement this in `openapi-operator-gen`, the generator would need to:

1. **Track Field Origin**: Distinguish between:
   - Fields from OpenAPI `required` array (creation requirements)
   - Fields added by the generator (e.g., `externalIDRef`, targeting fields)

2. **Identify Path Parameters**: For paths like `/pet/{petId}`:
   - Extract path parameter names
   - Generate CEL rules that make creation fields optional when path param is set

3. **Generate CEL Markers**: For each OpenAPI-required field, generate:
   ```go
   // +kubebuilder:validation:XValidation:rule="has(self.{pathParam}) || has(self.{field})",message="{field} is required when {pathParam} is not specified"
   ```

4. **Mark All Fields Optional**: Change from:
   ```go
   Name string `json:"name"`  // required
   ```
   To:
   ```go
   // +optional
   Name *string `json:"name,omitempty"`  // optional with CEL validation
   ```

## CEL Expression Reference

Useful CEL functions for validation:

| Expression | Description |
|------------|-------------|
| `has(self.field)` | Returns true if field is set (non-null) |
| `self.field != ""` | Check string is non-empty |
| `size(self.list) > 0` | Check list is non-empty |
| `self.a \|\| self.b` | Logical OR |
| `self.a && self.b` | Logical AND |
| `!has(self.field)` | Field is not set |

## Example: Full Pet CRD with Conditional Validation

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: pets.petstore.example.com
spec:
  group: petstore.example.com
  names:
    kind: Pet
    listKind: PetList
    plural: pets
    singular: pet
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              x-kubernetes-validations:
                - rule: "has(self.petId) || has(self.externalIDRef) || has(self.name)"
                  message: "name is required when neither petId nor externalIDRef is specified"
                - rule: "has(self.petId) || has(self.externalIDRef) || has(self.photoUrls)"
                  message: "photoUrls is required when neither petId nor externalIDRef is specified"
              properties:
                petId:
                  type: integer
                  format: int64
                externalIDRef:
                  type: string
                name:
                  type: string
                category:
                  type: object
                  properties:
                    id:
                      type: integer
                    name:
                      type: string
                photoUrls:
                  type: array
                  items:
                    type: string
                status:
                  type: string
                  enum: ["available", "pending", "sold"]
                tags:
                  type: array
                  items:
                    type: object
                    properties:
                      id:
                        type: integer
                      name:
                        type: string
```

## Implementation in openapi-operator-gen

The generator now automatically creates CEL validation rules for Resource CRDs (not Query or Action CRDs) when:

1. The resource has fields marked as `required` in the OpenAPI spec
2. The resource has a way to reference existing resources (path parameters or externalIDRef)

### How It Works

1. **Field Tracking**: The `FieldDefinition` struct has an `OpenAPIRequired` field that tracks whether a field was required in the original OpenAPI spec (vs controller-added fields).

2. **Rule Generation**: The `generateCELValidationRules()` function in `pkg/mapper/resource.go`:
   - Identifies path parameter fields (fields merged with URL path params like `{petId}`)
   - Identifies if `externalIDRef` is available for the resource
   - For each OpenAPI-required field, generates a CEL rule like:
     ```
     has(self.id) || has(self.name)
     ```

3. **Template Output**: The `types.go.tmpl` template emits `// +kubebuilder:validation:XValidation` markers before the Spec struct.

### Generated Example

For the Pet resource from the Petstore API:

```go
// PetSpec defines the desired state of Pet
// +kubebuilder:validation:XValidation:rule="has(self.id) || has(self.name)",message="name is required when creating a new resource"
// +kubebuilder:validation:XValidation:rule="has(self.id) || has(self.photoUrls)",message="photoUrls is required when creating a new resource"
type PetSpec struct {
    // +optional
    Id *int64 `json:"id,omitempty"`

    // +optional
    Name string `json:"name,omitempty"`

    // +optional
    PhotoUrls []string `json:"photoUrls,omitempty"`
    // ...
}
```

This allows users to:
- Create new pets by providing `name` and `photoUrls`
- Reference existing pets by providing just `id`

## References

- [Kubernetes CEL Validation](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules)
- [CEL Language Definition](https://github.com/google/cel-spec)
- [Kubebuilder Validation Markers](https://book.kubebuilder.io/reference/markers/crd-validation.html)
