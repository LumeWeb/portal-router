package router

import (
	swagger "go.lumeweb.com/gswagger"
	"net/http"
	"strings"
)

// SwaggerOption defines a function type for modifying swagger.Definitions.
// Used to apply customizations to OpenAPI/Swagger documentation.
type SwaggerOption func(*swagger.Definitions)

// FieldSchema defines the interface for providing field-level schema information
// used to generate sorting and filtering documentation.
type FieldSchema interface {
	SortableFields() []string
	FilterOperators() map[string][]string // field -> []operator
}

// SchemaProvider defines an interface for providing schema information
// based on type.
type SchemaProvider interface {
	ForType(any) FieldSchema
}

// WithSwaggerOptions creates a RouteOption that applies multiple Swagger definition options.
func WithSwaggerOptions(opts ...SwaggerOption) RouteOption {
	return func(d *RouteDefinition) {
		for _, opt := range opts {
			opt(&d.Swagger)
		}
	}
}

// WithRequestBody creates a Swagger option for request body definition.
func WithRequestBody(value interface{}, description string, required bool) SwaggerOption {
	return func(d *swagger.Definitions) {
		d.RequestBody = &swagger.ContentValue{
			Description: description,
			Required:    required,
			Content: map[string]swagger.Schema{
				"application/json": {
					Value: value,
				},
			},
		}
	}
}

// WithFileUpload creates a Swagger option for file upload definition.
func WithFileUpload(description string, required bool) SwaggerOption {
	return func(d *swagger.Definitions) {
		d.RequestBody = &swagger.ContentValue{
			Description: description,
			Required:    required,
			Content: map[string]swagger.Schema{
				"multipart/form-data": {
					Value: struct {
						File string `json:"file" form:"file"`
					}{},
				},
			},
		}
	}
}

// WithArrayResponse creates a Swagger option for array response definition.
func WithArrayResponse(status int, description string, itemValue interface{}) SwaggerOption {
	return func(d *swagger.Definitions) {
		if d.Responses == nil {
			d.Responses = make(map[int]swagger.ContentValue)
		}
		d.Responses[status] = swagger.ContentValue{
			Description: description,
			Content: map[string]swagger.Schema{
				"application/json": {
					Value: struct {
						Items []interface{} `json:"items"`
					}{
						Items: []interface{}{itemValue},
					},
				},
			},
		}
	}
}

// WithResponseHeaders creates a Swagger option for response with headers.
func WithResponseHeaders(status int, description string, content map[string]swagger.Schema, headers map[string]string) SwaggerOption {
	return func(d *swagger.Definitions) {
		if d.Responses == nil {
			d.Responses = make(map[int]swagger.ContentValue)
		}
		d.Responses[status] = swagger.ContentValue{
			Description: description,
			Content:     content,
			Headers:     headers,
		}
	}
}

// WithTags creates a Swagger option for adding tags.
func WithTags(tags ...string) SwaggerOption {
	return func(d *swagger.Definitions) {
		d.Tags = tags
	}
}

// WithSummary creates a Swagger option for setting summary.
func WithSummary(summary string) SwaggerOption {
	return func(d *swagger.Definitions) {
		d.Summary = summary
	}
}

// WithDescription creates a Swagger option for setting description.
func WithDescription(description string) SwaggerOption {
	return func(d *swagger.Definitions) {
		d.Description = description
	}
}

// WithSwagger creates a RouteOption that sets the Swagger documentation definitions
// for a route. Automatically includes appropriate error responses based on access level.
func WithSwagger(opts ...SwaggerOption) RouteOption {
	return func(d *RouteDefinition) {
		def := swagger.Definitions{}

		// Apply all provided options
		for _, opt := range opts {
			opt(&def)
		}

		// If Responses aren't explicitly set, add default ones based on access
		if def.Responses == nil {
			if d.Access == ACCESS_USER_ROLE || d.Access == ACCESS_ADMIN_ROLE {
				def.Responses = DefaultAuthErrorResponses()
			} else {
				def.Responses = DefaultPublicErrorResponses()
			}
		}

		d.Swagger = def
	}
}

// WithSchema creates a SwaggerOption that adds sorting and filtering parameters
// based on the provided FieldSchema implementation.
func WithSchema(schema FieldSchema) SwaggerOption {
	return func(d *swagger.Definitions) {
		// Add sort parameters
		sortFields := schema.SortableFields()
		d.Querystring["_sort"] = swagger.Parameter{
			Description: "Sort by fields: " + strings.Join(sortFields, ", "),
			Schema: &swagger.Schema{
				Value: map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
						"enum": sortFields,
					},
				},
			},
		}

		d.Querystring["_order"] = swagger.Parameter{
			Description: "Sort direction",
			Schema: &swagger.Schema{
				Value: map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
						"enum": []string{"asc", "desc"},
					},
				},
			},
		}

		// Add filter parameters
		operators := schema.FilterOperators()
		for field, ops := range operators {
			d.Querystring[field] = swagger.Parameter{
				Description: "Filter operators: " + strings.Join(ops, ", "),
				Schema: &swagger.Schema{
					Value: map[string]any{
						"type":       "object",
						"properties": createOperatorSchemas(ops),
					},
				},
			}
		}
	}
}

// createOperatorSchemas generates schema definitions for filter operators.
// Returns a map of operator names to their schema definitions.
func createOperatorSchemas(ops []string) map[string]any {
	schemas := make(map[string]any)
	for _, op := range ops {
		schemas[op] = map[string]any{
			"type":        "string", // Actual type resolved by provider
			"description": operatorDocs[op],
		}
	}
	return schemas
}

