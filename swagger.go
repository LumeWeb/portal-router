package router

import (
	"fmt"
	"net/http"
	"strings"

	swagger "go.lumeweb.com/gswagger"
	"go.lumeweb.com/portal-middleware/auth/jwt"
	"go.lumeweb.com/queryutil/filter"
)

const (
	MediaTypeJSON = "application/json"
)

// SwaggerOption defines a function type for modifying swagger.Definitions.
// Used to apply customizations to OpenAPI/Swagger documentation.
type SwaggerOption func(*swagger.Definitions, string)

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
			opt(&d.Swagger, d.Access)
		}
	}
}

// WithRequestBody creates a Swagger option for request body definition.
func WithRequestBody(value interface{}, description string, required bool) SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		d.RequestBody = &swagger.ContentValue{
			Description: description,
			Required:    required,
			Content: map[string]swagger.Schema{
				MediaTypeJSON: {
					Value: value,
				},
			},
		}
	}
}

// WithFileUpload creates a Swagger option for file upload definition.
func WithFileUpload(description string, required bool) SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		d.RequestBody = &swagger.ContentValue{
			Description: description,
			Required:    required,
			Content: map[string]swagger.Schema{
				"multipart/form-data": {
					Value: struct {
						File string `json:"file" jsonschema:"type=string,format=binary"`
					}{},
				},
			},
		}
	}
}

// WithArrayResponse creates a Swagger option for array response definition.
func WithArrayResponse(status int, description string, itemValue interface{}) SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		if d.Responses == nil {
			d.Responses = make(map[int]swagger.ContentValue)
		}
		d.Responses[status] = swagger.ContentValue{
			Description: description,
			Content: map[string]swagger.Schema{
				MediaTypeJSON: {
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
	return func(d *swagger.Definitions, accessRole string) {
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
	return func(d *swagger.Definitions, accessRole string) {
		d.Tags = tags
	}
}

// WithSummary creates a Swagger option for setting summary.
func WithSummary(summary string) SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		d.Summary = summary
	}
}

// WithDescription creates a Swagger option for setting description.
func WithDescription(description string) SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		d.Description = description
	}
}

// WithSwagger creates a RouteOption that sets the Swagger documentation definitions
// for a route. Automatically includes appropriate error responses based on access level.
// Preserves any existing success responses (2xx status codes) and allows custom
// success responses from options to be merged.
func WithSwagger(opts ...SwaggerOption) RouteOption {
	return func(d *RouteDefinition) {
		// Get default error responses based on access level
		var defaultErrors map[int]swagger.ContentValue
		if d.Access == ACCESS_USER_ROLE || d.Access == ACCESS_ADMIN_ROLE {
			defaultErrors = DefaultAuthErrorResponses()
		} else {
			defaultErrors = DefaultPublicErrorResponses()
		}

		// Initialize responses if nil
		if d.Swagger.Responses == nil {
			d.Swagger.Responses = make(map[int]swagger.ContentValue)
		}

		// Merge default error responses into existing responses, preserving existing success responses
		d.Swagger.Responses = MergeResponses(d.Swagger.Responses, defaultErrors)

		// Apply user-provided options directly to d.Swagger
		// This allows options like WithSuccessResponse to add/modify responses
		d.Swagger = applySwaggerOpts(d.Swagger, d.Access, opts)
	}
}

// WithSchema creates a SwaggerOption that adds sorting and filtering parameters
// based on the provided FieldSchema implementation.
func WithSchema(schema FieldSchema) SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
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

// Response defines a standardized response structure
type Response struct {
	Description string
	Content     *Content
	Headers     []Header
}

// Content defines the response content for a specific media type
type Content struct {
	MediaType string
	Schema    interface{}
}

// Header represents a single response header
type Header struct {
	Name  string
	Value string
}

// ResponseOption defines a function type for modifying Response properties
type ResponseOption func(*Response)

// ResponseError defines the interface for rich error responses that can
// provide HTTP status codes and custom serialization.
type ResponseError interface {
	error
	HttpStatus() int
}

