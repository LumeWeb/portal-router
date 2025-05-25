package router

import (
	swagger "go.lumeweb.com/gswagger"
	"net/http"
)

// Shared base for all endpoints
// baseDefinition returns a swagger.Definitions with common response schemas
// pre-configured for success and error cases.
func baseDefinition() swagger.Definitions {
	def := swagger.Definitions{
		Responses:   make(map[int]swagger.ContentValue),
		PathParams:  make(swagger.ParameterValue),
		Querystring: make(swagger.ParameterValue),
		Headers:     make(swagger.ParameterValue),
		Cookies:     make(swagger.ParameterValue),
	}
	def.Responses = map[int]swagger.ContentValue{
		http.StatusOK: {
			Description: "Success",
			Content:     nil, // Response body will be defined by the route
		},
		http.StatusUnprocessableEntity: {
			Description: "Validation Failed",
			Content: swagger.Content{
				"application/json": {
					Value: map[string]any{
						"error":  "validation failed",
						"fields": map[string]string{},
					},
				},
			},
		},
		http.StatusInternalServerError: {
			Description: "Internal Server Error",
			Content: swagger.Content{
				"application/json": {
					Value: map[string]string{
						"error": "Internal Server Error",
					},
				},
			},
		},
	}
	return def
}

// Helper to apply options to any definition
// applyOpts applies a set of SwaggerOptions to a swagger.Definitions,
// returning the modified definitions.
func applyOpts(d swagger.Definitions, opts []SwaggerOption) swagger.Definitions {
	// Make shallow copy of the definition
	result := d

	// Ensure maps are initialized if nil
	if result.Responses == nil {
		result.Responses = make(map[int]swagger.ContentValue)
	}
	if result.PathParams == nil {
		result.PathParams = make(swagger.ParameterValue)
	}
	if result.Querystring == nil {
		result.Querystring = make(swagger.ParameterValue)
	}
	if result.Headers == nil {
		result.Headers = make(swagger.ParameterValue)
	}
	if result.Cookies == nil {
		result.Cookies = make(swagger.ParameterValue)
	}
	if result.Parameters == nil {
		result.Parameters = make(map[string]swagger.ParameterDefinition)
	}

	// Safely apply options if they exist
	if opts != nil {
		for _, opt := range opts {
			if opt != nil {
				opt(&result)
			}
		}
	}

	return result
}