// ResponseError is a placeholder struct to define the schema for error responses.
// This struct is used by the Swagger documentation generation.
type ResponseError struct {
	Error string `json:"error"`
}

// DefineSwaggerErrorResponse creates a Swagger-compatible error response definition.
func DefineSwaggerErrorResponse(status int, errorMsg string) map[int]swagger.ContentValue {
	return map[int]swagger.ContentValue{
		status: {
			Description: errorMsg,
			Content: map[string]swagger.Schema{ // Corrected: Map value type is swagger.Schema
				"application/json": {
					Value: ResponseError{Error: errorMsg},
				},
			},
		},
	}
}

// DefineSwaggerErrorResponses combines multiple error responses for Swagger docs.
func DefineSwaggerErrorResponses(responses ...map[int]swagger.ContentValue) map[int]swagger.ContentValue {
	combined := make(map[int]swagger.ContentValue)
	for _, r := range responses {
		for code, resp := range r {
			combined[code] = resp
		}
	}
	return combined
}

// DefaultCoreErrorResponses returns a map containing core HTTP error responses shared by all routes (400, 404, 500).
func DefaultCoreErrorResponses() map[int]swagger.ContentValue {
	return DefineSwaggerErrorResponses(
		DefineSwaggerErrorResponse(http.StatusBadRequest, "Bad request"),
		DefineSwaggerErrorResponse(http.StatusNotFound, "Not found"),
		DefineSwaggerErrorResponse(http.StatusInternalServerError, "Internal server error"),
	)
}

// DefaultPublicErrorResponses returns a map containing common HTTP error responses for public routes.
// Includes core errors (400, 404, 500).
func DefaultPublicErrorResponses() map[int]swagger.ContentValue {
	return DefaultCoreErrorResponses()
}

// DefaultAuthErrorResponses returns a map containing common HTTP error responses for authenticated routes.
// Includes core errors (400, 404, 500) plus auth-specific errors (401, 403).
func DefaultAuthErrorResponses() map[int]swagger.ContentValue {
	return DefineSwaggerErrorResponses(
		DefaultCoreErrorResponses(),
		DefineSwaggerErrorResponse(http.StatusUnauthorized, "Unauthorized"),
		DefineSwaggerErrorResponse(http.StatusForbidden, "Forbidden"),
	)
}

var operatorDocs = map[string]string{
	"eq":           "Equal to",
	"ne":           "Not equal to",
	"neq":          "Not equal to (alias for ne)",
	"lt":           "Less than",
	"gt":           "Greater than",
	"lte":          "Less than or equal to",
	"gte":          "Greater than or equal to",
	"in":           "Value is in the specified array",
	"nin":          "Value is not in the specified array",
	"contains":     "Case-insensitive contains",
	"containss":    "Case-sensitive contains",
	"ncontains":    "Case-insensitive does not contain",
	"ncontainss":   "Case-sensitive does not contain",
	"between":      "Value is between two values (inclusive)",
	"nbetween":     "Value is not between two values",
	"null":         "Value is null",
	"nnull":        "Value is not null",
	"startswith":   "Case-insensitive starts with",
	"startswiths":  "Case-sensitive starts with",
	"nstartswith":  "Case-insensitive does not start with",
	"nstartswiths": "Case-sensitive does not start with",
	"endswith":     "Case-insensitive ends with",
	"endswiths":    "Case-sensitive ends with",
	"nendswith":    "Case-insensitive does not end with",
	"nendswiths":   "Case-sensitive does not end with",
	"ina":          "Array contains any of the specified values",
	"nina":         "Array does not contain any of the specified values",
	"like":         "Case-insensitive contains (alias for contains)",
}

// WithPathParam creates a SwaggerOption that adds a path parameter definition.
func WithPathParam(name, description string, schemaValue any) SwaggerOption {
	return func(d *swagger.Definitions) {
		*d = SwaggerPathParam(*d, name, description, schemaValue)
	}
}

// WithQueryParam creates a SwaggerOption that adds a query parameter definition.
func WithQueryParam(name, description string, schemaValue any) SwaggerOption {
	return func(d *swagger.Definitions) {
		*d = SwaggerQueryParam(*d, name, description, schemaValue)
	}
}

// WithHeaderParam creates a SwaggerOption that adds a header parameter definition.
func WithHeaderParam(name, description string, schemaValue any) SwaggerOption {
	return func(d *swagger.Definitions) {
		*d = SwaggerHeaderParam(*d, name, description, schemaValue)
	}
}

// WithCookieParam creates a SwaggerOption that adds a cookie parameter definition.
func WithCookieParam(name, description string, schemaValue any) SwaggerOption {
	return func(d *swagger.Definitions) {
		*d = SwaggerCookieParam(*d, name, description, schemaValue)
	}
}

// WithFilterParam creates a SwaggerOption that adds a filter parameter.
func WithFilterParam(name, description string, schemaValue any) SwaggerOption {
	return func(d *swagger.Definitions) {
		*d = SwaggerFilterParam(*d, name, description, schemaValue)
	}
}

// WithPaginationParams creates a SwaggerOption that adds standard pagination parameters.
func WithPaginationParams() SwaggerOption {
	return func(d *swagger.Definitions) {
		*d = SwaggerPaginationParams(*d)
	}
}

// WithSortParams creates a SwaggerOption that adds standard sorting parameters.
func WithSortParams(sortableFields []string) SwaggerOption {
	return func(d *swagger.Definitions) {
		*d = SwaggerSortParams(*d, sortableFields)
	}
}

// WithGlobalSearchParam creates a SwaggerOption that adds the global search parameter.
func WithGlobalSearchParam() SwaggerOption {
	return func(d *swagger.Definitions) {
		*d = SwaggerGlobalSearchParam(*d)
	}
}