// ErrorResponse is the default error response format for simple errors
type ErrorResponse struct {
	Message string `json:"error"`
}

func (e ErrorResponse) Error() string {
	return e.Message
}

func (e ErrorResponse) HttpStatus() int {
	// Default to 500 if not set
	return http.StatusInternalServerError
}

// ErrorResponder extends ResponseError with header support
type ErrorResponder interface {
	ResponseError
	Headers() map[string]string
}

// ErrorWrapper is a helper to implement ResponseError for simple errors
type ErrorWrapper struct {
	Message string
	Status  int
}

func (e *ErrorWrapper) Error() string   { return e.Message }
func (e *ErrorWrapper) HttpStatus() int { return e.Status }

// WithContent creates a ResponseOption that sets the response content
func WithContent(mediaType string, schema interface{}) ResponseOption {
	return func(r *Response) {
		r.Content = &Content{
			MediaType: mediaType,
			Schema:    schema,
		}
	}
}

// WithHeader creates a ResponseOption that adds a response header
func WithHeader(name, description string) ResponseOption {
	return func(r *Response) {
		r.Headers = append(r.Headers, Header{
			Name:  name,
			Value: description,
		})
	}
}

// WithJSONContent helper for common JSON responses
func WithJSONContent(schema interface{}) ResponseOption {
	return WithContent(MediaTypeJSON, schema)
}

// WithTotalCountHeader adds the X-Total-Count header to a response
func WithTotalCountHeader() ResponseOption {
	return WithHeader("X-Total-Count", "Total number of items")
}

// WithCacheControl helper for cache headers
func WithCacheControl(value string) ResponseOption {
	return WithHeader("Cache-Control", value)
}

// DefineResponse creates a Response with the given status code and options
func DefineResponse(status int, description string, opts ...ResponseOption) map[int]swagger.ContentValue {
	r := Response{
		Description: description,
	}

	for _, opt := range opts {
		opt(&r)
	}

	// Convert to swagger format (internal only)
	headers := make(map[string]string)
	for _, h := range r.Headers {
		headers[h.Name] = h.Value
	}

	contentValue := swagger.ContentValue{
		Description: r.Description,
		Headers:     headers,
	}

	if r.Content != nil {
		contentValue.Content = swagger.Content{
			r.Content.MediaType: swagger.Schema{Value: r.Content.Schema},
		}
	}

	return map[int]swagger.ContentValue{
		status: contentValue,
	}
}

// WithSuccessResponse creates a SwaggerOption that adds a success response while preserving existing responses.
// The success response will not overwrite any existing response for the same status code.
func WithSuccessResponse(status int, description string, opts ...ResponseOption) SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		// Initialize responses if nil
		if d.Responses == nil {
			d.Responses = make(map[int]swagger.ContentValue)
		}

		// Use DefineResponse to create the base response
		response := DefineResponse(status, description, opts...)

		// Merge with existing responses, preserving existing success responses
		d.Responses = MergeResponses(response, d.Responses)
	}
}

// WithPaginatedResponse creates a paginated list response helper
func WithPaginatedResponse(itemType interface{}, paginationMeta interface{}) SwaggerOption {
	return WithSuccessResponse(http.StatusOK, "Success",
		WithJSONContent(map[string]interface{}{
			"items":      []interface{}{itemType},
			"pagination": paginationMeta,
		}),
	)
}

// DefineSwaggerErrorResponse creates a Swagger-compatible error response definition.
// Supports:
// - string messages
// - error interface
// - ResponseError implementations
// - ErrorResponse struct
func DefineSwaggerErrorResponse(status int, errValue interface{}) map[int]swagger.ContentValue {
	var errorMsg string
	var schema interface{}

	switch v := errValue.(type) {
	case string:
		errorMsg = v
		schema = ErrorResponse{Message: v}
	case ResponseError:
		errorMsg = v.Error()
		schema = v
	case ErrorResponse:
		errorMsg = v.Message
		schema = v
	default:
		// Handle any other type including error interface
		if err, ok := errValue.(error); ok {
			errorMsg = err.Error()
			schema = ErrorResponse{Message: errorMsg}
		} else {
			errorMsg = fmt.Sprintf("%v", errValue)
			schema = ErrorResponse{Message: errorMsg}
		}
	}

	return map[int]swagger.ContentValue{
		status: {
			Description: errorMsg,
			Content: swagger.Content{
				MediaTypeJSON: {
					Value: schema,
				},
			},
		},
	}
}

// AsErrorResponse converts an error to a response object
func AsErrorResponse(err error) interface{} {
	if err == nil {
		return ErrorResponse{Message: ""}
	}
	if respErr, ok := err.(ResponseError); ok {
		return respErr
	}
	return ErrorResponse{Message: err.Error()}
}

// MergeResponses combines multiple response maps while preserving success responses (2xx).
// Later responses override earlier ones for the same status code, except success responses
// which are preserved from their first occurrence.
func MergeResponses(responses ...map[int]swagger.ContentValue) map[int]swagger.ContentValue {
	combined := make(map[int]swagger.ContentValue)

	// Process responses in order, preserving the first success response encountered
	for _, responseMap := range responses {
		for code, resp := range responseMap {
			// If it's a success response (2xx) and we don't have one for this code yet, add it.
			if code >= 200 && code < 300 {
				if _, exists := combined[code]; !exists {
					combined[code] = resp
				}
			} else {
				// If it's an error response or a non-2xx success code, always add/override.
				combined[code] = resp
			}
		}
	}

	return combined
}

// DefineSwaggerErrorResponses combines multiple error responses for Swagger docs.
// Preserves existing success responses (2xx) when merging.
func DefineSwaggerErrorResponses(responses ...map[int]swagger.ContentValue) map[int]swagger.ContentValue {
	return MergeResponses(responses...)
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
	return func(d *swagger.Definitions, accessRole string) {
		*d = SwaggerPathParam(*d, name, description, schemaValue)
	}
}

// WithQueryParam creates a SwaggerOption that adds a query parameter definition.
func WithQueryParam(name, description string, schemaValue any) SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		*d = SwaggerQueryParam(*d, name, description, schemaValue)
	}
}

// WithHeaderParam creates a SwaggerOption that adds a header parameter definition.
func WithHeaderParam(name, description string, schemaValue any) SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		*d = SwaggerHeaderParam(*d, name, description, schemaValue)
	}
}

// WithCookieParam creates a SwaggerOption that adds a cookie parameter definition.
func WithCookieParam(name, description string, schemaValue any) SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		*d = SwaggerCookieParam(*d, name, description, schemaValue)
	}
}

// WithFilterParam creates a SwaggerOption that adds a filter parameter.
func WithFilterParam(name, description string, schemaValue any) SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		*d = SwaggerFilterParam(*d, name, description, schemaValue)
	}
}

// WithFilterParamsFromSchema creates a SwaggerOption that adds all filter parameters
// from a FieldSchema's FilterOperators() map. Supports multiple formats:
// - Simple: field_operator=value (e.g. age_gt=30)
// - Array values: field_operator=value1,value2,value3 or field_operator[]=value1&field_operator[]=value2
// - Complex: filters[field][operator]=value (e.g. filters[age][gt]=30)
func WithFilterParamsFromSchema(schema FieldSchema) SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		operators := schema.FilterOperators()
		for fieldName, ops := range operators {
			for _, op := range ops {
				// Validate operator string
				if _, exists := operatorDocs[op]; !exists {
					continue // Skip unknown operators
				}
				// Determine if this operator expects array values
				isArrayOp := opIsMultiValue(filter.Operator(op)) ||
					filter.Operator(op) == filter.OpBetween || filter.Operator(op) == filter.OpNbetween

				// Add simple format param
				simpleParam := fmt.Sprintf("%s_%s", fieldName, op)
				paramDesc := fmt.Sprintf("Filter by %s %s",
					fieldName, strings.ToLower(op))

				if isArrayOp {
					// Array parameter schema
					*d = SwaggerFilterParam(*d, simpleParam, paramDesc,
						map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string", // Will be converted by parser
							},
							"style":              "form",
							"explode":            false,
							"x-csv":              true,
							"x-collectionFormat": "multi",
						})
				} else {
					// Single value parameter
					*d = SwaggerFilterParam(*d, simpleParam, paramDesc, "")
				}

				// Add complex format param
				complexParam := fmt.Sprintf("filters[%s][%s]", fieldName, op)
				*d = SwaggerFilterParam(*d, complexParam,
					fmt.Sprintf("Filter by %s %s",
						fieldName, strings.ToLower(op)),
					"")
			}
		}
	}
}

// opIsMultiValue checks if an operator expects multiple values
// convertErrorResponses converts a map of error responses from map[int]any to map[int]swagger.ContentValue
func convertErrorResponses(errResp map[int]any) map[int]swagger.ContentValue {
	converted := make(map[int]swagger.ContentValue)
	for code, body := range errResp {
		converted[code] = swagger.ContentValue{
			Description: http.StatusText(code),
			Content: swagger.Content{
				MediaTypeJSON: {
					Value: body,
				},
			},
		}
	}
	return converted
}

func opIsMultiValue(op filter.Operator) bool {
	return op == filter.OpIn || op == filter.OpNin ||
		op == filter.OpIna || op == filter.OpNina ||
		op == filter.OpBetween || op == filter.OpNbetween
}

// WithPaginationParams creates a SwaggerOption that adds standard pagination parameters.
func WithPaginationParams() SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		*d = SwaggerPaginationParams(*d)
	}
}

// WithSortParams creates a SwaggerOption that adds standard sorting parameters.
func WithSortParams(sortableFields []string) SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		*d = SwaggerSortParams(*d, sortableFields)
	}
}

// WithGlobalSearchParam creates a SwaggerOption that adds the global search parameter.
func WithGlobalSearchParam() SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		*d = SwaggerGlobalSearchParam(*d)
	}
}

// WithListEndpoint creates a SwaggerOption for configuring a list endpoint with pagination, sorting and filtering.
// It includes authentication, pagination, sorting, global search, and specific filter parameters.
//
// Parameters:
// - summary, description: Standard operation details.
// - purpose: JWT purpose (for authenticated endpoints). Use jwt.PurposeNone for public.
// - itemSchema: An instance of the schema for a single item in the list response.
// - paginationSchema: An instance of the schema for pagination metadata in the response (can be nil).
// - sortableFields: A list of fields that can be sorted.
// - filterParams: A slice of FilterParam structs defining additional filter query parameters.
// - opt: Additional SwaggerOption to apply to the definitions.
func WithListEndpoint(
	summary, description string,
	purpose jwt.Purpose,
	itemSchema any,
	paginationSchema any,
	sortableFields []string,
	filterParams []FilterParam,
	opts ...SwaggerOption,
) SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		*d = ListEndpointSwagger(
			summary,
			description,
			purpose,
			itemSchema,
			paginationSchema,
			sortableFields,
			filterParams,
			nil,
			opts...,
		)
	}
}

// WithErrorResponses creates a SwaggerOption that merges custom error responses with
// default responses based on access level while preserving existing success responses.
// The merging follows these rules:
// 1. Existing success responses (2xx) are preserved
// 2. Default error responses are added if not already present
// 3. Custom error responses override defaults
func WithErrorResponses(customErrors map[int]swagger.ContentValue) SwaggerOption {
	return func(d *swagger.Definitions, accessRole string) {
		// Get default error responses based on access level
		var defaultErrors map[int]swagger.ContentValue
		if accessRole == ACCESS_USER_ROLE || accessRole == ACCESS_ADMIN_ROLE {
			defaultErrors = DefaultAuthErrorResponses()
		} else {
			defaultErrors = DefaultPublicErrorResponses()
		}

		// Initialize responses if nil
		if d.Responses == nil {
			d.Responses = make(map[int]swagger.ContentValue)
		}

		// Merge in order: defaults -> custom errors.
		// Existing success responses in d.Responses are preserved by MergeResponses.
		d.Responses = MergeResponses(
			d.Responses, // Start with existing responses (preserves existing success)
			defaultErrors,
			customErrors,
		)
	}
}
